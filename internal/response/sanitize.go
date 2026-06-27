package response

import (
	"net/http"
	"strings"
)

var blockedResponseHeaders = map[string]bool{
	"set-cookie":          true,
	"cookie":              true,
	"authorization":       true,
	"proxy-authorization": true,
	"proxy-authenticate":  true,
	"www-authenticate":    true,
	"content-encoding":    true,
	"transfer-encoding":   true,
	"connection":          true,
	"keep-alive":          true,
	"alt-svc":             true,
	"server":              true,
	"date":                true,
}

func SanitizedHeaderMap(h http.Header) map[string]string {
	out := map[string]string{}
	for name, values := range h {
		lower := strings.ToLower(name)
		if blockedResponseHeaders[lower] || strings.HasPrefix(lower, "proxy-") {
			continue
		}
		if len(values) > 0 {
			out[lower] = strings.Join(values, ", ")
		}
	}
	return out
}

func CopyProxyHeaders(dst, src http.Header) {
	for name, values := range src {
		canonical := http.CanonicalHeaderKey(name)
		if isHopByHop(canonical) {
			continue
		}
		for _, value := range values {
			dst.Add(canonical, value)
		}
	}
}

func CopyCachedHeaders(dst http.Header, stored map[string]string) {
	for name, value := range stored {
		canonical := http.CanonicalHeaderKey(name)
		lower := strings.ToLower(name)
		if blockedResponseHeaders[lower] || isHopByHop(canonical) || strings.HasPrefix(lower, "proxy-") {
			continue
		}
		if canonical == "Content-Length" || canonical == "Content-Type" {
			continue
		}
		dst.Set(canonical, value)
	}
}

func isHopByHop(name string) bool {
	switch name {
	case "Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization", "Te", "Trailer", "Transfer-Encoding", "Upgrade":
		return true
	default:
		return false
	}
}
