// Package frontier provides a thread-safe BFS queue with deduplication and domain filtering.
package frontier

import (
	"net/url"
	"sync"

	"github.com/jnfarmer/distributed-crawl/parser"
)

type entry struct {
	url   *url.URL
	depth int
}

// Frontier is a thread-safe BFS queue with deduplication.
type Frontier struct {
	mu       sync.Mutex
	queue    []entry
	seen     map[string]bool
	domain   string
	maxDepth int
	maxPages int
	enqueued int
}

// New creates a Frontier seeded with the given URL.
func New(seed *url.URL, maxDepth, maxPages int) *Frontier {
	normalized := parser.Normalize(seed)
	f := &Frontier{
		seen:     map[string]bool{normalized: true},
		domain:   seed.Host,
		maxDepth: maxDepth,
		maxPages: maxPages,
		enqueued: 1,
	}
	f.queue = append(f.queue, entry{url: seed, depth: 0})
	return f
}

// Add enqueues URLs that pass domain, depth, dedup, and maxPages checks.
func (f *Frontier) Add(urls []*url.URL, depth int) int {
	f.mu.Lock()
	defer f.mu.Unlock()

	added := 0
	for _, u := range urls {
		if depth > f.maxDepth {
			continue
		}
		if u.Host != f.domain {
			continue
		}
		if f.enqueued >= f.maxPages {
			break
		}
		normalized := parser.Normalize(u)
		if f.seen[normalized] {
			continue
		}
		f.seen[normalized] = true
		f.queue = append(f.queue, entry{url: u, depth: depth})
		f.enqueued++
		added++
	}
	return added
}

// Next returns the next URL to crawl, its depth, and whether the queue was non-empty.
func (f *Frontier) Next() (*url.URL, int, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if len(f.queue) == 0 {
		return nil, 0, false
	}
	e := f.queue[0]
	f.queue = f.queue[1:]
	return e.url, e.depth, true
}

// Len returns the current queue length.
func (f *Frontier) Len() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.queue)
}
