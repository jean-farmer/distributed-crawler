package parser

import (
	"net/url"
	"testing"
)

func mustParse(raw string) *url.URL {
	u, err := url.Parse(raw)
	if err != nil {
		panic(err)
	}
	return u
}

func TestExtractLinks(t *testing.T) {
	base := mustParse("https://example.com/page")
	body := []byte(`<html><body>
		<a href="https://example.com/about">About</a>
		<a href="https://example.com/contact">Contact</a>
	</body></html>`)

	links, err := ExtractLinks(body, base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(links) != 2 {
		t.Fatalf("expected 2 links, got %d", len(links))
	}
	if links[0].String() != "https://example.com/about" {
		t.Errorf("expected https://example.com/about, got %s", links[0])
	}
	if links[1].String() != "https://example.com/contact" {
		t.Errorf("expected https://example.com/contact, got %s", links[1])
	}
}

func TestExtractLinks_RelativeURLs(t *testing.T) {
	base := mustParse("https://example.com/blog/post")
	body := []byte(`<html><body>
		<a href="/about">About</a>
		<a href="other-post">Other</a>
		<a href="../index">Index</a>
	</body></html>`)

	links, err := ExtractLinks(body, base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(links) != 3 {
		t.Fatalf("expected 3 links, got %d", len(links))
	}
	expected := []string{
		"https://example.com/about",
		"https://example.com/blog/other-post",
		"https://example.com/index",
	}
	for i, want := range expected {
		if links[i].String() != want {
			t.Errorf("link[%d]: expected %s, got %s", i, want, links[i])
		}
	}
}

func TestExtractLinks_IgnoresNonHTTP(t *testing.T) {
	base := mustParse("https://example.com/")
	body := []byte(`<html><body>
		<a href="mailto:user@example.com">Email</a>
		<a href="javascript:void(0)">JS</a>
		<a href="tel:+1234567890">Phone</a>
		<a href="data:text/html,hello">Data</a>
		<a href="https://example.com/valid">Valid</a>
	</body></html>`)

	links, err := ExtractLinks(body, base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d: %v", len(links), links)
	}
	if links[0].String() != "https://example.com/valid" {
		t.Errorf("expected https://example.com/valid, got %s", links[0])
	}
}

func TestExtractLinks_HandlesFragments(t *testing.T) {
	base := mustParse("https://example.com/")
	body := []byte(`<html><body>
		<a href="https://example.com/page#section1">Link</a>
		<a href="#top">Top</a>
	</body></html>`)

	links, err := ExtractLinks(body, base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// #top is a same-page fragment, should be excluded (resolves to base with no path change)
	// /page#section1 should have fragment stripped
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d: %v", len(links), links)
	}
	if links[0].Fragment != "" {
		t.Errorf("expected fragment stripped, got %s", links[0])
	}
	if links[0].String() != "https://example.com/page" {
		t.Errorf("expected https://example.com/page, got %s", links[0])
	}
}

func TestExtractLinks_EmptyAndMalformed(t *testing.T) {
	base := mustParse("https://example.com/")
	body := []byte(`<html><body>
		<a>No href</a>
		<a href="">Empty</a>
		<a href="   ">Whitespace</a>
		<a href="https://example.com/ok">OK</a>
	</body></html>`)

	links, err := ExtractLinks(body, base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d: %v", len(links), links)
	}
	if links[0].String() != "https://example.com/ok" {
		t.Errorf("expected https://example.com/ok, got %s", links[0])
	}
}

func TestNormalize(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "lowercase host",
			raw:  "https://EXAMPLE.COM/path",
			want: "https://example.com/path",
		},
		{
			name: "remove trailing slash",
			raw:  "https://example.com/path/",
			want: "https://example.com/path",
		},
		{
			name: "strip fragment",
			raw:  "https://example.com/path#section",
			want: "https://example.com/path",
		},
		{
			name: "sort query params",
			raw:  "https://example.com/path?z=1&a=2",
			want: "https://example.com/path?a=2&z=1",
		},
		{
			name: "root path keeps slash",
			raw:  "https://example.com/",
			want: "https://example.com/",
		},
		{
			name: "lowercase scheme",
			raw:  "HTTPS://example.com/path",
			want: "https://example.com/path",
		},
		{
			name: "combined normalization",
			raw:  "HTTPS://EXAMPLE.COM/Path/?z=1&a=2#frag",
			want: "https://example.com/Path?a=2&z=1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := mustParse(tt.raw)
			got := Normalize(u)
			if got != tt.want {
				t.Errorf("Normalize(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestNormalize_Idempotent(t *testing.T) {
	raw := "HTTPS://EXAMPLE.COM/Path/?z=1&a=2#frag"
	u := mustParse(raw)
	first := Normalize(u)
	u2 := mustParse(first)
	second := Normalize(u2)
	if first != second {
		t.Errorf("not idempotent: first=%q, second=%q", first, second)
	}
}
