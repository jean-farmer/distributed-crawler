// Package output provides serializers for the crawl results.
package output

import (
	"encoding/json"

	"github.com/jnfarmer/distributed-crawl/sitemap"
)

// JSON serializes a SiteMap to indented JSON.
func JSON(sm *sitemap.SiteMap) ([]byte, error) {
	return json.MarshalIndent(sm, "", "  ")
}
