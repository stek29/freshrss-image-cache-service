package app

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/stek29/freshrss-image-cache-service/internal/config"
	"github.com/stek29/freshrss-image-cache-service/internal/response"
)

type Handler struct {
	service *Service
	token   string
	cors    config.CORS
	logger  *slog.Logger
}

func NewHandler(service *Service, token string, cors config.CORS, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{service: service, token: token, cors: cors, logger: logger}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", h.root)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	return h.accessLog(mux)
}

func (h *Handler) root(w http.ResponseWriter, r *http.Request) {
	h.applyCORS(w, r)
	switch r.Method {
	case http.MethodGet:
		h.get(w, r)
	case http.MethodPost:
		h.post(w, r)
	case http.MethodOptions:
		w.Header().Set("Allow", "GET, POST, OPTIONS")
		w.WriteHeader(http.StatusNoContent)
	default:
		w.Header().Set("Allow", "GET, POST, OPTIONS")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	rawURL := r.URL.Query().Get("url")
	setAccessLogURL(r.Context(), rawURL)
	if rawURL == "" {
		http.Error(w, "missing url", http.StatusBadRequest)
		return
	}
	outcome, err := h.service.Resolve(r.Context(), rawURL, r.Header, r.URL.Query().Get("referer"))
	if err != nil {
		if errors.Is(err, ErrInvalidURL) {
			http.Error(w, "invalid url", http.StatusBadRequest)
			return
		}
		http.Error(w, "failed to fetch", http.StatusBadGateway)
		return
	}
	setAccessLogOrigin(r.Context(), outcome)
	setAccessLogCacheStatus(r.Context(), outcome.Status)
	h.writeOutcome(w, outcome)
}

func (h *Handler) post(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var req struct {
		URL         string `json:"url"`
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil || req.URL == "" || req.AccessToken == "" {
		writeJSONStatus(w, http.StatusBadRequest, "BAD_REQUEST")
		return
	}
	setAccessLogURL(r.Context(), req.URL)
	if subtle.ConstantTimeCompare([]byte(req.AccessToken), []byte(h.token)) != 1 {
		writeJSONStatus(w, http.StatusForbidden, "INVALID_TOKEN")
		return
	}
	outcome, err := h.service.Resolve(r.Context(), req.URL, http.Header{}, "")
	if err != nil {
		if errors.Is(err, ErrInvalidURL) {
			writeJSONStatus(w, http.StatusBadRequest, "INVALID_URL")
			return
		}
		writeJSONStatus(w, http.StatusBadGateway, "FAILED_TO_FETCH")
		return
	}
	setAccessLogOrigin(r.Context(), outcome)
	setAccessLogCacheStatus(r.Context(), outcome.Status)
	if outcome.Blob != nil {
		_ = outcome.Blob.Close()
	}
	if outcome.Status == StatusBypass {
		writeJSONStatus(w, http.StatusBadGateway, "FAILED_TO_FETCH")
		return
	}
	writeJSONStatus(w, http.StatusOK, "OK")
}

func (h *Handler) writeOutcome(w http.ResponseWriter, outcome *Outcome) {
	if outcome.ProxyResult != nil && outcome.Status == StatusBypass {
		response.CopyProxyHeaders(w.Header(), outcome.ProxyResult.Header)
		w.Header().Set("X-Piccache-Status", StatusBypass)
		w.WriteHeader(outcome.ProxyResult.StatusCode)
		_, _ = w.Write(outcome.ProxyResult.Body)
		return
	}
	defer outcome.Blob.Close()
	response.ServeCached(w, outcome.Status, outcome.Metadata, outcome.Blob, outcome.BlobInfo.Size)
}

type accessLogInfo struct {
	targetURL       string
	clientReferer   string
	clientUserAgent string
	originReferer   string
	originUserAgent string
	originStatus    int
	cacheStatus     string
}

type accessLogContextKey struct{}

func (h *Handler) accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}

		started := time.Now()
		info := &accessLogInfo{
			targetURL:       r.URL.Query().Get("url"),
			clientReferer:   accessLogReferer(r),
			clientUserAgent: r.UserAgent(),
		}
		ctx := context.WithValue(r.Context(), accessLogContextKey{}, info)
		r = r.WithContext(ctx)

		rec := &accessLogResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rec, r)

		if info.cacheStatus == "" {
			info.cacheStatus = rec.Header().Get("X-Piccache-Status")
		}
		h.logger.Info("access",
			"method", r.Method,
			"path", r.URL.Path,
			"url", info.targetURL,
			"client_referer", info.clientReferer,
			"client_user_agent", info.clientUserAgent,
			"origin_referer", info.originReferer,
			"origin_user_agent", info.originUserAgent,
			"origin_status", info.originStatus,
			"cache_status", info.cacheStatus,
			"status", rec.statusCode,
			"bytes", rec.bytesWritten,
			"duration", time.Since(started),
			"remote_addr", r.RemoteAddr,
		)
	})
}

type accessLogResponseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
	wroteHeader  bool
}

func (w *accessLogResponseWriter) WriteHeader(code int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *accessLogResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytesWritten += n
	return n, err
}

func setAccessLogURL(ctx context.Context, rawURL string) {
	info, ok := ctx.Value(accessLogContextKey{}).(*accessLogInfo)
	if !ok {
		return
	}
	info.targetURL = rawURL
}

func setAccessLogCacheStatus(ctx context.Context, status string) {
	info, ok := ctx.Value(accessLogContextKey{}).(*accessLogInfo)
	if !ok {
		return
	}
	info.cacheStatus = status
}

func setAccessLogOrigin(ctx context.Context, outcome *Outcome) {
	info, ok := ctx.Value(accessLogContextKey{}).(*accessLogInfo)
	if !ok || outcome == nil {
		return
	}
	info.originStatus = outcome.OriginStatusCode
	if outcome.OriginRequestHeaders == nil {
		return
	}
	info.originReferer = outcome.OriginRequestHeaders.Get("Referer")
	info.originUserAgent = outcome.OriginRequestHeaders.Get("User-Agent")
}

func accessLogReferer(r *http.Request) string {
	if referer := r.URL.Query().Get("referer"); referer != "" {
		return referer
	}
	return r.Header.Get("Referer")
}

func writeJSONStatus(w http.ResponseWriter, code int, status string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": status})
}

func (h *Handler) applyCORS(w http.ResponseWriter, r *http.Request) {
	if !h.cors.Enabled {
		return
	}
	origin := allowedOrigin(h.cors.AllowedOrigins, r.Header.Get("Origin"))
	if origin == "" {
		return
	}
	header := w.Header()
	header.Set("Access-Control-Allow-Origin", origin)
	if origin != "*" {
		header.Add("Vary", "Origin")
	}
	if len(h.cors.AllowedMethods) > 0 {
		header.Set("Access-Control-Allow-Methods", strings.Join(h.cors.AllowedMethods, ", "))
	}
	if len(h.cors.AllowedHeaders) > 0 {
		header.Set("Access-Control-Allow-Headers", strings.Join(h.cors.AllowedHeaders, ", "))
	}
	if len(h.cors.ExposeHeaders) > 0 {
		header.Set("Access-Control-Expose-Headers", strings.Join(h.cors.ExposeHeaders, ", "))
	}
	if h.cors.MaxAge > 0 {
		header.Set("Access-Control-Max-Age", strconv.Itoa(h.cors.MaxAge))
	}
}

func allowedOrigin(allowed []string, origin string) string {
	for _, candidate := range allowed {
		if candidate == "*" {
			return "*"
		}
		if origin != "" && strings.EqualFold(candidate, origin) {
			return origin
		}
	}
	return ""
}
