package app

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/stek29/freshrss-image-cache-service/internal/cache"
	"github.com/stek29/freshrss-image-cache-service/internal/config"
	"github.com/stek29/freshrss-image-cache-service/internal/fetch"
	"github.com/stek29/freshrss-image-cache-service/internal/response"
	"github.com/stek29/freshrss-image-cache-service/internal/storage"
)

const (
	StatusMiss        = "MISS"
	StatusHit         = "HIT"
	StatusRevalidated = "REVALIDATED"
	StatusRefreshed   = "REFRESHED"
	StatusStale       = "STALE"
	StatusBypass      = "BYPASS"
)

type Service struct {
	cfg   config.Config
	store interface {
		storage.BlobStore
		storage.MetadataStore
	}
	fetcher *fetch.Client
	group   singleflight.Group
	logger  *slog.Logger
	now     func() time.Time
}

type Outcome struct {
	Status               string
	Metadata             *cache.Metadata
	Blob                 io.ReadCloser
	BlobInfo             storage.BlobInfo
	ProxyResult          *fetch.Result
	OriginRequestHeaders http.Header
	OriginStatusCode     int
}

func NewService(cfg config.Config, store interface {
	storage.BlobStore
	storage.MetadataStore
}, fetcher *fetch.Client, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{cfg: cfg, store: store, fetcher: fetcher, logger: logger, now: time.Now}
}

func (s *Service) Resolve(ctx context.Context, rawURL string, incoming http.Header, referer string) (*Outcome, error) {
	u, ok := cache.ValidateURL(rawURL)
	if !ok {
		return nil, ErrInvalidURL
	}
	key := cache.Key(rawURL)
	meta, blob, info := s.loadEntry(ctx, key)
	if meta != nil && meta.IsFresh(s.now()) {
		return &Outcome{Status: StatusHit, Metadata: meta, Blob: blob, BlobInfo: info}, nil
	}
	if blob != nil {
		_ = blob.Close()
	}

	value, err, _ := s.group.Do(key, func() (any, error) {
		return s.refresh(ctx, key, rawURL, u, incoming, referer, meta != nil)
	})
	if err != nil {
		if meta != nil {
			blob, info, getErr := s.store.GetBlob(ctx, key)
			if getErr == nil {
				return &Outcome{Status: StatusStale, Metadata: meta, Blob: blob, BlobInfo: info}, nil
			}
		}
		return nil, err
	}
	outcome := value.(*Outcome)
	if outcome.Status == StatusBypass && meta != nil {
		if blob, info, getErr := s.store.GetBlob(ctx, key); getErr == nil {
			return &Outcome{Status: StatusStale, Metadata: meta, Blob: blob, BlobInfo: info, OriginRequestHeaders: outcome.OriginRequestHeaders, OriginStatusCode: outcome.OriginStatusCode}, nil
		}
	}
	return outcome, nil
}

func (s *Service) Warm(ctx context.Context, rawURL string) error {
	outcome, err := s.Resolve(ctx, rawURL, http.Header{}, "")
	if err != nil {
		return err
	}
	if outcome.Blob != nil {
		_ = outcome.Blob.Close()
	}
	if outcome.Status == StatusBypass {
		return ErrFetchFailed
	}
	return nil
}

