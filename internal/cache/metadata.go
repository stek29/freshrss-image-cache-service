package cache

import (
	"strings"
	"time"
)

type Metadata struct {
	Version             int         `json:"version"`
	Key                 string      `json:"key"`
	URL                 string      `json:"url"`
	CachedAt            time.Time   `json:"cached_at"`
	LastCheckedAt       time.Time   `json:"last_checked_at"`
	LastModifiedAt      *time.Time  `json:"last_modified_at,omitempty"`
	ExpiresAt           *time.Time  `json:"expires_at,omitempty"`
	ContentType         string      `json:"content_type"`
	DetectedContentType string      `json:"detected_content_type"`
	Size                int64       `json:"size"`
	Fetch               FetchInfo   `json:"fetch"`
	CachePolicy         CachePolicy `json:"cache_policy"`
}

type FetchInfo struct {
	StatusCode      int               `json:"status_code"`
	RequestHeaders  map[string]string `json:"request_headers"`
	ResponseHeaders map[string]string `json:"response_headers"`
}

type CachePolicy struct {
	NoStorePresent        bool `json:"no_store_present"`
	PrivatePresent        bool `json:"private_present"`
	NoCachePresent        bool `json:"no_cache_present"`
	MustRevalidatePresent bool `json:"must_revalidate_present"`
	MaxAgeSeconds         *int `json:"max_age_seconds,omitempty"`
}

func (m *Metadata) IsFresh(now time.Time) bool {
	if m == nil {
		return false
	}
	if m.CachePolicy.NoCachePresent || m.CachePolicy.MustRevalidatePresent {
		return false
	}
	if m.ExpiresAt == nil {
		return true
	}
	return now.Before(*m.ExpiresAt)
}

func (m *Metadata) ETag() string {
	return m.responseHeader("ETag")
}

func (m *Metadata) LastModified() string {
	if m == nil || m.Fetch.ResponseHeaders == nil {
		return ""
	}
	return m.responseHeader("Last-Modified")
}

func (m *Metadata) responseHeader(name string) string {
	if m == nil || m.Fetch.ResponseHeaders == nil {
		return ""
	}
	if v := m.Fetch.ResponseHeaders[strings.ToLower(name)]; v != "" {
		return v
	}
	return ""
}
