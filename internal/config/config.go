package config

import "time"

type Config struct {
	Listen      string      `yaml:"listen"`
	DataDir     string      `yaml:"data_dir"`
	AccessToken string      `yaml:"access_token"`
	HTTPClient  HTTPClient  `yaml:"http_client"`
	CachePolicy CachePolicy `yaml:"cache_policy"`
	Headers     Headers     `yaml:"headers"`
	CORS        CORS        `yaml:"cors"`
	Logging     Logging     `yaml:"logging"`
}

type HTTPClient struct {
	Timeout         time.Duration `yaml:"timeout"`
	MaxBodySize     int64         `yaml:"max_body_size"`
	FollowRedirects bool          `yaml:"follow_redirects"`
	MaxRedirects    int           `yaml:"max_redirects"`
}

type CachePolicy struct {
	CacheNoStore bool `yaml:"cache_no_store"`
	CachePrivate bool `yaml:"cache_private"`
}

type Headers struct {
	ForwardRequestHeaders []string                     `yaml:"forward_request_headers"`
	DefaultHeaders        map[string]string            `yaml:"default_headers"`
	HostHeaders           map[string]map[string]string `yaml:"host_headers"`
}

type CORS struct {
	Enabled        bool     `yaml:"enabled"`
	AllowedOrigins []string `yaml:"allowed_origins"`
	AllowedMethods []string `yaml:"allowed_methods"`
	AllowedHeaders []string `yaml:"allowed_headers"`
	ExposeHeaders  []string `yaml:"expose_headers"`
	MaxAge         int      `yaml:"max_age"`
}

type Logging struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

func Default() Config {
	return Config{
		Listen:      ":3000",
		DataDir:     "./images",
		AccessToken: "change-me",
		HTTPClient: HTTPClient{
			Timeout:         15 * time.Second,
			MaxBodySize:     50 * 1024 * 1024,
			FollowRedirects: true,
			MaxRedirects:    5,
		},
		CachePolicy: CachePolicy{
			CacheNoStore: true,
			CachePrivate: true,
		},
		Headers: Headers{
			ForwardRequestHeaders: []string{"User-Agent", "Accept", "Accept-Language", "Referer", "Origin"},
			DefaultHeaders: map[string]string{
				"User-Agent":      "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36",
				"Accept":          "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8",
				"Accept-Language": "en-US,en;q=0.9",
			},
			HostHeaders: map[string]map[string]string{},
		},
		CORS: CORS{
			Enabled:        true,
			AllowedOrigins: []string{"*"},
			AllowedMethods: []string{"GET", "POST", "OPTIONS"},
			AllowedHeaders: []string{"Content-Type"},
			ExposeHeaders:  []string{"X-Piccache-Status", "Warning"},
			MaxAge:         86400,
		},
		Logging: Logging{Level: "info", Format: "text"},
	}
}
