// Package sitemap defines the shared data types used across the crawler.
package sitemap

import (
	"encoding/json"
	"net/url"
	"time"
)

// Page represents a single crawled URL and everything discovered about it.
type Page struct {
	URL         string `json:"url"`
	Depth       int    `json:"depth"`
	StatusCode  int    `json:"status_code"`
	ContentType string `json:"content_type"`
	Links       []Link `json:"links"`
	Error       string `json:"error,omitempty"`
}

// Link is a directed edge in the site graph from one page to another.
type Link struct {
	URL    string `json:"url"`
	Text   string `json:"text"`
	Broken bool   `json:"broken"`
}

// SiteMap is the top-level output document produced by a crawl.
type SiteMap struct {
	Seed        string `json:"seed"`
	Domain      string `json:"domain"`
	Pages       []Page `json:"pages"`
	BrokenLinks []Link `json:"broken_links"`
	Stats       Stats  `json:"stats"`
}

// Stats summarizes the crawl run.
type Stats struct {
	PagesFound   int           `json:"pages_found"`
	PagesCrawled int           `json:"pages_crawled"`
	BrokenCount  int           `json:"broken_count"`
	Duration     time.Duration `json:"-"`
}

// MarshalJSON serializes Stats with Duration as a human-readable string.
func (s Stats) MarshalJSON() ([]byte, error) {
	type Alias Stats
	return json.Marshal(struct {
		Alias
		Duration string `json:"duration"`
	}{
		Alias:    Alias(s),
		Duration: s.Duration.String(),
	})
}

// CrawlResult is what a worker sends back after processing a single URL.
type CrawlResult struct {
	Page           Page
	DiscoveredURLs []*url.URL
}
