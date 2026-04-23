package testutil

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/jnfarmer/distributed-crawl/fetcher"
)

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

type FakeFetcher struct {
	mu    sync.Mutex
	Pages map[string]FakePage
	Calls []string
}

type FakePage struct {
	Body        string
	StatusCode  int
	ContentType string
	Err         error
}

var _ fetcher.Fetcher = (*FakeFetcher)(nil)

func (f *FakeFetcher) Fetch(ctx context.Context, u *url.URL) ([]byte, int, string, error) {
	f.mu.Lock()
	f.Calls = append(f.Calls, u.String())
	f.mu.Unlock()

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

func (f *FakeFetcher) GetCalls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	calls := make([]string, len(f.Calls))
	copy(calls, f.Calls)
	return calls
}

func (f *FakeFetcher) IsAllowed(_ context.Context, _ *url.URL) bool {
	return true
}
