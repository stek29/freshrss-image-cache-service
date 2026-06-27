package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/stek29/freshrss-image-cache-service/internal/cache"
)

type FileSystemStore struct {
	root string
}

func NewFileSystemStore(root string) *FileSystemStore {
	return &FileSystemStore{root: root}
}

func (s *FileSystemStore) GetBlob(_ context.Context, key string) (io.ReadCloser, BlobInfo, error) {
	if !validKey(key) {
		return nil, BlobInfo{}, ErrNotFound
	}
	path := s.blobPath(key)
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, BlobInfo{}, ErrNotFound
	}
	if err != nil {
		return nil, BlobInfo{}, err
	}
	stat, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, BlobInfo{}, err
	}
	return f, BlobInfo{Size: stat.Size()}, nil
}

func (s *FileSystemStore) PutBlob(_ context.Context, key string, r io.Reader, _ BlobInfo) error {
	if !validKey(key) {
		return fmt.Errorf("invalid cache key")
	}
	dir := s.shardDir(key)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+key+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := io.Copy(tmp, r); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, s.blobPath(key)); err != nil {
		return err
	}
	return syncDir(dir)
}

func (s *FileSystemStore) DeleteBlob(_ context.Context, key string) error {
	if !validKey(key) {
		return nil
	}
	err := os.Remove(s.blobPath(key))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (s *FileSystemStore) GetMetadata(_ context.Context, key string) (*cache.Metadata, error) {
	if !validKey(key) {
		return nil, ErrNotFound
	}
	b, err := os.ReadFile(s.metadataPath(key))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var meta cache.Metadata
	if err := json.Unmarshal(b, &meta); err != nil {
		return nil, ErrNotFound
	}
	if meta.Key != key {
		return nil, ErrNotFound
	}
	if _, err := os.Stat(s.blobPath(key)); errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	return &meta, nil
}

func (s *FileSystemStore) PutMetadata(_ context.Context, key string, metadata *cache.Metadata) error {
	if !validKey(key) {
		return fmt.Errorf("invalid cache key")
	}
	dir := s.shardDir(key)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+key+".*.json.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.WriteString("\n"); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, s.metadataPath(key)); err != nil {
		return err
	}
	return syncDir(dir)
}

func (s *FileSystemStore) DeleteMetadata(_ context.Context, key string) error {
	if !validKey(key) {
		return nil
	}
	err := os.Remove(s.metadataPath(key))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (s *FileSystemStore) blobPath(key string) string {
	return filepath.Join(s.shardDir(key), key)
}

func (s *FileSystemStore) metadataPath(key string) string {
	return s.blobPath(key) + ".json"
}

func (s *FileSystemStore) shardDir(key string) string {
	return filepath.Join(s.root, key[:2])
}

func validKey(key string) bool {
	if len(key) != 64 {
		return false
	}
	return strings.IndexFunc(key, func(r rune) bool {
		return !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f'))
	}) == -1
}

func syncDir(dir string) error {
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}
