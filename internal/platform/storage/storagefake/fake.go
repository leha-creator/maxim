// Package storagefake — хранилище в памяти для тестов (07 §9).
package storagefake

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"sync"
	"time"

	"github.com/leha-creator/maxim/internal/platform/storage"
)

// Object — сохранённый объект.
type Object struct {
	Data        []byte
	ContentType string
}

// Storage — реализация storage.ObjectStorage в памяти.
type Storage struct {
	mu      sync.Mutex
	objects map[string]Object
}

var _ storage.ObjectStorage = (*Storage)(nil)

// New создаёт пустое хранилище.
func New() *Storage { return &Storage{objects: map[string]Object{}} }

func (s *Storage) PutPrivate(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	if int64(len(data)) != size {
		return fmt.Errorf("storagefake: size %d, read %d bytes", size, len(data))
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.objects[key] = Object{Data: data, ContentType: contentType}
	return nil
}

// PresignGet возвращает ссылку вида https://storage.test/<key>?expires=<секунды>.
func (s *Storage) PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.objects[key]; !ok {
		return "", storage.ErrNotFound
	}
	return fmt.Sprintf("https://storage.test/%s?expires=%d", url.PathEscape(key), int(ttl.Seconds())), nil
}

func (s *Storage) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.objects, key)
	return nil
}

// Get возвращает объект по ключу.
func (s *Storage) Get(key string) (Object, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, ok := s.objects[key]
	if ok {
		o.Data = bytes.Clone(o.Data)
	}
	return o, ok
}

// Len — число объектов.
func (s *Storage) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.objects)
}
