package output

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/jnfarmer/distributed-crawl/sitemap"
)

// DOT serializes a SiteMap as a Graphviz DOT directed graph.
func DOT(sm *sitemap.SiteMap) ([]byte, error) {
	if sm == nil {
		return nil, errors.New("output: nil SiteMap")
	}

	var buf bytes.Buffer

	fmt.Fprintf(&buf, "digraph %q {\n", sm.Domain)
	fmt.Fprintf(&buf, "  rankdir=LR;\n")

	for _, page := range sm.Pages {
		fmt.Fprintf(&buf, "  %q;\n", page.URL)
	}

	for _, page := range sm.Pages {
		for _, link := range page.Links {
			if link.Broken {
				fmt.Fprintf(&buf, "  %q -> %q [color=red];\n", page.URL, link.URL)
			} else {
				fmt.Fprintf(&buf, "  %q -> %q;\n", page.URL, link.URL)
			}
		}
	}

	fmt.Fprintf(&buf, "}\n")
	return buf.Bytes(), nil
}
