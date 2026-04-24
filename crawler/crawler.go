// Package crawler implements the BFS crawl orchestration loop.
package crawler

import (
	"context"
	"errors"
	"net/url"
	"sync"
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

type crawlJob struct {
	url   *url.URL
	depth int
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
	if c.cfg.Workers < 1 {
		return nil, errors.New("workers must be at least 1")
	}

	seedURL, err := url.Parse(c.cfg.Seed)
	if err != nil {
		return nil, err
	}

	fr, err := frontier.New(seedURL, c.cfg.MaxDepth, c.cfg.MaxPages)
	if err != nil {
		return nil, err
	}

	start := time.Now()

	jobs := make(chan crawlJob, c.cfg.Workers)
	results := make(chan sitemap.CrawlResult, c.cfg.Workers)

	var wg sync.WaitGroup
	for range c.cfg.Workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				result := c.processURL(ctx, job)
				results <- result
			}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var pages []sitemap.Page
	inFlight := 0

	pending, hasPending := dequeueJob(fr)

dispatch:
	for hasPending || inFlight > 0 {
		var jobsCh chan crawlJob
		if hasPending {
			jobsCh = jobs
		}

		select {
		case jobsCh <- pending:
			inFlight++
			pending, hasPending = dequeueJob(fr)
		case res := <-results:
			inFlight--
			pages = append(pages, res.Page)
			fr.Add(res.DiscoveredURLs, res.Page.Depth+1)
			if !hasPending {
				pending, hasPending = dequeueJob(fr)
			}
		case <-ctx.Done():
			break dispatch
		}
	}

	close(jobs)
	for res := range results {
		pages = append(pages, res.Page)
	}
	return c.buildSiteMap(seedURL, pages, start), nil
}

func dequeueJob(fr *frontier.Frontier) (crawlJob, bool) {
	u, depth, ok := fr.Next()
	if !ok {
		return crawlJob{}, false
	}
	return crawlJob{url: u, depth: depth}, true
}

func (c *Crawler) processURL(ctx context.Context, job crawlJob) sitemap.CrawlResult {
	page := sitemap.Page{
		URL:   job.url.String(),
		Depth: job.depth,
	}

	if !c.fetcher.IsAllowed(ctx, job.url) {
		return sitemap.CrawlResult{Page: page}
	}

	body, statusCode, contentType, fetchErr := c.fetcher.Fetch(ctx, job.url)
	page.StatusCode = statusCode
	page.ContentType = contentType

	if fetchErr != nil {
		page.Error = fetchErr.Error()
		return sitemap.CrawlResult{Page: page}
	}

	links, _ := parser.ExtractLinks(body, job.url)
	for _, link := range links {
		page.Links = append(page.Links, sitemap.Link{
			URL:  link.String(),
			Text: "",
		})
	}

	return sitemap.CrawlResult{Page: page, DiscoveredURLs: links}
}

func (c *Crawler) buildSiteMap(seed *url.URL, pages []sitemap.Page, start time.Time) *sitemap.SiteMap {
	if pages == nil {
		pages = []sitemap.Page{}
	}

	statusByURL := make(map[string]int, len(pages))
	for _, p := range pages {
		statusByURL[p.URL] = p.StatusCode
	}

	var brokenLinks []sitemap.Link
	for _, p := range pages {
		for _, link := range p.Links {
			if code, ok := statusByURL[link.URL]; ok && (code < 200 || code >= 400) {
				link.Broken = true
				brokenLinks = append(brokenLinks, link)
			}
		}
	}
	if brokenLinks == nil {
		brokenLinks = []sitemap.Link{}
	}

	crawled := 0
	for _, p := range pages {
		if p.StatusCode > 0 {
			crawled++
		}
	}

	return &sitemap.SiteMap{
		Seed:        seed.String(),
		Domain:      seed.Host,
		Pages:       pages,
		BrokenLinks: brokenLinks,
		Stats: sitemap.Stats{
			PagesFound:   len(pages),
			PagesCrawled: crawled,
			BrokenCount:  len(brokenLinks),
			Duration:     time.Since(start),
		},
	}
}
