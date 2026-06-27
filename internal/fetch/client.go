package fetch

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/stek29/freshrss-image-cache-service/internal/config"
)

type Client struct {
	http        *http.Client
	maxBodySize int64
}

type Result struct {
	StatusCode          int
	Header              http.Header
	Body                []byte
	ContentType         string
	DetectedContentType string
	Duration            time.Duration
	RequestHeaders      http.Header
}

func NewClient(cfg config.HTTPClient) *Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DisableCompression = true
	client := &http.Client{
		Timeout:   cfg.Timeout,
		Transport: transport,
	}
	if !cfg.FollowRedirects {
		client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		}
	} else if cfg.MaxRedirects > 0 {
		client.CheckRedirect = func(_ *http.Request, via []*http.Request) error {
			if len(via) >= cfg.MaxRedirects {
				return fmt.Errorf("stopped after %d redirects", cfg.MaxRedirects)
			}
			return nil
		}
	}
	return &Client{http: client, maxBodySize: cfg.MaxBodySize}
}

func (c *Client) Do(ctx context.Context, rawURL string, headers http.Header) (*Result, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	for name, values := range headers {
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}
	req.Header.Del("Accept-Encoding")

	start := time.Now()
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	limit := c.maxBodySize
	if limit <= 0 {
		limit = 50 * 1024 * 1024
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, ErrBodyTooLarge
	}
	originType := NormalizeContentType(resp.Header.Get("Content-Type"))
	detectedType := DetectContentType(body)
	return &Result{
		StatusCode:          resp.StatusCode,
		Header:              resp.Header.Clone(),
		Body:                body,
		ContentType:         originType,
		DetectedContentType: detectedType,
		Duration:            time.Since(start),
		RequestHeaders:      req.Header.Clone(),
	}, nil
}

func (r *Result) ValidImage200() bool {
	if r == nil || r.StatusCode != http.StatusOK {
		return false
	}
	if r.Header.Get("Content-Encoding") != "" {
		return false
	}
	return IsImageType(r.ContentType) || IsImageType(r.DetectedContentType)
}

func (r *Result) Reader() io.Reader {
	if r == nil {
		return bytes.NewReader(nil)
	}
	return bytes.NewReader(r.Body)
}

var ErrBodyTooLarge = errors.New("origin response body exceeds configured max size")
