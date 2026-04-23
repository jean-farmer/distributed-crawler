// Package parser extracts and normalizes links from HTML pages.
package parser

import (
	"bytes"
	"net/url"
	"sort"
	"strings"

	"golang.org/x/net/html"
)

// ExtractLinks parses HTML and returns all absolute HTTP(S) URLs found in anchor tags.
func ExtractLinks(body []byte, base *url.URL) ([]*url.URL, error) {
	tokenizer := html.NewTokenizer(bytes.NewReader(body))
	var links []*url.URL

	for {
		tt := tokenizer.Next()
		if tt == html.ErrorToken {
			break
		}
		if tt != html.StartTagToken && tt != html.SelfClosingTagToken {
			continue
		}
		tn, hasAttr := tokenizer.TagName()
		if string(tn) != "a" || !hasAttr {
			continue
		}

		var href string
		for {
			key, val, more := tokenizer.TagAttr()
			if string(key) == "href" {
				href = strings.TrimSpace(string(val))
			}
			if !more {
				break
			}
		}

		if href == "" {
			continue
		}

		parsed, err := url.Parse(href)
		if err != nil {
			continue
		}

		resolved := base.ResolveReference(parsed)

		if resolved.Scheme != "http" && resolved.Scheme != "https" {
			continue
		}

		// Skip fragment-only links (same page anchors)
		if resolved.Host == base.Host && resolved.Path == base.Path && resolved.RawQuery == base.RawQuery && parsed.Host == "" && parsed.Path == "" {
			continue
		}

		resolved.Fragment = ""
		resolved.RawFragment = ""

		links = append(links, resolved)
	}

	return links, nil
}

// Normalize canonicalizes a URL for deduplication.
func Normalize(u *url.URL) string {
	normalized := *u
	normalized.Scheme = strings.ToLower(normalized.Scheme)
	normalized.Host = strings.ToLower(normalized.Host)
	normalized.Fragment = ""
	normalized.RawFragment = ""

	if normalized.Path != "/" {
		normalized.Path = strings.TrimRight(normalized.Path, "/")
	}

	if normalized.RawQuery != "" {
		params := normalized.Query()
		keys := make([]string, 0, len(params))
		for k := range params {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var parts []string
		for _, k := range keys {
			for _, v := range params[k] {
				parts = append(parts, url.QueryEscape(k)+"="+url.QueryEscape(v))
			}
		}
		normalized.RawQuery = strings.Join(parts, "&")
	}

	return normalized.String()
}
