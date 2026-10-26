package npm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type RegistryClient interface {
	PublishedAt(ctx context.Context, packageName, version string) (time.Time, error)
}

// VersionFacts are registry facts about one published version and, for an
// update, the version it replaces.
type VersionFacts struct {
	Published          time.Time
	PackageCreated     time.Time
	Publisher          string
	Provenance         bool
	HasPrevious        bool
	PreviousPublisher  string
	PreviousProvenance bool
}

// FactsClient is implemented by registry clients that can report more than
// the publish time. The analysis uses it when the client supports it.
type FactsClient interface {
	VersionFacts(ctx context.Context, packageName, version, previousVersion string) (VersionFacts, error)
}

// maxPackumentBytes bounds the full metadata document read for one package.
// Popular packages with thousands of versions are tens of megabytes.
const maxPackumentBytes = 64 << 20

type HTTPRegistryClient struct {
	client  *http.Client
	baseURL string

	mu    sync.Mutex
	cache map[string]packumentResult
}

type packumentResult struct {
	doc *packument
	err error
}

type packument struct {
	Time     map[string]string           `json:"time"`
	Versions map[string]packumentVersion `json:"versions"`
}

type packumentVersion struct {
	NPMUser struct {
		Name string `json:"name"`
	} `json:"_npmUser"`
	Dist struct {
		Attestations *struct {
			Provenance json.RawMessage `json:"provenance"`
		} `json:"attestations"`
	} `json:"dist"`
}

func NewRegistryClient() *HTTPRegistryClient {
	return NewRegistryClientWithBaseURL("https://registry.npmjs.org")
}

// NewRegistryClientWithBaseURL points the client at another npm-compatible
// registry, for example a test server.
func NewRegistryClientWithBaseURL(baseURL string) *HTTPRegistryClient {
	return &HTTPRegistryClient{
		client:  &http.Client{Timeout: 15 * time.Second},
		baseURL: strings.TrimRight(baseURL, "/"),
		cache:   map[string]packumentResult{},
	}
}

func (c *HTTPRegistryClient) PublishedAt(ctx context.Context, packageName, version string) (time.Time, error) {
	if packageName == "" || version == "" {
		return time.Time{}, fmt.Errorf("missing package name or version")
	}
	doc, err := c.packument(ctx, packageName)
	if err != nil {
		return time.Time{}, err
	}
	published, ok := doc.timeOf(version)
	if !ok {
		return time.Time{}, fmt.Errorf("publish time not found for %s@%s", packageName, version)
	}
	return published, nil
}

func (c *HTTPRegistryClient) VersionFacts(ctx context.Context, packageName, version, previousVersion string) (VersionFacts, error) {
	if packageName == "" || version == "" {
		return VersionFacts{}, fmt.Errorf("missing package name or version")
	}
	doc, err := c.packument(ctx, packageName)
	if err != nil {
		return VersionFacts{}, err
	}
	facts := VersionFacts{}
	facts.Published, _ = doc.timeOf(version)
	facts.PackageCreated, _ = doc.timeOf("created")
	if meta, ok := doc.Versions[version]; ok {
		facts.Publisher = meta.NPMUser.Name
		facts.Provenance = meta.hasProvenance()
	}
	if previousVersion != "" {
		if meta, ok := doc.Versions[previousVersion]; ok {
			facts.HasPrevious = true
			facts.PreviousPublisher = meta.NPMUser.Name
			facts.PreviousProvenance = meta.hasProvenance()
		}
	}
	return facts, nil
}

func (c *HTTPRegistryClient) packument(ctx context.Context, packageName string) (*packument, error) {
	c.mu.Lock()
	cached, ok := c.cache[packageName]
	c.mu.Unlock()
	if ok {
		return cached.doc, cached.err
	}
	doc, err := c.fetchPackument(ctx, packageName)
	c.mu.Lock()
	c.cache[packageName] = packumentResult{doc: doc, err: err}
	c.mu.Unlock()
	return doc, err
}

func (c *HTTPRegistryClient) fetchPackument(ctx context.Context, packageName string) (*packument, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/"+url.PathEscape(packageName), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("npm registry returned HTTP %d for %s", resp.StatusCode, packageName)
	}
	limited := &io.LimitedReader{R: resp.Body, N: maxPackumentBytes + 1}
	var doc packument
	if err := json.NewDecoder(limited).Decode(&doc); err != nil {
		if limited.N <= 0 {
			return nil, fmt.Errorf("npm metadata for %s is larger than %d MiB", packageName, maxPackumentBytes>>20)
		}
		return nil, err
	}
	return &doc, nil
}

func (p *packument) timeOf(key string) (time.Time, bool) {
	value, ok := p.Time[key]
	if !ok || value == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, false
	}
	return parsed.UTC(), true
}

func (v packumentVersion) hasProvenance() bool {
	if v.Dist.Attestations == nil {
		return false
	}
	raw := strings.TrimSpace(string(v.Dist.Attestations.Provenance))
	return raw != "" && raw != "null"
}
