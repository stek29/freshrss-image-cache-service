package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

func Load() (Config, error) {
	cfg := Default()
	path := os.Getenv("IMAGE_CACHE_CONFIG")
	if path == "" {
		for _, candidate := range []string{"./config.yaml", "/etc/freshrss-image-cache-service/config.yaml"} {
			if _, err := os.Stat(candidate); err == nil {
				path = candidate
				break
			}
		}
	}
	if path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return cfg, err
		}
		if err := yaml.Unmarshal(b, &cfg); err != nil {
			return cfg, err
		}
	}
	applyEnv(&cfg)
	normalize(&cfg)
	if cfg.AccessToken == "" {
		return cfg, errors.New("access_token is required")
	}
	return cfg, nil
}

func applyEnv(cfg *Config) {
	if v := os.Getenv("IMAGE_CACHE_LISTEN"); v != "" {
		cfg.Listen = v
	}
	if v := os.Getenv("IMAGE_CACHE_DATA_DIR"); v != "" {
		cfg.DataDir = v
	}
	if v := os.Getenv("IMAGE_CACHE_ACCESS_TOKEN"); v != "" {
		cfg.AccessToken = v
	}
}

func normalize(cfg *Config) {
	def := Default()
	if cfg.Listen == "" {
		cfg.Listen = def.Listen
	}
	if cfg.DataDir == "" {
		cfg.DataDir = def.DataDir
	}
	if cfg.HTTPClient.Timeout == 0 {
		cfg.HTTPClient.Timeout = def.HTTPClient.Timeout
	}
	if cfg.HTTPClient.MaxBodySize == 0 {
		cfg.HTTPClient.MaxBodySize = def.HTTPClient.MaxBodySize
	}
	if cfg.HTTPClient.MaxRedirects == 0 {
		cfg.HTTPClient.MaxRedirects = def.HTTPClient.MaxRedirects
	}
	if cfg.Headers.DefaultHeaders == nil {
		cfg.Headers.DefaultHeaders = def.Headers.DefaultHeaders
	}
	if len(cfg.Headers.ForwardRequestHeaders) == 0 {
		cfg.Headers.ForwardRequestHeaders = def.Headers.ForwardRequestHeaders
	}
	if cfg.Headers.HostHeaders == nil {
		cfg.Headers.HostHeaders = map[string]map[string]string{}
	}
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = def.Logging.Level
	}
	if cfg.Logging.Format == "" {
		cfg.Logging.Format = def.Logging.Format
	}
}

func (d *HTTPClient) UnmarshalYAML(value *yaml.Node) error {
	type rawClient struct {
		Timeout         string `yaml:"timeout"`
		MaxBodySize     string `yaml:"max_body_size"`
		FollowRedirects *bool  `yaml:"follow_redirects"`
		MaxRedirects    int    `yaml:"max_redirects"`
	}
	var raw rawClient
	if err := value.Decode(&raw); err != nil {
		return err
	}
	if raw.Timeout != "" {
		parsed, err := time.ParseDuration(raw.Timeout)
		if err != nil {
			return err
		}
		d.Timeout = parsed
	}
	if raw.MaxBodySize != "" {
		parsed, err := parseBytes(raw.MaxBodySize)
		if err != nil {
			return err
		}
		d.MaxBodySize = parsed
	}
	if raw.FollowRedirects != nil {
		d.FollowRedirects = *raw.FollowRedirects
	}
	d.MaxRedirects = raw.MaxRedirects
	return nil
}

func parseBytes(raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, fmt.Errorf("empty size")
	}
	multiplier := int64(1)
	units := map[string]int64{
		"kib": 1024, "kb": 1000, "k": 1024,
		"mib": 1024 * 1024, "mb": 1000 * 1000, "m": 1024 * 1024,
		"gib": 1024 * 1024 * 1024, "gb": 1000 * 1000 * 1000, "g": 1024 * 1024 * 1024,
	}
	lower := strings.ToLower(raw)
	for suffix, m := range units {
		if strings.HasSuffix(lower, suffix) {
			multiplier = m
			raw = strings.TrimSpace(raw[:len(raw)-len(suffix)])
			break
		}
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, err
	}
	return n * multiplier, nil
}
