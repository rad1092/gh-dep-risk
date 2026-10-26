package npm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const samplePackument = `{
  "name": "left-pad",
  "time": {
    "created": "2026-01-02T03:04:05.000Z",
    "1.0.0": "2026-01-02T03:04:05.000Z",
    "2.0.0": "2026-04-20T10:00:00.123Z"
  },
  "versions": {
    "1.0.0": {"_npmUser": {"name": "alice"}, "dist": {"attestations": {"url": "https://registry.npmjs.org/-/npm/v1/attestations/left-pad@1.0.0", "provenance": {"predicateType": "https://slsa.dev/provenance/v1"}}}},
    "2.0.0": {"_npmUser": {"name": "mallory"}, "dist": {}}
  }
}`

func TestVersionFactsReadsPublisherProvenanceAndDates(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Path != "/left-pad" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(samplePackument))
	}))
	defer server.Close()

	client := NewRegistryClientWithBaseURL(server.URL)
	facts, err := client.VersionFacts(context.Background(), "left-pad", "2.0.0", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if facts.Publisher != "mallory" || facts.PreviousPublisher != "alice" {
		t.Fatalf("publishers: %+v", facts)
	}
	if facts.Provenance || !facts.PreviousProvenance || !facts.HasPrevious {
		t.Fatalf("provenance: %+v", facts)
	}
	if want := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC); !facts.PackageCreated.Equal(want) {
		t.Fatalf("created = %s", facts.PackageCreated)
	}
	published, err := client.PublishedAt(context.Background(), "left-pad", "2.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if !published.Equal(facts.Published) || published.IsZero() {
		t.Fatalf("published = %s, facts %s", published, facts.Published)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("expected one registry request per package, got %d", got)
	}
}

func TestPackumentLargerThanTheLimitIsAnError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"time":{"1.0.0":"`))
		_, _ = w.Write([]byte(strings.Repeat("x", maxPackumentBytes+10)))
		_, _ = w.Write([]byte(`"}}`))
	}))
	defer server.Close()

	_, err := NewRegistryClientWithBaseURL(server.URL).PublishedAt(context.Background(), "huge", "1.0.0")
	if err == nil || !strings.Contains(err.Error(), "larger than 64 MiB") {
		t.Fatalf("expected a size error, got %v", err)
	}
}
