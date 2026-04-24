// Package main is the CLI entry point for the sitemap crawler.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/jnfarmer/distributed-crawl/crawler"
	"github.com/jnfarmer/distributed-crawl/fetcher"
	"github.com/jnfarmer/distributed-crawl/output"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	seedURL := flag.String("url", "", "seed URL to crawl (required)")
	workers := flag.Int("workers", 5, "number of concurrent workers")
	maxDepth := flag.Int("depth", 10, "maximum BFS depth")
	maxPages := flag.Int("pages", 100, "maximum pages to crawl")
	rps := flag.Float64("rps", 2.0, "max requests per second")
	timeout := flag.Duration("timeout", 10*time.Second, "HTTP request timeout")
	userAgent := flag.String("ua", "sitemap-bot/1.0", "User-Agent string")
	format := flag.String("format", "json", "output format: json or dot")
	flag.Parse()

	if *seedURL == "" {
		flag.Usage()
		return errors.New("-url is required")
	}

	if *format != "json" && *format != "dot" {
		return fmt.Errorf("unsupported format %q (use json or dot)", *format)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	fmt.Fprintf(os.Stderr, "crawling %s (workers=%d depth=%d pages=%d rps=%.1f)\n",
		*seedURL, *workers, *maxDepth, *maxPages, *rps)

	f := fetcher.NewHTTPFetcher(*rps, *timeout, *userAgent)

	cfg := crawler.Config{
		Workers:  *workers,
		MaxDepth: *maxDepth,
		MaxPages: *maxPages,
		Seed:     *seedURL,
	}

	sm, err := crawler.New(cfg, f).Run(ctx)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "done: %d pages crawled, %d broken links, %s\n",
		sm.Stats.PagesCrawled, sm.Stats.BrokenCount, sm.Stats.Duration)

	var data []byte
	switch *format {
	case "json":
		data, err = output.JSON(sm)
	case "dot":
		data, err = output.DOT(sm)
	}
	if err != nil {
		return err
	}

	fmt.Println(string(data))
	return nil
}
