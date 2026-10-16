// Package storage — S3-совместимое объектное хранилище (07 §1, A-26).
// Закрытая область хранит оригиналы и документы; наружу они выдаются только
// кратковременными подписанными ссылками (07 §4).
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/leha-creator/maxim/internal/platform/clock"
	"github.com/leha-creator/maxim/internal/platform/ids"
	"github.com/leha-creator/maxim/internal/platform/logx"
)

// ErrNotFound — объекта нет.
var ErrNotFound = errors.New("storage: object not found")

// ObjectStorage — закрытая область хранилища.
type ObjectStorage interface {
	PutPrivate(ctx context.Context, key string, r io.Reader, size int64, contentType string) error
	PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error)
	Delete(ctx context.Context, key string) error
}

// NewKey генерирует имя объекта: `<area>/<uuidv7><ext>`. Имя от пользователя не используется (07 §4).
func NewKey(area, ext string) string {
	return area + "/" + ids.New().String() + ext
}

// S3Options — параметры подключения.
type S3Options struct {
	Endpoint  string
	Region    string
	Bucket    string
	AccessKey string
	SecretKey string
	UseSSL    bool
}

// S3 — реализация на minio-go.
type S3 struct {
	client *minio.Client
	bucket string
	clock  clock.Clock
	log    *slog.Logger
}

// NewS3 подключается к хранилищу и проверяет, что корзина есть.
func NewS3(ctx context.Context, o S3Options, clk clock.Clock, log *slog.Logger) (*S3, error) {
	client, err := minio.New(o.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(o.AccessKey, o.SecretKey, ""),
		Secure: o.UseSSL,
		Region: o.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("storage client: %w", err)
	}
	ok, err := client.BucketExists(ctx, o.Bucket)
	if err != nil {
		return nil, fmt.Errorf("storage bucket check: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("storage bucket %q does not exist", o.Bucket)
	}
	log.DebugContext(ctx, "storage connected", logx.KeyOp, "Storage.Open", "endpoint", o.Endpoint, "bucket", o.Bucket)
	return &S3{client: client, bucket: o.Bucket, clock: clk, log: log}, nil
}

// PutPrivate сохраняет объект в закрытой области.
func (s *S3) PutPrivate(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
	start := s.clock.Now()
	_, err := s.client.PutObject(ctx, s.bucket, key, r, size, minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		s.log.ErrorContext(ctx, "object storage failed", logx.KeyOp, "Storage.Put", "key", key, logx.KeyError, err.Error())
		return fmt.Errorf("put object: %w", err)
	}
	s.log.DebugContext(ctx, "object stored", logx.KeyOp, "Storage.Put", "key", key, "size", size,
		"duration_ms", s.clock.Now().Sub(start).Milliseconds())
	return nil
}

// PresignGet выдаёт подписанную ссылку на чтение со сроком ttl.
func (s *S3) PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error) {
	u, err := s.client.PresignedGetObject(ctx, s.bucket, key, ttl, url.Values{})
	if err != nil {
		s.log.ErrorContext(ctx, "object storage failed", logx.KeyOp, "Storage.Presign", "key", key, logx.KeyError, err.Error())
		return "", fmt.Errorf("presign object: %w", err)
	}
	return u.String(), nil
}

// Delete удаляет объект.
func (s *S3) Delete(ctx context.Context, key string) error {
	if err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{}); err != nil {
		s.log.ErrorContext(ctx, "object storage failed", logx.KeyOp, "Storage.Delete", "key", key, logx.KeyError, err.Error())
		return fmt.Errorf("delete object: %w", err)
	}
	s.log.DebugContext(ctx, "object deleted", logx.KeyOp, "Storage.Delete", "key", key)
	return nil
}
