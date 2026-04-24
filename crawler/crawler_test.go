package crawler

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jnfarmer/distributed-crawl/internal/testutil"
)

func TestCrawl_SinglePage(t *testing.T) {
	f := &testutil.FakeFetcher{
		Pages: map[string]testutil.FakePage{
			"https://example.com/": {Body: `<html><body>Hello</body></html>`},
		},
	}

	cfg := Config{
		Workers:  1,
		MaxDepth: 10,
		MaxPages: 100,
		Seed:     "https://example.com/",
	}
	sm, err := New(cfg, f).Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sm.Pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(sm.Pages))
	}
	if sm.Pages[0].URL != "https://example.com/" {
		t.Errorf("expected https://example.com/, got %s", sm.Pages[0].URL)
	}
	if sm.Pages[0].StatusCode != 200 {
		t.Errorf("expected status 200, got %d", sm.Pages[0].StatusCode)
	}
}

func TestCrawl_FollowsLinks(t *testing.T) {
	f := &testutil.FakeFetcher{
		Pages: map[string]testutil.FakePage{
			"https://example.com/":  {Body: `<html><body><a href="https://example.com/b">B</a></body></html>`},
			"https://example.com/b": {Body: `<html><body><a href="https://example.com/c">C</a></body></html>`},
			"https://example.com/c": {Body: `<html><body>end</body></html>`},
		},
	}

	cfg := Config{Workers: 1, MaxDepth: 10, MaxPages: 100, Seed: "https://example.com/"}
	sm, err := New(cfg, f).Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sm.Pages) != 3 {
		t.Fatalf("expected 3 pages, got %d", len(sm.Pages))
	}

	depths := make(map[string]int)
	for _, p := range sm.Pages {
		depths[p.URL] = p.Depth
	}
	if depths["https://example.com/"] != 0 {
		t.Errorf("expected seed at depth 0, got %d", depths["https://example.com/"])
	}
	if depths["https://example.com/b"] != 1 {
		t.Errorf("expected /b at depth 1, got %d", depths["https://example.com/b"])
	}
	if depths["https://example.com/c"] != 2 {
		t.Errorf("expected /c at depth 2, got %d", depths["https://example.com/c"])
	}
}

func TestCrawl_RespectsDomainBoundary(t *testing.T) {
	f := &testutil.FakeFetcher{
		Pages: map[string]testutil.FakePage{
			"https://example.com/":   {Body: `<html><body><a href="https://other.com/page">External</a></body></html>`},
			"https://other.com/page": {Body: `<html><body>external</body></html>`},
		},
	}

	cfg := Config{Workers: 1, MaxDepth: 10, MaxPages: 100, Seed: "https://example.com/"}
	sm, err := New(cfg, f).Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sm.Pages) != 1 {
		t.Errorf("expected 1 page (seed only, external rejected), got %d", len(sm.Pages))
	}
}

func TestCrawl_RespectsMaxDepth(t *testing.T) {
	f := &testutil.FakeFetcher{
		Pages: map[string]testutil.FakePage{
			"https://example.com/":  {Body: `<html><body><a href="https://example.com/a">A</a></body></html>`},
			"https://example.com/a": {Body: `<html><body><a href="https://example.com/b">B</a></body></html>`},
			"https://example.com/b": {Body: `<html><body><a href="https://example.com/c">C</a></body></html>`},
			"https://example.com/c": {Body: `<html><body>end</body></html>`},
		},
	}

	cfg := Config{Workers: 1, MaxDepth: 1, MaxPages: 100, Seed: "https://example.com/"}
	sm, err := New(cfg, f).Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, p := range sm.Pages {
		if p.Depth > 1 {
			t.Errorf("found page at depth %d (%s), maxDepth is 1", p.Depth, p.URL)
		}
	}
	if len(sm.Pages) != 2 {
		t.Errorf("expected 2 pages (depth 0 and 1), got %d", len(sm.Pages))
	}
}

func TestCrawl_RespectsMaxPages(t *testing.T) {
	pages := make(map[string]testutil.FakePage)
	for i := 0; i < 20; i++ {
		u := fmt.Sprintf("https://example.com/p%d", i)
		body := `<html><body>`
		for j := i + 1; j < 20; j++ {
			body += fmt.Sprintf(`<a href="https://example.com/p%d">link</a>`, j)
		}
		body += `</body></html>`
		pages[u] = testutil.FakePage{Body: body}
	}
	pages["https://example.com/"] = testutil.FakePage{
		Body: `<html><body><a href="https://example.com/p0">p0</a></body></html>`,
	}

	f := &testutil.FakeFetcher{Pages: pages}
	cfg := Config{Workers: 1, MaxDepth: 10, MaxPages: 5, Seed: "https://example.com/"}
	sm, err := New(cfg, f).Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sm.Pages) > 5 {
		t.Errorf("expected at most 5 pages, got %d", len(sm.Pages))
	}
}

