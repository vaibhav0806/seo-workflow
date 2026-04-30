package competitor

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFetcherHandlesSitemapIndexAndLastMod(t *testing.T) {
	fetcher := &sitemapFetcher{
		httpClient: &http.Client{Transport: stubTransport{
			responses: map[string]string{
				"https://example.com/sitemap.xml": `<?xml version="1.0" encoding="UTF-8"?>
<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <sitemap><loc>https://example.com/blog.xml</loc></sitemap>
  <sitemap><loc>https://example.com/docs.xml</loc></sitemap>
</sitemapindex>`,
				"https://example.com/blog.xml": `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>https://createos.sh/blog/ai-agents</loc><lastmod>2026-04-28</lastmod></url>
</urlset>`,
				"https://example.com/docs.xml": `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>https://createos.sh/docs/mcp</loc><lastmod>2026-04-21T18:31:25Z</lastmod></url>
</urlset>`,
			},
		}},
	}
	entries, err := fetcher.Fetch(context.Background(), "https://example.com/sitemap.xml")
	require.NoError(t, err)
	require.Len(t, entries, 2)
	require.NotNil(t, entries[0].LastMod)
	require.NotNil(t, entries[1].LastMod)
}

func TestFetcherRejectsHTML(t *testing.T) {
	fetcher := &sitemapFetcher{
		httpClient: &http.Client{Transport: stubTransport{
			responses: map[string]string{
				"https://example.com/sitemap.xml": "<!doctype html><html><body>blocked</body></html>",
			},
		}},
	}
	_, err := fetcher.Fetch(context.Background(), "https://example.com/sitemap.xml")
	require.Error(t, err)
	require.Contains(t, err.Error(), "returned html")
}

type stubTransport struct {
	responses map[string]string
}

func (t stubTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	body, ok := t.responses[req.URL.String()]
	if !ok {
		return nil, fmt.Errorf("unexpected request url: %s", req.URL.String())
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}, nil
}
