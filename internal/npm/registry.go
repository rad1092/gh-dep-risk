package npm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type RegistryClient interface {
	PublishedAt(ctx context.Context, packageName, version string) (time.Time, error)
}

type HTTPRegistryClient struct {
	client  *http.Client
	baseURL string
}

func NewRegistryClient() *HTTPRegistryClient {
	return &HTTPRegistryClient{
		client:  &http.Client{Timeout: 5 * time.Second},
		baseURL: "https://registry.npmjs.org",
	}
}

func (c *HTTPRegistryClient) PublishedAt(ctx context.Context, packageName, version string) (time.Time, error) {
	if packageName == "" || version == "" {
		return time.Time{}, fmt.Errorf("missing package name or version")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/"+url.PathEscape(packageName), nil)
	if err != nil {
		return time.Time{}, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return time.Time{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return time.Time{}, fmt.Errorf("npm registry returned HTTP %d", resp.StatusCode)
	}
	resp.Body = io.NopCloser(io.LimitReader(resp.Body, 1<<20))
	var payload struct {
		Time map[string]string `json:"time"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return time.Time{}, err
	}
	value, ok := payload.Time[version]
	if !ok || value == "" {
		return time.Time{}, fmt.Errorf("publish time not found for %s@%s", packageName, version)
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, err
	}
	return parsed.UTC(), nil
}
