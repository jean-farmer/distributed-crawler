// Package main is the CLI entry point for the sitemap crawler.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"
)

func main() {
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
		fmt.Fprintln(os.Stderr, "error: -url is required")
		flag.Usage()
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "config: url=%s workers=%d depth=%d pages=%d rps=%.1f timeout=%s ua=%s format=%s\n",
		*seedURL, *workers, *maxDepth, *maxPages, *rps, *timeout, *userAgent, *format)
}
