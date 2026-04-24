package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jnfarmer/distributed-crawl/crawler"
	"github.com/jnfarmer/distributed-crawl/internal/testutil"
	"github.com/jnfarmer/distributed-crawl/output"
	"github.com/jnfarmer/distributed-crawl/sitemap"
)

func fivePageSite() map[string]testutil.FakePage {
	return map[string]testutil.FakePage{
		"https://example.com/": {Body: `<html><body>
			<a href="https://example.com/about">About</a>
			<a href="https://example.com/blog">Blog</a>
			<a href="https://example.com/broken">Broken</a>
		</body></html>`},
		"https://example.com/about": {Body: `<html><body>
			<a href="https://example.com/">Home</a>
			<a href="https://example.com/contact">Contact</a>
		</body></html>`},
		"https://example.com/blog": {Body: `<html><body>
			<a href="https://example.com/">Home</a>
		</body></html>`},
		"https://example.com/contact": {Body: `<html><body>
			<a href="https://example.com/about">About</a>
		</body></html>`},
		"https://example.com/broken": {StatusCode: 404, Body: `<html><body>not found</body></html>`},
	}
}

func TestIntegration_FullCrawl(t *testing.T) {
	f := &testutil.FakeFetcher{Pages: fivePageSite()}
	cfg := crawler.Config{Workers: 3, MaxDepth: 10, MaxPages: 100, Seed: "https://example.com/"}
	sm, err := crawler.New(cfg, f).Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sm.Pages) != 5 {
		t.Errorf("expected 5 pages, got %d", len(sm.Pages))
	}

	urls := make(map[string]sitemap.Page)
	for _, p := range sm.Pages {
		urls[p.URL] = p
	}

	if urls["https://example.com/"].Depth != 0 {
		t.Errorf("expected seed at depth 0, got %d", urls["https://example.com/"].Depth)
	}
	if urls["https://example.com/about"].Depth != 1 {
		t.Errorf("expected /about at depth 1, got %d", urls["https://example.com/about"].Depth)
	}
	if urls["https://example.com/contact"].Depth != 2 {
		t.Errorf("expected /contact at depth 2, got %d", urls["https://example.com/contact"].Depth)
	}

	seed := urls["https://example.com/"]
	if len(seed.Links) != 3 {
		t.Errorf("expected seed to have 3 links, got %d", len(seed.Links))
	}
	if seed.ContentType != "text/html" {
		t.Errorf("expected seed content type text/html, got %s", seed.ContentType)
	}

	about := urls["https://example.com/about"]
	if len(about.Links) != 2 {
		t.Errorf("expected /about to have 2 links, got %d", len(about.Links))
	}

	brokenPage, ok := urls["https://example.com/broken"]
	if !ok {
		t.Fatal("expected /broken in results")
	}
	if brokenPage.StatusCode != 404 {
		t.Errorf("expected /broken status 404, got %d", brokenPage.StatusCode)
	}

	if sm.Stats.PagesFound != 5 {
		t.Errorf("expected stats.pages_found=5, got %d", sm.Stats.PagesFound)
	}
	if sm.Stats.PagesCrawled != 5 {
		t.Errorf("expected stats.pages_crawled=5, got %d", sm.Stats.PagesCrawled)
	}
	if sm.Stats.Duration <= 0 {
		t.Error("expected positive duration")
	}
	if sm.Stats.BrokenCount != 1 {
		t.Errorf("expected stats.broken_count=1, got %d", sm.Stats.BrokenCount)
	}
	if len(sm.BrokenLinks) != 1 {
		t.Errorf("expected 1 broken link, got %d", len(sm.BrokenLinks))
	}
}

func TestIntegration_JSONOutput(t *testing.T) {
	f := &testutil.FakeFetcher{Pages: fivePageSite()}
	cfg := crawler.Config{Workers: 3, MaxDepth: 10, MaxPages: 100, Seed: "https://example.com/"}
	sm, err := crawler.New(cfg, f).Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := output.JSON(sm)
	if err != nil {
		t.Fatalf("JSON error: %v", err)
	}

	var decoded sitemap.SiteMap
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("JSON output is not valid: %v", err)
	}

	if decoded.Seed != "https://example.com/" {
		t.Errorf("expected seed https://example.com/, got %s", decoded.Seed)
	}
	if decoded.Domain != "example.com" {
		t.Errorf("expected domain example.com, got %s", decoded.Domain)
	}
	if len(decoded.Pages) != 5 {
		t.Errorf("expected 5 pages in JSON, got %d", len(decoded.Pages))
	}
	if decoded.Pages == nil {
		t.Error("expected pages array, got null")
	}
	if decoded.BrokenLinks == nil {
		t.Error("expected broken_links array, got null")
	}
	if len(decoded.BrokenLinks) != 1 {
		t.Errorf("expected 1 broken link, got %d", len(decoded.BrokenLinks))
	}
}