func (s *Service) refresh(ctx context.Context, key, rawURL string, u *url.URL, incoming http.Header, referer string, hadCache bool) (*Outcome, error) {
	now := s.now()
	originHeaders := fetch.BuildHeaders(s.cfg.Headers, u, incoming, referer)
	if hadCache {
		if meta, err := s.store.GetMetadata(ctx, key); err == nil {
			if etag := meta.ETag(); etag != "" {
				originHeaders.Set("If-None-Match", etag)
			}
			if lm := meta.LastModified(); lm != "" {
				originHeaders.Set("If-Modified-Since", lm)
			}
		}
	}
	result, err := s.fetcher.Do(ctx, rawURL, originHeaders)
	if err != nil {
		s.logger.Warn("origin fetch failed", "url", rawURL, "had_cache", hadCache, "err", err)
		if hadCache {
			return &Outcome{Status: StatusBypass, OriginRequestHeaders: originHeaders.Clone()}, nil
		}
		return nil, err
	}
	if hadCache && result.StatusCode == http.StatusNotModified {
		meta, err := s.store.GetMetadata(ctx, key)
		if err != nil {
			return outcomeFromResult(StatusBypass, result), nil
		}
		cache.MergeRevalidationHeaders(meta, result.Header, now)
		if err := s.store.PutMetadata(ctx, key, meta); err != nil {
			return nil, err
		}
		blob, info, err := s.store.GetBlob(ctx, key)
		if err != nil {
			return nil, err
		}
		return &Outcome{Status: StatusRevalidated, Metadata: meta, Blob: blob, BlobInfo: info, OriginRequestHeaders: result.RequestHeaders.Clone(), OriginStatusCode: result.StatusCode}, nil
	}
	if !result.ValidImage200() {
		return outcomeFromResult(StatusBypass, result), nil
	}
	policy, expiresAt, lastModifiedAt := cache.AnalyzeHeaders(result.Header, now)
	if !cache.CacheableByPolicy(policy, cache.StorePolicy{CacheNoStore: s.cfg.CachePolicy.CacheNoStore, CachePrivate: s.cfg.CachePolicy.CachePrivate}) {
		return outcomeFromResult(StatusBypass, result), nil
	}
	contentType := fetch.SafeContentType(result.ContentType, result.DetectedContentType)
	meta := &cache.Metadata{
		Version:             1,
		Key:                 key,
		URL:                 rawURL,
		CachedAt:            now.UTC(),
		LastCheckedAt:       now.UTC(),
		LastModifiedAt:      lastModifiedAt,
		ExpiresAt:           expiresAt,
		ContentType:         contentType,
		DetectedContentType: result.DetectedContentType,
		Size:                int64(len(result.Body)),
		Fetch: cache.FetchInfo{
			StatusCode:      result.StatusCode,
			RequestHeaders:  fetch.HeaderMap(result.RequestHeaders),
			ResponseHeaders: response.SanitizedHeaderMap(result.Header),
		},
		CachePolicy: policy,
	}
	if err := s.store.PutBlob(ctx, key, bytes.NewReader(result.Body), storage.BlobInfo{Size: int64(len(result.Body)), ContentType: contentType}); err != nil {
		return nil, err
	}
	if err := s.store.PutMetadata(ctx, key, meta); err != nil {
		return nil, err
	}
	blob, info, err := s.store.GetBlob(ctx, key)
	if err != nil {
		return nil, err
	}
	status := StatusMiss
	if hadCache {
		status = StatusRefreshed
	}
	return &Outcome{Status: status, Metadata: meta, Blob: blob, BlobInfo: info, OriginRequestHeaders: result.RequestHeaders.Clone(), OriginStatusCode: result.StatusCode}, nil
}

func outcomeFromResult(status string, result *fetch.Result) *Outcome {
	if result == nil {
		return &Outcome{Status: status}
	}
	return &Outcome{
		Status:               status,
		ProxyResult:          result,
		OriginRequestHeaders: result.RequestHeaders.Clone(),
		OriginStatusCode:     result.StatusCode,
	}
}

func (s *Service) loadEntry(ctx context.Context, key string) (*cache.Metadata, io.ReadCloser, storage.BlobInfo) {
	meta, err := s.store.GetMetadata(ctx, key)
	if err != nil {
		return nil, nil, storage.BlobInfo{}
	}
	blob, info, err := s.store.GetBlob(ctx, key)
	if err != nil {
		return nil, nil, storage.BlobInfo{}
	}
	return meta, blob, info
}

var (
	ErrInvalidURL  = errors.New("invalid url")
	ErrFetchFailed = errors.New("failed to fetch cacheable image")
)
