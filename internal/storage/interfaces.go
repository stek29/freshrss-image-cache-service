package storage

import (
	"context"
	"io"

	"github.com/stek29/freshrss-image-cache-service/internal/cache"
)

var ErrNotFound = errNotFound{}

type errNotFound struct{}

func (errNotFound) Error() string { return "not found" }

type BlobInfo struct {
	Size        int64
	ContentType string
}

type BlobStore interface {
	GetBlob(ctx context.Context, key string) (io.ReadCloser, BlobInfo, error)
	PutBlob(ctx context.Context, key string, r io.Reader, info BlobInfo) error
	DeleteBlob(ctx context.Context, key string) error
}

type MetadataStore interface {
	GetMetadata(ctx context.Context, key string) (*cache.Metadata, error)
	PutMetadata(ctx context.Context, key string, metadata *cache.Metadata) error
	DeleteMetadata(ctx context.Context, key string) error
}
