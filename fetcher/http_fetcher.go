package fetcher

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const defaultMaxBodySize = 10 * 1024 * 1024 // 10 MB

var _ Fetcher = (*HTTPFetcher)(nil)

// HTTPFetcher implements Fetcher using a real HTTP client with rate limiting and robots.txt.
type HTTPFetcher struct {
	client      *http.Client
	limiter     *rate.Limiter
	userAgent   string
	maxBodySize int64

	mu     sync.Mutex
	robots map[string]*robotsEntry
}

type robotsEntry struct {
	once sync.Once
	data *robotsData
}

type robotsData struct {
	disallowed []string
}

// NewHTTPFetcher creates a new HTTPFetcher with the given settings.
func NewHTTPFetcher(reqPerSec float64, timeout time.Duration, userAgent string) *HTTPFetcher {
	return &HTTPFetcher{
		client: &http.Client{
			Timeout: timeout,
		},
		limiter:     rate.NewLimiter(rate.Limit(reqPerSec), 1),
		userAgent:   userAgent,
		maxBodySize: defaultMaxBodySize,
		robots:      make(map[string]*robotsEntry),
	}
}

// Fetch retrieves the content at the given URL, respecting the rate limiter.
func (f *HTTPFetcher) Fetch(ctx context.Context, u *url.URL) (body []byte, statusCode int, contentType string, err error) {
	if err = f.limiter.Wait(ctx); err != nil {
		return nil, 0, "", err
	}

	req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if reqErr != nil {
		return nil, 0, "", fmt.Errorf("creating request: %w", reqErr)
	}
	req.Header.Set("User-Agent", f.userAgent)

	resp, doErr := f.client.Do(req)
	if doErr != nil {
		return nil, 0, "", doErr
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("closing response body: %w", cerr)
		}
	}()

	body, err = io.ReadAll(io.LimitReader(resp.Body, f.maxBodySize))
	if err != nil {
		return nil, 0, "", fmt.Errorf("reading body: %w", err)
	}

	return body, resp.StatusCode, resp.Header.Get("Content-Type"), nil
}

// IsAllowed checks robots.txt for the given URL.
func (f *HTTPFetcher) IsAllowed(ctx context.Context, u *url.URL) bool {
	rd := f.getRobots(ctx, u)
	if rd == nil {
		return true
	}
	for _, path := range rd.disallowed {
		if strings.HasPrefix(u.Path, path) {
			return false
		}
	}
	return true
}

func (f *HTTPFetcher) getRobots(ctx context.Context, u *url.URL) *robotsData {
	host := u.Host

	f.mu.Lock()
	entry, ok := f.robots[host]
	if !ok {
		entry = &robotsEntry{}
		f.robots[host] = entry
	}
	f.mu.Unlock()

	entry.once.Do(func() {
		entry.data = f.fetchRobots(ctx, u)
	})

	return entry.data
}

func (f *HTTPFetcher) fetchRobots(ctx context.Context, u *url.URL) *robotsData {
	robotsURL := &url.URL{
		Scheme: u.Scheme,
		Host:   u.Host,
		Path:   "/robots.txt",
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, robotsURL.String(), nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", f.userAgent)

	resp, err := f.client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			if cerr := resp.Body.Close(); cerr != nil {
				return nil
			}
		}
		return nil
	}

	rd := parseRobots(resp.Body)
	if err := resp.Body.Close(); err != nil {
		return rd
	}

	return rd
}

func parseRobots(r io.Reader) *robotsData {
	rd := &robotsData{}
	scanner := bufio.NewScanner(r)
	inMatchingGroup := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(strings.ToLower(parts[0]))
		value := strings.TrimSpace(parts[1])

		switch key {
		case "user-agent":
			inMatchingGroup = value == "*"
		case "disallow":
			if inMatchingGroup && value != "" {
				rd.disallowed = append(rd.disallowed, value)
			}
		}
	}
	return rd
}
