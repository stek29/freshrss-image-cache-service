package fetch

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/stek29/freshrss-image-cache-service/internal/config"
)

func BuildHeaders(cfg config.Headers, target *url.URL, incoming http.Header, queryReferer string) http.Header {
	out := http.Header{}
	setHeaders(out, cfg.DefaultHeaders)
	if hostHeaders, ok := cfg.HostHeaders[strings.ToLower(target.Hostname())]; ok {
		setHeaders(out, hostHeaders)
	} else if hostHeaders, ok := cfg.HostHeaders[target.Host]; ok {
		setHeaders(out, hostHeaders)
	}
	allowed := map[string]bool{}
	for _, name := range cfg.ForwardRequestHeaders {
		allowed[http.CanonicalHeaderKey(name)] = true
	}
	for name, values := range incoming {
		canonical := http.CanonicalHeaderKey(name)
		if !allowed[canonical] || blockedRequestHeader(canonical) {
			continue
		}
		out.Del(canonical)
		for _, value := range values {
			out.Add(canonical, value)
		}
	}
	if queryReferer != "" {
		out.Set("Referer", queryReferer)
	}
	out.Del("Accept-Encoding")
	out.Del("Host")
	out.Del("Content-Length")
	out.Del("Connection")
	out.Del("Transfer-Encoding")
	return out
}

func HeaderMap(h http.Header) map[string]string {
	out := map[string]string{}
	for name, values := range h {
		if len(values) == 0 {
			continue
		}
		out[strings.ToLower(name)] = strings.Join(values, ", ")
	}
	return out
}

func setHeaders(h http.Header, values map[string]string) {
	for name, value := range values {
		if value == "" {
			continue
		}
		h.Set(http.CanonicalHeaderKey(name), value)
	}
}

func blockedRequestHeader(name string) bool {
	if strings.HasPrefix(name, "X-Forwarded-") {
		return true
	}
	switch name {
	case "Cookie", "Authorization", "Proxy-Authorization", "Forwarded", "Host", "Connection", "Transfer-Encoding", "Content-Length", "Accept-Encoding":
		return true
	default:
		return false
	}
}
