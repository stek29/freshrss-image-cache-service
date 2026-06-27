package app

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stek29/freshrss-image-cache-service/internal/cache"
	"github.com/stek29/freshrss-image-cache-service/internal/config"
	"github.com/stek29/freshrss-image-cache-service/internal/fetch"
	"github.com/stek29/freshrss-image-cache-service/internal/storage"
)

func TestGETMissThenHit(t *testing.T) {
	var hits atomic.Int32
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "max-age=3600")
		_, _ = w.Write(testPNG(t))
	}))
	defer origin.Close()

	server := testServer(t)
	ts := httptest.NewServer(server)
	defer ts.Close()

	res := get(t, ts.URL+"/?url="+origin.URL+"/image.png")
	if res.Header.Get("X-Piccache-Status") != StatusMiss {
		t.Fatalf("expected MISS, got %s", res.Header.Get("X-Piccache-Status"))
	}
	_ = res.Body.Close()
	res = get(t, ts.URL+"/?url="+origin.URL+"/image.png")
	if res.Header.Get("X-Piccache-Status") != StatusHit {
		t.Fatalf("expected HIT, got %s", res.Header.Get("X-Piccache-Status"))
	}
	_ = res.Body.Close()
	if hits.Load() != 1 {
		t.Fatalf("origin hits = %d", hits.Load())
	}
}

func TestGETBypassUnsupported(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("nope"))
	}))
	defer origin.Close()
	ts := httptest.NewServer(testServer(t))
	defer ts.Close()

	res := get(t, ts.URL+"/?url="+origin.URL+"/page")
	defer res.Body.Close()
	if res.StatusCode != http.StatusTeapot {
		t.Fatalf("expected origin status, got %d", res.StatusCode)
	}
	if res.Header.Get("X-Piccache-Status") != StatusBypass {
		t.Fatalf("expected BYPASS, got %s", res.Header.Get("X-Piccache-Status"))
	}
}

func TestPOSTCompatibility(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(testPNG(t))
	}))
	defer origin.Close()
	ts := httptest.NewServer(testServer(t))
	defer ts.Close()

	body, _ := json.Marshal(map[string]string{"url": origin.URL + "/image.png", "access_token": "secret"})
	res, err := http.Post(ts.URL+"/", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
	b, _ := io.ReadAll(res.Body)
	if !bytes.Contains(b, []byte(`"status":"OK"`)) {
		t.Fatalf("unexpected body: %s", b)
	}
}

func TestGETRevalidatedWithETag(t *testing.T) {
	var hits atomic.Int32
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("ETag", `"v1"`)
		w.Header().Set("Cache-Control", "no-cache")
		if r.Header.Get("If-None-Match") == `"v1"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(testPNG(t))
	}))
	defer origin.Close()
	ts := httptest.NewServer(testServer(t))
	defer ts.Close()

	res := get(t, ts.URL+"/?url="+origin.URL+"/image.png")
	_ = res.Body.Close()
	if res.Header.Get("X-Piccache-Status") != StatusMiss {
		t.Fatalf("expected MISS, got %s", res.Header.Get("X-Piccache-Status"))
	}
	res = get(t, ts.URL+"/?url="+origin.URL+"/image.png")
	_ = res.Body.Close()
	if res.Header.Get("X-Piccache-Status") != StatusRevalidated {
		t.Fatalf("expected REVALIDATED, got %s", res.Header.Get("X-Piccache-Status"))
	}
	if hits.Load() != 2 {
		t.Fatalf("origin hits = %d", hits.Load())
	}
}

func TestGETStaleFallback(t *testing.T) {
	var fail atomic.Bool
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Cache-Control", "max-age=0")
		if fail.Load() {
			http.Error(w, "down", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(testPNG(t))
	}))
	defer origin.Close()
	ts := httptest.NewServer(testServer(t))
	defer ts.Close()

	res := get(t, ts.URL+"/?url="+origin.URL+"/image.png")
	_ = res.Body.Close()
	time.Sleep(10 * time.Millisecond)
	fail.Store(true)
	res = get(t, ts.URL+"/?url="+origin.URL+"/image.png")
	defer res.Body.Close()
	if res.Header.Get("X-Piccache-Status") != StatusStale {
		t.Fatalf("expected STALE, got %s", res.Header.Get("X-Piccache-Status"))
	}
	if res.Header.Get("Warning") == "" {
		t.Fatalf("expected stale warning")
	}
}

func TestPOSTInvalidToken(t *testing.T) {
	ts := httptest.NewServer(testServer(t))
	defer ts.Close()
	body, _ := json.Marshal(map[string]string{"url": "https://example.com/image.png", "access_token": "bad"})
	res, err := http.Post(ts.URL+"/", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d", res.StatusCode)
	}
}

func TestMetadataStoresLowercaseHeaders(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("ETag", `"v1"`)
		_, _ = w.Write(testPNG(t))
	}))
	defer origin.Close()

	cfg := config.Default()
	cfg.DataDir = t.TempDir()
	store := storage.NewFileSystemStore(cfg.DataDir)
	svc := NewService(cfg, store, fetch.NewClient(cfg.HTTPClient), slog.New(slog.NewTextHandler(io.Discard, nil)))
	outcome, err := svc.Resolve(t.Context(), origin.URL+"/image.png", http.Header{"User-Agent": {"tester"}}, "")
	if err != nil {
		t.Fatal(err)
	}
	_ = outcome.Blob.Close()
	meta, err := store.GetMetadata(t.Context(), cache.Key(origin.URL+"/image.png"))
	if err != nil {
		t.Fatal(err)
	}
	if meta.Fetch.ResponseHeaders["etag"] != `"v1"` {
		t.Fatalf("etag not stored lower-case: %#v", meta.Fetch.ResponseHeaders)
	}
	if meta.Fetch.ResponseHeaders["Content-Type"] != "" {
		t.Fatalf("canonical response header key should not be stored: %#v", meta.Fetch.ResponseHeaders)
	}
	if meta.Fetch.RequestHeaders["user-agent"] != "tester" {
		t.Fatalf("request headers not lower-case: %#v", meta.Fetch.RequestHeaders)
	}
}

func testServer(t *testing.T) http.Handler {
	t.Helper()
	cfg := config.Default()
	cfg.DataDir = t.TempDir()
	cfg.AccessToken = "secret"
	store := storage.NewFileSystemStore(cfg.DataDir)
	svc := NewService(cfg, store, fetch.NewClient(cfg.HTTPClient), slog.New(slog.NewTextHandler(io.Discard, nil)))
	return NewHandler(svc, cfg.AccessToken, slog.Default()).Routes()
}

func get(t *testing.T, u string) *http.Response {
	t.Helper()
	res, err := http.Get(u)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func testPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}
