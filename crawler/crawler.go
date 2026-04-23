// Package crawler implements the BFS crawl orchestration loop.
package crawler

import (
	"context"
	"net/url"
	"time"

	"github.com/jnfarmer/distributed-crawl/fetcher"
	"github.com/jnfarmer/distributed-crawl/frontier"
	"github.com/jnfarmer/distributed-crawl/parser"
	"github.com/jnfarmer/distributed-crawl/sitemap"
)

// Config holds the crawler's configuration.
type Config struct {
	Workers  int
	MaxDepth int
	MaxPages int
	Seed     string
}

// Crawler coordinates the BFS crawl.
type Crawler struct {
	cfg     Config
	fetcher fetcher.Fetcher
}

// New creates a Crawler with the given configuration and fetcher.
func New(cfg Config, f fetcher.Fetcher) *Crawler {
	return &Crawler{cfg: cfg, fetcher: f}
}

// Run executes the crawl and returns the completed SiteMap.
func (c *Crawler) Run(ctx context.Context) (*sitemap.SiteMap, error) {
	seedURL, err := url.Parse(c.cfg.Seed)
	if err != nil {
		return nil, err
	}

	fr, err := frontier.New(seedURL, c.cfg.MaxDepth, c.cfg.MaxPages)
	if err != nil {
		return nil, err
	}

	start := time.Now()
	var pages []sitemap.Page

	for {
		select {
		case <-ctx.Done():
			return c.buildSiteMap(seedURL, pages, start), nil
		default:
		}

		u, depth, ok := fr.Next()
		if !ok {
			break
		}

		if !c.fetcher.IsAllowed(ctx, u) {
			continue
		}

		body, statusCode, contentType, fetchErr := c.fetcher.Fetch(ctx, u)

		page := sitemap.Page{
			URL:         u.String(),
			Depth:       depth,
			StatusCode:  statusCode,
			ContentType: contentType,
		}

		if fetchErr != nil {
			page.Error = fetchErr.Error()
			pages = append(pages, page)
			continue
		}

		links, _ := parser.ExtractLinks(body, u)
		for _, link := range links {
			page.Links = append(page.Links, sitemap.Link{
				URL:  link.String(),
				Text: "",
			})
		}

		fr.Add(links, depth+1)
		pages = append(pages, page)
	}

	return c.buildSiteMap(seedURL, pages, start), nil
}

func (c *Crawler) buildSiteMap(seed *url.URL, pages []sitemap.Page, start time.Time) *sitemap.SiteMap {
	sm := &sitemap.SiteMap{
		Seed:   seed.String(),
		Domain: seed.Host,
		Pages:  pages,
		Stats: sitemap.Stats{
			PagesFound:   len(pages),
			PagesCrawled: len(pages),
			Duration:     time.Since(start),
		},
	}
	if sm.Pages == nil {
		sm.Pages = []sitemap.Page{}
	}
	return sm
}
