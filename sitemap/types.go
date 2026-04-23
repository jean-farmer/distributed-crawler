package sitemap

import (
	"encoding/json"
	"net/url"
	"time"
)

type Page struct {
	URL         string `json:"url"`
	Depth       int    `json:"depth"`
	StatusCode  int    `json:"status_code"`
	ContentType string `json:"content_type"`
	Links       []Link `json:"links"`
	Error       string `json:"error,omitempty"`
}

type Link struct {
	URL    string `json:"url"`
	Text   string `json:"text"`
	Broken bool   `json:"broken"`
}

type SiteMap struct {
	Seed        string `json:"seed"`
	Domain      string `json:"domain"`
	Pages       []Page `json:"pages"`
	BrokenLinks []Link `json:"broken_links"`
	Stats       Stats  `json:"stats"`
}

type Stats struct {
	PagesFound   int           `json:"pages_found"`
	PagesCrawled int           `json:"pages_crawled"`
	BrokenCount  int           `json:"broken_count"`
	Duration     time.Duration `json:"-"`
}

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

type CrawlResult struct {
	Page           Page
	DiscoveredURLs []*url.URL
}
