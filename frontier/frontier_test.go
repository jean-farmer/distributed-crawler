package frontier

import (
	"fmt"
	"net/url"
	"sync"
	"testing"
)

func mustParse(raw string) *url.URL {
	u, err := url.Parse(raw)
	if err != nil {
		panic(err)
	}
	return u
}

func mustNew(t *testing.T, seed *url.URL, maxDepth, maxPages int) *Frontier {
	t.Helper()
	f, err := New(seed, maxDepth, maxPages)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	return f
}

func drainSeed(t *testing.T, f *Frontier) {
	t.Helper()
	u, depth, ok := f.Next()
	if !ok {
		t.Fatal("expected seed URL in frontier")
	}
	if depth != 0 {
		t.Errorf("expected seed at depth 0, got %d", depth)
	}
	_ = u
}

func TestNew_NilSeed(t *testing.T) {
	_, err := New(nil, 10, 100)
	if err == nil {
		t.Fatal("expected error for nil seed, got nil")
	}
}

func TestFrontier_BasicEnqueueDequeue(t *testing.T) {
	f := mustNew(t, mustParse("https://example.com/"), 10, 100)
	drainSeed(t, f)

	urls := []*url.URL{
		mustParse("https://example.com/a"),
		mustParse("https://example.com/b"),
		mustParse("https://example.com/c"),
	}
	f.Add(urls, 1)

	for _, want := range urls {
		u, depth, ok := f.Next()
		if !ok {
			t.Fatalf("expected URL, got empty")
		}
		if u.String() != want.String() {
			t.Errorf("expected %s, got %s", want, u)
		}
		if depth != 1 {
			t.Errorf("expected depth 1, got %d", depth)
		}
	}

	_, _, ok := f.Next()
	if ok {
		t.Error("expected empty frontier after dequeuing all URLs")
	}
}

func TestFrontier_Deduplication(t *testing.T) {
	f := mustNew(t, mustParse("https://example.com/"), 10, 100)
	drainSeed(t, f)

	same := mustParse("https://example.com/page")
	f.Add([]*url.URL{same}, 1)
	f.Add([]*url.URL{same}, 1)

	u, _, ok := f.Next()
	if !ok {
		t.Fatal("expected one URL")
	}
	if u.String() != same.String() {
		t.Errorf("expected %s, got %s", same, u)
	}

	_, _, ok = f.Next()
	if ok {
		t.Error("duplicate URL should not be enqueued twice")
	}
}

func TestFrontier_DomainFiltering(t *testing.T) {
	f := mustNew(t, mustParse("https://example.com/"), 10, 100)
	drainSeed(t, f)

	urls := []*url.URL{
		mustParse("https://example.com/ok"),
		mustParse("https://other.com/nope"),
		mustParse("https://sub.example.com/also-nope"),
	}
	added := f.Add(urls, 1)

	if added != 1 {
		t.Errorf("expected 1 URL added, got %d", added)
	}

	u, _, ok := f.Next()
	if !ok {
		t.Fatal("expected one URL")
	}
	if u.String() != "https://example.com/ok" {
		t.Errorf("expected https://example.com/ok, got %s", u)
	}

	_, _, ok = f.Next()
	if ok {
		t.Error("off-domain URLs should be rejected")
	}
}

func TestFrontier_DomainFiltering_CaseInsensitive(t *testing.T) {
	f := mustNew(t, mustParse("https://Example.COM/"), 10, 100)
	drainSeed(t, f)

	added := f.Add([]*url.URL{mustParse("https://example.com/page")}, 1)
	if added != 1 {
		t.Errorf("expected case-insensitive domain match to accept URL, got added=%d", added)
	}
}

func TestFrontier_DepthLimit(t *testing.T) {
	f := mustNew(t, mustParse("https://example.com/"), 2, 100)
	drainSeed(t, f)

	f.Add([]*url.URL{mustParse("https://example.com/depth1")}, 1)
	f.Add([]*url.URL{mustParse("https://example.com/depth2")}, 2)
	f.Add([]*url.URL{mustParse("https://example.com/depth3")}, 3)

	count := 0
	for {
		_, _, ok := f.Next()
		if !ok {
			break
		}
		count++
	}

	if count != 2 {
		t.Errorf("expected 2 URLs within depth limit, got %d", count)
	}
}

