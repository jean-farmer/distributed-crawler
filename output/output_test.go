package output

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/jnfarmer/distributed-crawl/sitemap"
)

func sampleSiteMap() *sitemap.SiteMap {
	return &sitemap.SiteMap{
		Seed:   "https://example.com/",
		Domain: "example.com",
		Pages: []sitemap.Page{
			{
				URL:         "https://example.com/",
				Depth:       0,
				StatusCode:  200,
				ContentType: "text/html",
				Links: []sitemap.Link{
					{URL: "https://example.com/about", Text: "About"},
					{URL: "https://example.com/broken", Text: "Broken", Broken: true},
				},
			},
			{
				URL:         "https://example.com/about",
				Depth:       1,
				StatusCode:  200,
				ContentType: "text/html",
			},
			{
				URL:         "https://example.com/broken",
				Depth:       1,
				StatusCode:  404,
				ContentType: "text/html",
				Error:       "not found",
			},
		},
		BrokenLinks: []sitemap.Link{
			{URL: "https://example.com/broken", Text: "Broken", Broken: true},
		},
		Stats: sitemap.Stats{
			PagesFound:   3,
			PagesCrawled: 3,
			BrokenCount:  1,
			Duration:     2*time.Second + 500*time.Millisecond,
		},
	}
}

func TestJSON_BasicOutput(t *testing.T) {
	sm := sampleSiteMap()
	data, err := JSON(sm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var decoded sitemap.SiteMap
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if decoded.Seed != sm.Seed {
		t.Errorf("expected seed %s, got %s", sm.Seed, decoded.Seed)
	}
	if decoded.Domain != sm.Domain {
		t.Errorf("expected domain %s, got %s", sm.Domain, decoded.Domain)
	}
	if len(decoded.Pages) != len(sm.Pages) {
		t.Errorf("expected %d pages, got %d", len(sm.Pages), len(decoded.Pages))
	}
	if decoded.Stats.PagesFound != sm.Stats.PagesFound {
		t.Errorf("expected pages_found %d, got %d", sm.Stats.PagesFound, decoded.Stats.PagesFound)
	}
}

func TestJSON_BrokenLinksIncluded(t *testing.T) {
	sm := sampleSiteMap()
	data, err := JSON(sm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var decoded sitemap.SiteMap
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if len(decoded.BrokenLinks) != 1 {
		t.Fatalf("expected 1 broken link, got %d", len(decoded.BrokenLinks))
	}
	if decoded.BrokenLinks[0].URL != "https://example.com/broken" {
		t.Errorf("expected broken link URL https://example.com/broken, got %s", decoded.BrokenLinks[0].URL)
	}
	if !decoded.BrokenLinks[0].Broken {
		t.Error("expected broken link to have Broken=true")
	}
}

func TestJSON_EmptyCrawl(t *testing.T) {
	sm := &sitemap.SiteMap{
		Seed:        "https://example.com/",
		Domain:      "example.com",
		Pages:       []sitemap.Page{},
		BrokenLinks: []sitemap.Link{},
		Stats: sitemap.Stats{
			Duration: time.Second,
		},
	}
	data, err := JSON(sm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var decoded sitemap.SiteMap
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if decoded.Pages == nil {
		t.Error("expected empty pages array, got null")
	}
	if decoded.BrokenLinks == nil {
		t.Error("expected empty broken_links array, got null")
	}
}

func TestJSON_DurationIsHumanReadable(t *testing.T) {
	sm := sampleSiteMap()
	data, err := JSON(sm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	raw := string(data)
	if !strings.Contains(raw, `"duration"`) {
		t.Error("expected duration field in JSON output")
	}
	if !strings.Contains(raw, "2.5s") {
		t.Errorf("expected human-readable duration '2.5s' in output, got: %s", raw)
	}
}

func TestDOT_BasicGraph(t *testing.T) {
	sm := sampleSiteMap()
	data, err := DOT(sm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := string(data)

	if !strings.HasPrefix(out, "digraph") {
		t.Error("expected DOT output to start with 'digraph'")
	}

	if !strings.Contains(out, `"https://example.com/" -> "https://example.com/about"`) {
		t.Error("expected edge from root to /about")
	}
	if !strings.Contains(out, `"https://example.com/" -> "https://example.com/broken"`) {
		t.Error("expected edge from root to /broken")
	}
}

func TestDOT_BrokenLinksRed(t *testing.T) {
	sm := sampleSiteMap()
	data, err := DOT(sm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := string(data)

	if !strings.Contains(out, `"https://example.com/" -> "https://example.com/broken" [color=red]`) {
		t.Error("expected broken link edge to have [color=red]")
	}
	if strings.Contains(out, `"https://example.com/" -> "https://example.com/about" [color=red]`) {
		t.Error("non-broken link should not have [color=red]")
	}
}

func TestDOT_EmptyGraph(t *testing.T) {
	sm := &sitemap.SiteMap{
		Seed:   "https://example.com/",
		Domain: "example.com",
		Pages:  []sitemap.Page{},
		Stats: sitemap.Stats{
			Duration: time.Second,
		},
	}
	data, err := DOT(sm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := string(data)

	if !strings.HasPrefix(out, "digraph") {
		t.Error("expected DOT output to start with 'digraph'")
	}
	if strings.Contains(out, "->") {
		t.Error("expected no edges in empty graph")
	}
}

func TestJSON_NilSiteMap(t *testing.T) {
	_, err := JSON(nil)
	if err == nil {
		t.Fatal("expected error for nil SiteMap, got nil")
	}
}

func TestDOT_NilSiteMap(t *testing.T) {
	_, err := DOT(nil)
	if err == nil {
		t.Fatal("expected error for nil SiteMap, got nil")
	}
}

func TestJSON_NilSlicesNormalized(t *testing.T) {
	sm := &sitemap.SiteMap{
		Seed:   "https://example.com/",
		Domain: "example.com",
		Stats: sitemap.Stats{
			Duration: time.Second,
		},
	}
	data, err := JSON(sm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	raw := string(data)
	if strings.Contains(raw, `"pages": null`) {
		t.Error("expected pages to be [] not null when nil input")
	}
	if strings.Contains(raw, `"broken_links": null`) {
		t.Error("expected broken_links to be [] not null when nil input")
	}
}

func TestDOT_IsolatedNodeAppears(t *testing.T) {
	sm := &sitemap.SiteMap{
		Seed:   "https://example.com/",
		Domain: "example.com",
		Pages: []sitemap.Page{
			{URL: "https://example.com/leaf", Depth: 0, StatusCode: 200},
		},
	}
	data, err := DOT(sm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := string(data)
	if !strings.Contains(out, `"https://example.com/leaf"`) {
		t.Error("expected isolated node to appear in DOT output")
	}
	if strings.Contains(out, "->") {
		t.Error("expected no edges for page with no links")
	}
}
