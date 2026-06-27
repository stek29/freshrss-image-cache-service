package cache

import "testing"

func TestKeyStable(t *testing.T) {
	got := Key("https://example.com/image.jpg?a=1&b=2")
	if got != "a28a1a51efbb8d687b15ab02358745d759e30c03614f2edc485dccaf40502082" {
		t.Fatalf("unexpected key: %s", got)
	}
}

func TestValidateURL(t *testing.T) {
	valid := []string{"http://example.com/a.png", "https://example.com/a.png"}
	for _, raw := range valid {
		if _, ok := ValidateURL(raw); !ok {
			t.Fatalf("expected valid URL: %s", raw)
		}
	}
	invalid := []string{"", "/x", "ftp://example.com/x", "https:///x"}
	for _, raw := range invalid {
		if _, ok := ValidateURL(raw); ok {
			t.Fatalf("expected invalid URL: %s", raw)
		}
	}
}
