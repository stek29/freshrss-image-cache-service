package app

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

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
	return mux
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
	if subtle.ConstantTimeCompare([]byte(req.AccessToken), []byte(h.token)) != 1 {
		writeJSONStatus(w, http.StatusForbidden, "INVALID_TOKEN")
		return
	}
	if err := h.service.Warm(r.Context(), req.URL); err != nil {
		if errors.Is(err, ErrInvalidURL) {
			writeJSONStatus(w, http.StatusBadRequest, "INVALID_URL")
			return
		}
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
