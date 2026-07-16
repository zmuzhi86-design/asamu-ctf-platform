package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"asamu.local/platform/api/internal/config"
)

type Object struct {
	Key, ContentType string
	Size             int64
}

type Storage interface {
	Put(context.Context, string, io.Reader, int64, string) error
	Open(context.Context, string) (io.ReadCloser, error)
	Delete(context.Context, string) error
	SignedURL(context.Context, string, time.Duration) (string, error)
	Ready(context.Context) error
}

func Open(cfg config.Storage) (Storage, error) {
	if cfg.Driver != "" && cfg.Driver != "local" {
		return nil, fmt.Errorf("unsupported storage driver %q: slim edition supports local storage only", cfg.Driver)
	}
	if err := os.MkdirAll(cfg.LocalRoot, 0o750); err != nil {
		return nil, err
	}
	return &Local{root: cfg.LocalRoot}, nil
}

type Local struct{ root string }

func (l *Local) path(key string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(key))
	if clean == "." || filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") {
		return "", fmt.Errorf("invalid object key")
	}
	return filepath.Join(l.root, clean), nil
}

func (l *Local) Put(_ context.Context, key string, reader io.Reader, _ int64, _ string) error {
	path, err := l.path(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o640)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, reader)
	return err
}

func (l *Local) Open(_ context.Context, key string) (io.ReadCloser, error) {
	path, err := l.path(key)
	if err != nil {
		return nil, err
	}
	return os.Open(path)
}

func (l *Local) Delete(_ context.Context, key string) error {
	path, err := l.path(key)
	if err != nil {
		return err
	}
	return os.Remove(path)
}

func (l *Local) SignedURL(_ context.Context, key string, _ time.Duration) (string, error) {
	if _, err := l.path(key); err != nil {
		return "", err
	}
	return "/api/v1/storage/" + key, nil
}

func (l *Local) Ready(context.Context) error {
	_, err := os.Stat(l.root)
	return err
}
