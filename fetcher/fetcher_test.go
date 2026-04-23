package fetcher

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func mustParse(raw string) *url.URL {
	u, err := url.Parse(raw)
	if err != nil {
		panic(err)
	}
	return u
}

func TestHTTPFetcher_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(w, "<html><body>hello</body></html>")
	}))
	defer srv.Close()

	f := NewHTTPFetcher(100, 5*time.Second, "test-bot/1.0")
	body, status, contentType, err := f.Fetch(context.Background(), mustParse(srv.URL))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != 200 {
		t.Errorf("expected status 200, got %d", status)
	}
	if !strings.Contains(contentType, "text/html") {
		t.Errorf("expected text/html content type, got %q", contentType)
	}
	if string(body) != "<html><body>hello</body></html>" {
		t.Errorf("unexpected body: %q", string(body))
	}
}

func TestHTTPFetcher_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	f := NewHTTPFetcher(100, 5*time.Second, "test-bot/1.0")
	_, status, _, err := f.Fetch(context.Background(), mustParse(srv.URL))

	if err != nil {
		t.Fatalf("404 should not return error, got: %v", err)
	}
	if status != 404 {
		t.Errorf("expected status 404, got %d", status)
	}
}

func TestHTTPFetcher_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		time.Sleep(2 * time.Second)
	}))
	defer srv.Close()

	f := NewHTTPFetcher(100, 100*time.Millisecond, "test-bot/1.0")
	_, _, _, err := f.Fetch(context.Background(), mustParse(srv.URL))

	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

func TestHTTPFetcher_ContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		time.Sleep(5 * time.Second)
	}))
	defer srv.Close()

	f := NewHTTPFetcher(100, 10*time.Second, "test-bot/1.0")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	_, _, _, err := f.Fetch(ctx, mustParse(srv.URL))
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
	if elapsed > 1*time.Second {
		t.Errorf("cancelled context should return fast, took %v", elapsed)
	}
}

func TestHTTPFetcher_UserAgent(t *testing.T) {
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		_, _ = fmt.Fprint(w, "ok")
	}))
	defer srv.Close()

	f := NewHTTPFetcher(100, 5*time.Second, "my-custom-bot/2.0")
	_, _, _, err := f.Fetch(context.Background(), mustParse(srv.URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotUA != "my-custom-bot/2.0" {
		t.Errorf("expected User-Agent 'my-custom-bot/2.0', got %q", gotUA)
	}
}

func TestHTTPFetcher_RobotsDisallowed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			_, _ = fmt.Fprint(w, "User-agent: *\nDisallow: /secret/\n")
			return
		}
		_, _ = fmt.Fprint(w, "ok")
	}))
	defer srv.Close()

	f := NewHTTPFetcher(100, 5*time.Second, "test-bot/1.0")
	disallowed := mustParse(srv.URL + "/secret/page")
	if f.IsAllowed(context.Background(), disallowed) {
		t.Error("expected /secret/page to be disallowed by robots.txt")
	}
}

func TestHTTPFetcher_RobotsAllowed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			_, _ = fmt.Fprint(w, "User-agent: *\nDisallow: /secret/\n")
			return
		}
		_, _ = fmt.Fprint(w, "ok")
	}))
	defer srv.Close()

	f := NewHTTPFetcher(100, 5*time.Second, "test-bot/1.0")
	allowed := mustParse(srv.URL + "/public/page")
	if !f.IsAllowed(context.Background(), allowed) {
		t.Error("expected /public/page to be allowed by robots.txt")
	}
}

func TestHTTPFetcher_RobotsMissing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			http.NotFound(w, r)
			return
		}
		_, _ = fmt.Fprint(w, "ok")
	}))
	defer srv.Close()

	f := NewHTTPFetcher(100, 5*time.Second, "test-bot/1.0")
	u := mustParse(srv.URL + "/any/page")
	if !f.IsAllowed(context.Background(), u) {
		t.Error("expected all URLs to be allowed when robots.txt is missing")
	}
}

func TestHTTPFetcher_RobotsOncePerHost(t *testing.T) {
	var robotsRequests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			robotsRequests.Add(1)
			_, _ = fmt.Fprint(w, "User-agent: *\nDisallow: /secret/\n")
			return
		}
		_, _ = fmt.Fprint(w, "ok")
	}))
	defer srv.Close()

	f := NewHTTPFetcher(1000, 5*time.Second, "test-bot/1.0")
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			f.IsAllowed(context.Background(), mustParse(srv.URL+"/page"))
		}()
	}
	wg.Wait()

	got := robotsRequests.Load()
	if got != 1 {
		t.Errorf("expected exactly 1 robots.txt request, got %d", got)
	}
}

func TestHTTPFetcher_RateLimiting(t *testing.T) {
	var requestCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		_, _ = fmt.Fprint(w, "ok")
	}))
	defer srv.Close()

	rps := 10.0
	f := NewHTTPFetcher(rps, 5*time.Second, "test-bot/1.0")
	n := 5

	start := time.Now()
	for i := 0; i < n; i++ {
		_, _, _, err := f.Fetch(context.Background(), mustParse(srv.URL))
		if err != nil {
			t.Fatalf("request %d failed: %v", i, err)
		}
	}
	elapsed := time.Since(start)

	got := int(requestCount.Load())
	if got != n {
		t.Errorf("expected %d requests, got %d", n, got)
	}
	// With rate limiting at 10 rps, 5 requests should take at least ~400ms
	// (first request is immediate, then 4 waits of ~100ms each).
	minExpected := time.Duration(float64(n-1)/rps*1000) * time.Millisecond * 8 / 10
	if elapsed < minExpected {
		t.Errorf("rate limiting too fast: %d requests in %v (expected at least %v)", n, elapsed, minExpected)
	}
}

func TestHTTPFetcher_LargeBodyTruncated(t *testing.T) {
	largeBody := strings.Repeat("x", 20*1024*1024) // 20 MB
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, largeBody)
	}))
	defer srv.Close()

	f := NewHTTPFetcher(100, 30*time.Second, "test-bot/1.0")
	body, status, _, err := f.Fetch(context.Background(), mustParse(srv.URL))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != 200 {
		t.Errorf("expected status 200, got %d", status)
	}
	if int64(len(body)) > defaultMaxBodySize {
		t.Errorf("body should be capped at %d bytes, got %d", defaultMaxBodySize, len(body))
	}
}
