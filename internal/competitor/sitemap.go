package competitor

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

const (
	maxSitemapBytes = 20 << 20
	maxSitemapDepth = 4
)

type rawSitemapEntry struct {
	URL     string
	LastMod *time.Time
}

type sitemapFetcher struct {
	httpClient *http.Client
}

func NewSitemapFetcher(timeoutSecs int) *sitemapFetcher {
	return &sitemapFetcher{httpClient: &http.Client{Timeout: time.Duration(timeoutSecs) * time.Second}}
}

func (f *sitemapFetcher) Fetch(ctx context.Context, sitemapURL string) ([]rawSitemapEntry, error) {
	return f.fetchRecursive(ctx, strings.TrimSpace(sitemapURL), 0, map[string]struct{}{})
}

func (f *sitemapFetcher) fetchRecursive(ctx context.Context, sitemapURL string, depth int, visited map[string]struct{}) ([]rawSitemapEntry, error) {
	if depth > maxSitemapDepth {
		return nil, fmt.Errorf("sitemap recursion exceeded max depth for %q", sitemapURL)
	}
	if sitemapURL == "" {
		return nil, fmt.Errorf("sitemap url is empty")
	}
	if _, exists := visited[sitemapURL]; exists {
		return nil, nil
	}
	visited[sitemapURL] = struct{}{}

	raw, err := f.fetchRaw(ctx, sitemapURL)
	if err != nil {
		return nil, err
	}
	if looksLikeHTML(raw) {
		return nil, fmt.Errorf("sitemap endpoint returned html, not xml: %q", sitemapURL)
	}

	root, urls, nested, parseErr := parseSitemapXML(raw)
	if parseErr != nil {
		return nil, fmt.Errorf("parse sitemap xml %q: %w", sitemapURL, parseErr)
	}

	if strings.EqualFold(root, "sitemapindex") || likelySitemapIndexURLs(nested) {
		aggregate := make([]rawSitemapEntry, 0)
		seen := map[string]struct{}{}
		for _, next := range nested {
			children, childErr := f.fetchRecursive(ctx, next, depth+1, visited)
			if childErr != nil {
				return nil, childErr
			}
			for _, child := range children {
				if _, exists := seen[child.URL]; exists {
					continue
				}
				seen[child.URL] = struct{}{}
				aggregate = append(aggregate, child)
			}
		}
		sort.Slice(aggregate, func(i, j int) bool { return aggregate[i].URL < aggregate[j].URL })
		return aggregate, nil
	}

	if len(urls) == 0 {
		return nil, fmt.Errorf("unsupported sitemap xml format for %q", sitemapURL)
	}

	seen := map[string]struct{}{}
	unique := make([]rawSitemapEntry, 0, len(urls))
	for _, entry := range urls {
		if _, exists := seen[entry.URL]; exists {
			continue
		}
		seen[entry.URL] = struct{}{}
		unique = append(unique, entry)
	}
	sort.Slice(unique, func(i, j int) bool { return unique[i].URL < unique[j].URL })
	return unique, nil
}

func (f *sitemapFetcher) fetchRaw(ctx context.Context, sitemapURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sitemapURL, nil)
	if err != nil {
		return "", fmt.Errorf("build sitemap request: %w", err)
	}
	resp, err := f.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch sitemap: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("fetch sitemap status=%d body=%q", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxSitemapBytes))
	if err != nil {
		return "", fmt.Errorf("read sitemap body: %w", err)
	}
	return string(body), nil
}

func parseSitemapXML(raw string) (string, []rawSitemapEntry, []string, error) {
	decoder := xml.NewDecoder(strings.NewReader(raw))
	root := ""
	urls := make([]rawSitemapEntry, 0)
	nested := make([]string, 0)

	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", nil, nil, err
		}

		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if root == "" {
			root = strings.TrimSpace(start.Name.Local)
		}

		switch strings.ToLower(strings.TrimSpace(start.Name.Local)) {
		case "url":
			var node struct {
				Loc     string `xml:"loc"`
				LastMod string `xml:"lastmod"`
			}
			if err := decoder.DecodeElement(&node, &start); err != nil {
				return "", nil, nil, err
			}
			loc := strings.TrimSpace(node.Loc)
			if loc == "" {
				continue
			}
			urls = append(urls, rawSitemapEntry{URL: loc, LastMod: parseLastMod(node.LastMod)})
		case "sitemap":
			var node struct {
				Loc string `xml:"loc"`
			}
			if err := decoder.DecodeElement(&node, &start); err != nil {
				return "", nil, nil, err
			}
			loc := strings.TrimSpace(node.Loc)
			if loc != "" {
				nested = append(nested, loc)
			}
		}
	}

	return root, urls, nested, nil
}

func parseLastMod(value string) *time.Time {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return nil
	}
	formats := []string{
		time.RFC3339,
		"2006-01-02",
		"2006-01-02T15:04:05-0700",
		"2006-01-02T15:04:05Z0700",
	}
	for _, format := range formats {
		parsed, err := time.Parse(format, raw)
		if err == nil {
			t := parsed.UTC()
			return &t
		}
	}
	return nil
}

func looksLikeHTML(raw string) bool {
	trimmed := strings.TrimSpace(strings.ToLower(raw))
	return strings.HasPrefix(trimmed, "<!doctype html") || strings.HasPrefix(trimmed, "<html")
}

func likelySitemapIndexURLs(urls []string) bool {
	if len(urls) == 0 {
		return false
	}
	for _, loc := range urls {
		l := strings.ToLower(strings.TrimSpace(loc))
		if !(strings.HasSuffix(l, ".xml") || strings.HasSuffix(l, ".xml.gz") || strings.Contains(l, "sitemap")) {
			return false
		}
	}
	return true
}
