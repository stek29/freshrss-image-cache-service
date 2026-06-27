package fetch

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stek29/freshrss-image-cache-service/internal/config"
)

func TestBuildHeadersPrecedence(t *testing.T) {
	target, _ := url.Parse("https://example.com/image.jpg")
	cfg := config.Default().Headers
	cfg.DefaultHeaders["Referer"] = "https://default.example/"
	cfg.HostHeaders["example.com"] = map[string]string{
		"Referer":    "https://host.example/",
		"User-Agent": "host-agent",
	}
	incoming := http.Header{
		"User-Agent":      {"client-agent"},
		"Accept-Encoding": {"br"},
		"Referer":         {"https://incoming.example/"},
		"Cookie":          {"secret"},
	}
	got := BuildHeaders(cfg, target, incoming, "https://query.example/")
	if got.Get("User-Agent") != "client-agent" {
		t.Fatalf("incoming user agent should win, got %q", got.Get("User-Agent"))
	}
	if got.Get("Referer") != "https://query.example/" {
		t.Fatalf("query referer should win, got %q", got.Get("Referer"))
	}
	if got.Get("Accept-Encoding") != "" {
		t.Fatalf("accept-encoding must not be forwarded")
	}
	if got.Get("Cookie") != "" {
		t.Fatalf("cookie must not be forwarded")
	}
}

func TestBuildHeadersHostSpecificLowercase(t *testing.T) {
	target, _ := url.Parse("https://EXAMPLE.com/image.jpg")
	cfg := config.Default().Headers
	cfg.HostHeaders["example.com"] = map[string]string{"Referer": "https://host.example/"}
	got := BuildHeaders(cfg, target, http.Header{}, "")
	if got.Get("Referer") != "https://host.example/" {
		t.Fatalf("host-specific referer mismatch: %q", got.Get("Referer"))
	}
}