func TestIntegration_DOTOutput(t *testing.T) {
	f := &testutil.FakeFetcher{Pages: fivePageSite()}
	cfg := crawler.Config{Workers: 3, MaxDepth: 10, MaxPages: 100, Seed: "https://example.com/"}
	sm, err := crawler.New(cfg, f).Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := output.DOT(sm)
	if err != nil {
		t.Fatalf("DOT error: %v", err)
	}

	out := string(data)

	if !strings.HasPrefix(out, "digraph") {
		t.Error("expected DOT output to start with 'digraph'")
	}

	for _, page := range sm.Pages {
		if !strings.Contains(out, fmt.Sprintf("%q", page.URL)) {
			t.Errorf("expected node %q in DOT output", page.URL)
		}
	}

	if !strings.Contains(out, "->") {
		t.Error("expected at least one edge in DOT output")
	}
}

func TestIntegration_MaxDepthHonored(t *testing.T) {
	pages := make(map[string]testutil.FakePage)
	pages["https://example.com/"] = testutil.FakePage{
		Body: `<html><body><a href="https://example.com/d1">d1</a></body></html>`,
	}
	for i := 1; i <= 10; i++ {
		u := fmt.Sprintf("https://example.com/d%d", i)
		next := fmt.Sprintf("https://example.com/d%d", i+1)
		pages[u] = testutil.FakePage{
			Body: fmt.Sprintf(`<html><body><a href="%s">next</a></body></html>`, next),
		}
	}

	f := &testutil.FakeFetcher{Pages: pages}
	cfg := crawler.Config{Workers: 3, MaxDepth: 2, MaxPages: 100, Seed: "https://example.com/"}
	sm, err := crawler.New(cfg, f).Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, p := range sm.Pages {
		if p.Depth > 2 {
			t.Errorf("page %s at depth %d exceeds maxDepth=2", p.URL, p.Depth)
		}
	}
	if len(sm.Pages) != 3 {
		t.Errorf("expected 3 pages (depth 0,1,2), got %d", len(sm.Pages))
	}
}

func TestIntegration_MaxPagesHonored(t *testing.T) {
	pages := make(map[string]testutil.FakePage)
	seedBody := `<html><body>`
	for i := 0; i < 20; i++ {
		u := fmt.Sprintf("https://example.com/p%d", i)
		seedBody += fmt.Sprintf(`<a href="%s">p%d</a>`, u, i)
		pages[u] = testutil.FakePage{Body: `<html><body>page</body></html>`}
	}
	seedBody += `</body></html>`
	pages["https://example.com/"] = testutil.FakePage{Body: seedBody}

	f := &testutil.FakeFetcher{Pages: pages}
	cfg := crawler.Config{Workers: 3, MaxDepth: 10, MaxPages: 5, Seed: "https://example.com/"}
	sm, err := crawler.New(cfg, f).Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sm.Pages) > 5 {
		t.Errorf("expected at most 5 pages, got %d", len(sm.Pages))
	}
}

func TestIntegration_ContextCancellation(t *testing.T) {
	pages := make(map[string]testutil.FakePage)
	seedBody := `<html><body>`
	for i := 0; i < 200; i++ {
		u := fmt.Sprintf("https://example.com/p%d", i)
		seedBody += fmt.Sprintf(`<a href="%s">link</a>`, u)
		pages[u] = testutil.FakePage{Body: `<html><body>page</body></html>`}
	}
	seedBody += `</body></html>`
	pages["https://example.com/"] = testutil.FakePage{Body: seedBody}

	f := &testutil.FakeFetcher{Pages: pages, Delay: 5 * time.Millisecond}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	cfg := crawler.Config{Workers: 3, MaxDepth: 10, MaxPages: 1000, Seed: "https://example.com/"}
	sm, err := crawler.New(cfg, f).Run(ctx)
	if err != nil {
		t.Fatalf("expected partial results, not error: %v", err)
	}

	if len(sm.Pages) == 0 {
		t.Error("expected at least some pages before cancellation")
	}
	if len(sm.Pages) >= 201 {
		t.Errorf("expected cancellation to stop crawl early, got %d pages", len(sm.Pages))
	}
}
