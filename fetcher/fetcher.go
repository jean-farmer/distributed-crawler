// Package fetcher defines the interface for fetching web pages.
package fetcher

import (
	"context"
	"net/url"
)

// Fetcher retrieves web pages and checks robots.txt permissions.
type Fetcher interface {
	Fetch(ctx context.Context, u *url.URL) (body []byte, statusCode int, contentType string, err error)
	IsAllowed(ctx context.Context, u *url.URL) bool
}
