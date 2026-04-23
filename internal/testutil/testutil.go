// Package testutil provides shared test helpers for the crawler.
package testutil

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/jnfarmer/distributed-crawl/fetcher"
)

// FakeSite starts an httptest server that serves canned HTML pages from a path-to-body map.
func FakeSite(t testing.TB, pages map[string]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := pages[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		if _, err := w.Write([]byte(body)); err != nil {
			panic(fmt.Sprintf("FakeSite: write failed: %v", err))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// FakeFetcher implements fetcher.Fetcher with canned responses for testing.
type FakeFetcher struct {
	mu    sync.Mutex
	Pages map[string]FakePage
	Calls []string
	Delay time.Duration
}

// FakePage defines the canned response for a single URL.
type FakePage struct {
	Body        string
	StatusCode  int
	ContentType string
	Err         error
}

var _ fetcher.Fetcher = (*FakeFetcher)(nil)

// Fetch returns the canned response for the given URL.
func (f *FakeFetcher) Fetch(ctx context.Context, u *url.URL) ([]byte, int, string, error) {
	f.mu.Lock()
	f.Calls = append(f.Calls, u.String())
	delay := f.Delay
	f.mu.Unlock()

	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return nil, 0, "", ctx.Err()
		}
	}

	key := u.String()
	page, ok := f.Pages[key]
	if !ok {
		return nil, 404, "", fmt.Errorf("not found: %s", key)
	}
	if page.Err != nil {
		return nil, 0, "", page.Err
	}
	statusCode := page.StatusCode
	if statusCode == 0 {
		statusCode = 200
	}
	contentType := page.ContentType
	if contentType == "" {
		contentType = "text/html"
	}
	return []byte(page.Body), statusCode, contentType, nil
}

// GetCalls returns a snapshot of all URLs fetched so far, safe for concurrent use.
func (f *FakeFetcher) GetCalls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	calls := make([]string, len(f.Calls))
	copy(calls, f.Calls)
	return calls
}

// IsAllowed always returns true for the fake fetcher.
func (f *FakeFetcher) IsAllowed(_ context.Context, _ *url.URL) bool {
	return true
}
