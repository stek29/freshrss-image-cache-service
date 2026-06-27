package response

import (
	"io"
	"net/http"
	"strconv"

	"github.com/stek29/freshrss-image-cache-service/internal/cache"
)

func ServeCached(w http.ResponseWriter, status string, meta *cache.Metadata, body io.Reader, size int64) {
	CopyCachedHeaders(w.Header(), meta.Fetch.ResponseHeaders)
	w.Header().Set("Content-Type", meta.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	w.Header().Set("X-Piccache-Status", status)
	if status == "STALE" {
		w.Header().Set("Warning", `110 - "Response is stale"`)
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, body)
}