func TestCrawl_HandlesErrors(t *testing.T) {
	f := &testutil.FakeFetcher{
		Pages: map[string]testutil.FakePage{
			"https://example.com/": {Body: `<html><body>
				<a href="https://example.com/ok">OK</a>
				<a href="https://example.com/fail">Fail</a>
			</body></html>`},
			"https://example.com/ok":   {Body: `<html><body>ok</body></html>`},
			"https://example.com/fail": {Err: errors.New("network error")},
		},
	}

	cfg := Config{Workers: 1, MaxDepth: 10, MaxPages: 100, Seed: "https://example.com/"}
	sm, err := New(cfg, f).Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var foundError bool
	for _, p := range sm.Pages {
		if p.URL == "https://example.com/fail" && p.Error != "" {
			foundError = true
		}
	}
	if !foundError {
		t.Error("expected error page for /fail")
	}
	if len(sm.Pages) != 3 {
		t.Errorf("expected 3 pages (seed + ok + fail), got %d", len(sm.Pages))
	}
}

func TestCrawl_ContextCancellation(t *testing.T) {
	seedBody := `<html><body>`
	for i := 0; i < 100; i++ {
		seedBody += fmt.Sprintf(`<a href="https://example.com/p%d">link</a>`, i)
	}
	seedBody += `</body></html>`

	pages := make(map[string]testutil.FakePage)
	pages["https://example.com/"] = testutil.FakePage{Body: seedBody}
	for i := 0; i < 100; i++ {
		u := fmt.Sprintf("https://example.com/p%d", i)
		pages[u] = testutil.FakePage{Body: `<html><body>page</body></html>`}
	}

	f := &testutil.FakeFetcher{Pages: pages, Delay: 5 * time.Millisecond}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	cfg := Config{Workers: 1, MaxDepth: 10, MaxPages: 1000, Seed: "https://example.com/"}
	sm, err := New(cfg, f).Run(ctx)

	if err != nil {
		t.Fatalf("Run should return partial results, not error: %v", err)
	}
	if len(sm.Pages) == 0 {
		t.Error("expected at least some pages before cancellation")
	}
	if len(sm.Pages) >= 101 {
		t.Errorf("expected cancellation to stop crawl early, got %d pages", len(sm.Pages))
	}
}

func TestCrawl_ConcurrentFaster(t *testing.T) {
	pages := make(map[string]testutil.FakePage)
	seedBody := `<html><body>`
	for i := 0; i < 10; i++ {
		u := fmt.Sprintf("https://example.com/p%d", i)
		seedBody += fmt.Sprintf(`<a href="%s">link</a>`, u)
		pages[u] = testutil.FakePage{Body: `<html><body>page</body></html>`}
	}
	seedBody += `</body></html>`
	pages["https://example.com/"] = testutil.FakePage{Body: seedBody}

	run := func(workers int) time.Duration {
		f := &testutil.FakeFetcher{Pages: pages, Delay: 50 * time.Millisecond}
		cfg := Config{Workers: workers, MaxDepth: 10, MaxPages: 100, Seed: "https://example.com/"}
		start := time.Now()
		sm, err := New(cfg, f).Run(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(sm.Pages) != 11 {
			t.Errorf("workers=%d: expected 11 pages, got %d", workers, len(sm.Pages))
		}
		return time.Since(start)
	}

	serial := run(1)
	parallel := run(5)

	if parallel >= serial {
		t.Errorf("concurrent (workers=5) should be faster than serial: serial=%v, parallel=%v", serial, parallel)
	}
	if float64(parallel) > float64(serial)*0.6 {
		t.Errorf("concurrent should be significantly faster: serial=%v, parallel=%v (ratio=%.2f)", serial, parallel, float64(parallel)/float64(serial))
	}
}

func TestCrawl_NoDuplicateFetches(t *testing.T) {
	pages := make(map[string]testutil.FakePage)
	seedBody := `<html><body>`
	for i := 0; i < 10; i++ {
		u := fmt.Sprintf("https://example.com/p%d", i)
		body := `<html><body>`
		for j := 0; j < 10; j++ {
			body += fmt.Sprintf(`<a href="https://example.com/p%d">link</a>`, j)
		}
		body += `</body></html>`
		pages[u] = testutil.FakePage{Body: body}
		seedBody += fmt.Sprintf(`<a href="%s">link</a>`, u)
	}
	seedBody += `</body></html>`
	pages["https://example.com/"] = testutil.FakePage{Body: seedBody}

	f := &testutil.FakeFetcher{Pages: pages}
	cfg := Config{Workers: 5, MaxDepth: 10, MaxPages: 100, Seed: "https://example.com/"}
	sm, err := New(cfg, f).Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	calls := f.GetCalls()
	seen := make(map[string]bool)
	for _, c := range calls {
		if seen[c] {
			t.Errorf("URL fetched more than once: %s", c)
		}
		seen[c] = true
	}

	if len(sm.Pages) != 11 {
		t.Errorf("expected 11 pages, got %d", len(sm.Pages))
	}
}
