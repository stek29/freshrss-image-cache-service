package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stek29/freshrss-image-cache-service/internal/cache"
)

func TestFileSystemStoreRoundTrip(t *testing.T) {
	ctx := context.Background()
	store := NewFileSystemStore(t.TempDir())
	key := cache.Key("https://example.com/image.png")
	meta := &cache.Metadata{
		Version:     1,
		Key:         key,
		URL:         "https://example.com/image.png",
		CachedAt:    time.Now().UTC(),
		ContentType: "image/png",
		Fetch:       cache.FetchInfo{ResponseHeaders: map[string]string{}},
	}
	if err := store.PutBlob(ctx, key, bytes.NewBufferString("image"), BlobInfo{}); err != nil {
		t.Fatal(err)
	}
	if err := store.PutMetadata(ctx, key, meta); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetMetadata(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if got.URL != meta.URL {
		t.Fatalf("metadata URL mismatch: %q", got.URL)
	}
	blob, info, err := store.GetBlob(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	defer blob.Close()
	if info.Size != 5 {
		t.Fatalf("size mismatch: %d", info.Size)
	}
}

func TestFileSystemStoreCorruptMetadata(t *testing.T) {
	root := t.TempDir()
	store := NewFileSystemStore(root)
	key := cache.Key("https://example.com/image.png")
	if err := store.PutBlob(context.Background(), key, bytes.NewBufferString("image"), BlobInfo{}); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, key[:2])
	if err := os.WriteFile(filepath.Join(dir, key+".json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetMetadata(context.Background(), key); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound for corrupt metadata, got %v", err)
	}
}

func TestMetadataJSONShape(t *testing.T) {
	key := cache.Key("https://example.com/image.png")
	meta := cache.Metadata{Version: 1, Key: key, Fetch: cache.FetchInfo{ResponseHeaders: map[string]string{"etag": `"x"`}}}
	b, err := json.Marshal(meta)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(b, []byte(`"version":1`)) {
		t.Fatalf("unexpected json: %s", b)
	}
}
