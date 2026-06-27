package cache

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

type StorePolicy struct {
	CacheNoStore bool
	CachePrivate bool
}

func AnalyzeHeaders(h http.Header, now time.Time) (CachePolicy, *time.Time, *time.Time) {
	var policy CachePolicy
	cc := parseCacheControl(h.Values("Cache-Control"))
	if _, ok := cc["no-store"]; ok {
		policy.NoStorePresent = true
	}
	if _, ok := cc["private"]; ok {
		policy.PrivatePresent = true
	}
	if _, ok := cc["no-cache"]; ok {
		policy.NoCachePresent = true
	}
	if _, ok := cc["must-revalidate"]; ok {
		policy.MustRevalidatePresent = true
	}

	var expiresAt *time.Time
	if raw, ok := cc["max-age"]; ok {
		if seconds, err := strconv.Atoi(raw); err == nil {
			policy.MaxAgeSeconds = &seconds
			age := headerSeconds(h.Get("Age"))
			exp := now.Add(time.Duration(seconds-age) * time.Second)
			expiresAt = &exp
		}
	} else if expRaw := h.Get("Expires"); expRaw != "" {
		if exp, err := http.ParseTime(expRaw); err == nil {
			expiresAt = &exp
		}
	}

	var lastModifiedAt *time.Time
	if lm := h.Get("Last-Modified"); lm != "" {
		if parsed, err := http.ParseTime(lm); err == nil {
			lastModifiedAt = &parsed
		}
	}
	return policy, expiresAt, lastModifiedAt
}

func CacheableByPolicy(policy CachePolicy, cfg StorePolicy) bool {
	if policy.NoStorePresent && !cfg.CacheNoStore {
		return false
	}
	if policy.PrivatePresent && !cfg.CachePrivate {
		return false
	}
	return true
}

func MergeRevalidationHeaders(old *Metadata, headers http.Header, now time.Time) {
	if old == nil {
		return
	}
	for _, name := range []string{"Cache-Control", "ETag", "Last-Modified", "Expires", "Content-Type", "Content-Length"} {
		if v := headers.Values(name); len(v) > 0 {
			old.Fetch.ResponseHeaders[strings.ToLower(name)] = strings.Join(v, ", ")
		}
	}
	policy, expiresAt, lastModifiedAt := AnalyzeHeaders(headers, now)
	old.LastCheckedAt = now
	old.ExpiresAt = expiresAt
	old.LastModifiedAt = lastModifiedAt
	old.CachePolicy = policy
}

func parseCacheControl(values []string) map[string]string {
	out := map[string]string{}
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			name, val, _ := strings.Cut(part, "=")
			name = strings.ToLower(strings.TrimSpace(name))
			val = strings.Trim(strings.TrimSpace(val), `"`)
			out[name] = val
		}
	}
	return out
}

func headerSeconds(raw string) int {
	if raw == "" {
		return 0
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds < 0 {
		return 0
	}
	return seconds
}
