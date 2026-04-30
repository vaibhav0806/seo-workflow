package gsc

import "sort"

// URLMetric stores impressions observed for a URL.
type URLMetric struct {
	URL         string
	Impressions int64
}

// MergeDiscoveries combines sitemap and analytics URLs into a deduplicated,
// impression-prioritized list.
func MergeDiscoveries(sitemapURLs []string, analytics []URLMetric) []string {
	impressionsByURL := make(map[string]int64, len(sitemapURLs)+len(analytics))

	for _, url := range sitemapURLs {
		if url == "" {
			continue
		}
		if _, exists := impressionsByURL[url]; !exists {
			impressionsByURL[url] = 0
		}
	}

	for _, metric := range analytics {
		if metric.URL == "" {
			continue
		}
		current, exists := impressionsByURL[metric.URL]
		if !exists || metric.Impressions > current {
			impressionsByURL[metric.URL] = metric.Impressions
		}
	}

	merged := make([]string, 0, len(impressionsByURL))
	for url := range impressionsByURL {
		merged = append(merged, url)
	}

	sort.Slice(merged, func(i, j int) bool {
		leftURL, rightURL := merged[i], merged[j]
		leftImp, rightImp := impressionsByURL[leftURL], impressionsByURL[rightURL]
		if leftImp == rightImp {
			return leftURL < rightURL
		}
		return leftImp > rightImp
	})

	return merged
}
