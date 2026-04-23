package fetcher

import (
	"context"
	"net/url"
)

type Fetcher interface {
	Fetch(ctx context.Context, u *url.URL) (body []byte, statusCode int, contentType string, err error)
	IsAllowed(ctx context.Context, u *url.URL) bool
}