func TestFrontier_MaxPages(t *testing.T) {
	f := mustNew(t, mustParse("https://example.com/"), 10, 3)

	urls := []*url.URL{
		mustParse("https://example.com/a"),
		mustParse("https://example.com/b"),
		mustParse("https://example.com/c"),
		mustParse("https://example.com/d"),
		mustParse("https://example.com/e"),
	}
	added := f.Add(urls, 1)

	if added != 2 {
		t.Errorf("expected 2 URLs added (maxPages=3, seed=1), got %d", added)
	}
}

func TestFrontier_MaxPages_SubsequentAddReturnsZero(t *testing.T) {
	f := mustNew(t, mustParse("https://example.com/"), 10, 2)
	f.Add([]*url.URL{mustParse("https://example.com/a")}, 1)

	added := f.Add([]*url.URL{mustParse("https://example.com/b")}, 1)
	if added != 0 {
		t.Errorf("expected 0 URLs added after maxPages hit, got %d", added)
	}
}

func TestFrontier_EmptyReturnsNotOk(t *testing.T) {
	f := mustNew(t, mustParse("https://example.com/"), 10, 100)
	drainSeed(t, f)

	_, _, ok := f.Next()
	if ok {
		t.Error("expected ok=false from empty frontier")
	}
}

func TestFrontier_NormalizesBeforeDedup(t *testing.T) {
	f := mustNew(t, mustParse("https://example.com/"), 10, 100)
	drainSeed(t, f)

	f.Add([]*url.URL{mustParse("https://example.com/page")}, 1)
	f.Add([]*url.URL{mustParse("https://example.com/page/")}, 1)
	f.Add([]*url.URL{mustParse("https://EXAMPLE.COM/page")}, 1)

	count := 0
	for {
		_, _, ok := f.Next()
		if !ok {
			break
		}
		count++
	}

	if count != 1 {
		t.Errorf("expected 1 unique URL after normalization, got %d", count)
	}
}

func TestFrontier_ConcurrentAdd(t *testing.T) {
	f := mustNew(t, mustParse("https://example.com/"), 10, 1000)

	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(batch int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				u := mustParse(fmt.Sprintf("https://example.com/%d/%d", batch, j))
				f.Add([]*url.URL{u}, 1)
			}
		}(i)
	}

	wg.Wait()

	seen := make(map[string]bool)
	for {
		u, _, ok := f.Next()
		if !ok {
			break
		}
		s := u.String()
		if seen[s] {
			t.Errorf("duplicate URL dequeued: %s", s)
		}
		seen[s] = true
	}

	if len(seen) != 101 {
		t.Errorf("expected 101 unique URLs (seed + 100), got %d", len(seen))
	}
}

func TestFrontier_ConcurrentAddAndNext(t *testing.T) {
	f := mustNew(t, mustParse("https://example.com/"), 10, 10000)

	var wg sync.WaitGroup

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(batch int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				u := mustParse(fmt.Sprintf("https://example.com/%d/%d", batch, j))
				f.Add([]*url.URL{u}, 1)
			}
		}(i)
	}

	var dequeued int
	var dequeueMu sync.Mutex
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_, _, ok := f.Next()
				if ok {
					dequeueMu.Lock()
					dequeued++
					dequeueMu.Unlock()
				}
			}
		}()
	}

	wg.Wait()

	remaining := 0
	for {
		_, _, ok := f.Next()
		if !ok {
			break
		}
		remaining++
	}

	total := dequeued + remaining
	// 1 seed + 500 unique URLs = 501 total enqueued.
	if total != 501 {
		t.Errorf("expected 501 total dequeued (seed + 500), got %d (concurrent=%d, remaining=%d)", total, dequeued, remaining)
	}
}

func TestFrontier_Len(t *testing.T) {
	f := mustNew(t, mustParse("https://example.com/"), 10, 100)

	if f.Len() != 1 {
		t.Errorf("expected len 1 (seed), got %d", f.Len())
	}

	f.Add([]*url.URL{
		mustParse("https://example.com/a"),
		mustParse("https://example.com/b"),
	}, 1)

	if f.Len() != 3 {
		t.Errorf("expected len 3, got %d", f.Len())
	}

	f.Next()
	if f.Len() != 2 {
		t.Errorf("expected len 2 after dequeue, got %d", f.Len())
	}
}

func TestFrontier_SeedIsDeduped(t *testing.T) {
	f := mustNew(t, mustParse("https://example.com/"), 10, 100)

	added := f.Add([]*url.URL{mustParse("https://example.com/")}, 1)
	if added != 0 {
		t.Errorf("seed URL should be deduped, but %d were added", added)
	}
}
