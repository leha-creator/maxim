package storage_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/leha-creator/maxim/internal/platform/clock"
	"github.com/leha-creator/maxim/internal/platform/logx"
	"github.com/leha-creator/maxim/internal/platform/storage"
	"github.com/leha-creator/maxim/internal/platform/storage/storagefake"
)

// MinioImage — образ S3-совместимого хранилища для тестов (тот же, что в compose.dev.yml).
const MinioImage = "pgsty/minio:RELEASE.2026-08-04T00-00-00Z"

// contract проверяет поведение, общее для настоящего и поддельного хранилища.
func contract(t *testing.T, s storage.ObjectStorage, read func(t *testing.T, key, presigned string) []byte) {
	t.Helper()
	ctx := context.Background()
	key := storage.NewKey("test", ".pdf")
	if !strings.HasPrefix(key, "test/") || !strings.HasSuffix(key, ".pdf") {
		t.Fatalf("key %q", key)
	}
	data := []byte("%PDF-1.7 test document")
	if err := s.PutPrivate(ctx, key, bytes.NewReader(data), int64(len(data)), "application/pdf"); err != nil {
		t.Fatalf("put: %v", err)
	}
	u, err := s.PresignGet(ctx, key, 10*time.Minute)
	if err != nil || u == "" {
		t.Fatalf("presign: %q %v", u, err)
	}
	if got := read(t, key, u); !bytes.Equal(got, data) {
		t.Fatalf("read back %q", got)
	}
	if err := s.Delete(ctx, key); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestFakeContract(t *testing.T) {
	s := storagefake.New()
	contract(t, s, func(t *testing.T, key, presigned string) []byte {
		if !strings.Contains(presigned, "expires=600") {
			t.Fatalf("presigned %q", presigned)
		}
		o, ok := s.Get(key)
		if !ok || o.ContentType != "application/pdf" {
			t.Fatalf("object %v %v", o, ok)
		}
		return o.Data
	})
	if s.Len() != 0 {
		t.Fatal("object not deleted")
	}
}

func TestMinioContract(t *testing.T) {
	ctx := context.Background()
	ctr, err := testcontainers.Run(ctx, MinioImage,
		testcontainers.WithCmd("server", "/data"),
		testcontainers.WithEnv(map[string]string{"MINIO_ROOT_USER": "minio", "MINIO_ROOT_PASSWORD": "minio-secret"}),
		testcontainers.WithExposedPorts("9000/tcp"),
		testcontainers.WithWaitStrategy(wait.ForHTTP("/minio/health/live").WithPort("9000/tcp").WithStartupTimeout(time.Minute)),
	)
	if err != nil {
		if os.Getenv("CI") == "true" {
			t.Fatalf("MinIO is required in CI: %v", err)
		}
		t.Skipf("Docker is not available, MinIO test is skipped: %v", err)
	}
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(ctr) })
	endpoint, err := ctr.PortEndpoint(ctx, "9000/tcp", "")
	if err != nil {
		t.Fatal(err)
	}
	admin, err := minio.New(endpoint, &minio.Options{Creds: credentials.NewStaticV4("minio", "minio-secret", "")})
	if err != nil {
		t.Fatal(err)
	}
	if err := admin.MakeBucket(ctx, "private", minio.MakeBucketOptions{}); err != nil {
		t.Fatal(err)
	}

	opts := storage.S3Options{Endpoint: endpoint, Region: "us-east-1", Bucket: "private", AccessKey: "minio", SecretKey: "minio-secret"}
	if _, err := storage.NewS3(ctx, storage.S3Options{Endpoint: endpoint, Region: "us-east-1", Bucket: "missing",
		AccessKey: "minio", SecretKey: "minio-secret"}, clock.System{}, logx.Discard()); err == nil {
		t.Fatal("missing bucket accepted")
	}
	s, err := storage.NewS3(ctx, opts, clock.System{}, logx.Discard())
	if err != nil {
		t.Fatal(err)
	}
	contract(t, s, func(t *testing.T, key, presigned string) []byte {
		resp, err := http.Get(presigned)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("presigned GET: %d", resp.StatusCode)
		}
		b, _ := io.ReadAll(resp.Body)
		return b
	})
	// Закрытая область без подписи недоступна.
	resp, err := http.Get("http://" + endpoint + "/private/test/anything")
	if err == nil {
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			t.Fatal("private bucket is readable without signature")
		}
	}
}
