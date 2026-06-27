package fetch

import (
	"bytes"
	"mime"
	"net/http"
	"strings"
)

func NormalizeContentType(raw string) string {
	if raw == "" {
		return ""
	}
	mediaType, _, err := mime.ParseMediaType(raw)
	if err != nil {
		mediaType = strings.Split(raw, ";")[0]
	}
	return strings.ToLower(strings.TrimSpace(mediaType))
}

func DetectContentType(body []byte) string {
	prefix := body
	if len(prefix) > 512 {
		prefix = prefix[:512]
	}
	detected := NormalizeContentType(http.DetectContentType(prefix))
	if detected == "text/xml" || detected == "text/plain" {
		if looksLikeSVG(prefix) {
			return "image/svg+xml"
		}
	}
	return detected
}

func IsImageType(mediaType string) bool {
	return strings.HasPrefix(NormalizeContentType(mediaType), "image/")
}

func SafeContentType(origin, detected string) string {
	origin = NormalizeContentType(origin)
	detected = NormalizeContentType(detected)
	if IsImageType(origin) {
		return origin
	}
	if IsImageType(detected) {
		return detected
	}
	return "application/octet-stream"
}

func looksLikeSVG(prefix []byte) bool {
	p := bytes.TrimSpace(prefix)
	p = bytes.ToLower(p)
	return bytes.Contains(p, []byte("<svg")) || bytes.Contains(p, []byte(":svg"))
}
