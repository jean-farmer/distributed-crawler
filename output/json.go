// Package output provides serializers for the crawl results.
package output

import (
	"encoding/json"
	"errors"

	"github.com/jnfarmer/distributed-crawl/sitemap"
)

// JSON serializes a SiteMap to indented JSON.
func JSON(sm *sitemap.SiteMap) ([]byte, error) {
	if sm == nil {
		return nil, errors.New("output: nil SiteMap")
	}
	if sm.Pages == nil {
		sm.Pages = []sitemap.Page{}
	}
	if sm.BrokenLinks == nil {
		sm.BrokenLinks = []sitemap.Link{}
	}
	return json.MarshalIndent(sm, "", "  ")
}
