package oneshot

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nodeops/seo-workflow/internal/classifier"
	"github.com/nodeops/seo-workflow/internal/gsc"
)

const (
	searchAnalyticsEndpoint = "https://searchconsole.googleapis.com/webmasters/v3/sites/%s/searchAnalytics/query"
	urlInspectionEndpoint   = "https://searchconsole.googleapis.com/v1/urlInspection/index:inspect"
	maxSitemapDepth         = 4
	maxSitemapBytes         = 20 << 20
)

type GSCAdapter struct {
	httpClient   *http.Client
	accessToken  string
	sitemapURL   string
	lookbackDays int
	rowLimit     int

	mu            sync.RWMutex
	sitemapURLSet map[string]struct{}
}

type urlSetXML struct {
	URLs []struct {
		Loc string `xml:"loc"`
	} `xml:"url"`
}

type sitemapIndexXML struct {
	Sitemaps []struct {
		Loc string `xml:"loc"`
	} `xml:"sitemap"`
}

type searchAnalyticsResponse struct {
	Rows []struct {
		Keys        []string `json:"keys"`
		Impressions float64  `json:"impressions"`
	} `json:"rows"`
}

type inspectResponse struct {
	InspectionResult struct {
		IndexStatusResult struct {
			CoverageState  string `json:"coverageState"`
			PageFetchState string `json:"pageFetchState"`
		} `json:"indexStatusResult"`
	} `json:"inspectionResult"`
}

func NewGSCAdapter(accessToken string, sitemapURL string, lookbackDays int, rowLimit int, timeoutSecs int) *GSCAdapter {
	return &GSCAdapter{
		httpClient:    &http.Client{Timeout: time.Duration(timeoutSecs) * time.Second},
		accessToken:   accessToken,
		sitemapURL:    sitemapURL,
		lookbackDays:  lookbackDays,
		rowLimit:      rowLimit,
		sitemapURLSet: map[string]struct{}{},
	}
}

func (a *GSCAdapter) Discover(ctx context.Context, property string) ([]string, []gsc.URLMetric, error) {
	sitemapURLs, err := a.fetchSitemapURLs(ctx, a.sitemapURL, 0, map[string]struct{}{})
	if err != nil {
		return nil, nil, fmt.Errorf("fetch sitemap urls: %w", err)
	}

	sitemapSet := make(map[string]struct{}, len(sitemapURLs))
	for _, pageURL := range sitemapURLs {
		sitemapSet[pageURL] = struct{}{}
	}

	a.mu.Lock()
	a.sitemapURLSet = sitemapSet
	a.mu.Unlock()

	analyticsURLs, err := a.querySearchAnalytics(ctx, property)
	if err != nil {
		return nil, nil, fmt.Errorf("query search analytics: %w", err)
	}

	return sitemapURLs, analyticsURLs, nil
}

func (a *GSCAdapter) InspectURL(ctx context.Context, property string, pageURL string) (classifier.InspectionSignal, error) {
	requestBody, err := json.Marshal(map[string]string{
		"inspectionUrl": pageURL,
		"siteUrl":       property,
		"languageCode":  "en-US",
	})
	if err != nil {
		return classifier.InspectionSignal{}, fmt.Errorf("marshal url inspection request: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, urlInspectionEndpoint, bytes.NewReader(requestBody))
	if err != nil {
		return classifier.InspectionSignal{}, fmt.Errorf("build url inspection request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+a.accessToken)
	request.Header.Set("Content-Type", "application/json")

	response, err := a.httpClient.Do(request)
	if err != nil {
		return classifier.InspectionSignal{}, fmt.Errorf("execute url inspection request: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return classifier.InspectionSignal{}, fmt.Errorf("url inspection api status=%d body=%q", response.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed inspectResponse
	if err := json.NewDecoder(response.Body).Decode(&parsed); err != nil {
		return classifier.InspectionSignal{}, fmt.Errorf("decode url inspection response: %w", err)
	}

	a.mu.RLock()
	_, inSitemap := a.sitemapURLSet[strings.TrimSpace(pageURL)]
	a.mu.RUnlock()

	return classifier.InspectionSignal{
		CoverageState:  strings.TrimSpace(parsed.InspectionResult.IndexStatusResult.CoverageState),
		PageFetchState: strings.TrimSpace(parsed.InspectionResult.IndexStatusResult.PageFetchState),
		InSitemap:      inSitemap,
	}, nil
}

func (a *GSCAdapter) Load(ctx context.Context, _ string) (string, error) {
	content, err := a.fetchRawSitemap(ctx, a.sitemapURL)
	if err != nil {
		return "", err
	}
	return content, nil
}

func (a *GSCAdapter) querySearchAnalytics(ctx context.Context, property string) ([]gsc.URLMetric, error) {
	today := time.Now().UTC()
	endDate := today.AddDate(0, 0, -2)
	if a.lookbackDays == 1 {
		endDate = today.AddDate(0, 0, -1)
	}
	startDate := endDate.AddDate(0, 0, -(a.lookbackDays - 1))

	payload, err := json.Marshal(map[string]any{
		"startDate":  startDate.Format("2006-01-02"),
		"endDate":    endDate.Format("2006-01-02"),
		"dimensions": []string{"page"},
		"rowLimit":   a.rowLimit,
		"type":       "web",
	})
	if err != nil {
		return nil, fmt.Errorf("marshal search analytics request: %w", err)
	}

	endpoint := fmt.Sprintf(searchAnalyticsEndpoint, url.PathEscape(property))
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build search analytics request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+a.accessToken)
	request.Header.Set("Content-Type", "application/json")

	response, err := a.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("execute search analytics request: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return nil, fmt.Errorf("search analytics api status=%d body=%q", response.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed searchAnalyticsResponse
	if err := json.NewDecoder(response.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decode search analytics response: %w", err)
	}

	metrics := make([]gsc.URLMetric, 0, len(parsed.Rows))
	for _, row := range parsed.Rows {
		if len(row.Keys) == 0 {
			continue
		}
		pageURL := strings.TrimSpace(row.Keys[0])
		if pageURL == "" {
			continue
		}
		metrics = append(metrics, gsc.URLMetric{URL: pageURL, Impressions: int64(row.Impressions)})
	}
	return metrics, nil
}

func (a *GSCAdapter) fetchSitemapURLs(
	ctx context.Context,
	sourceURL string,
	depth int,
	visited map[string]struct{},
) ([]string, error) {
	if depth > maxSitemapDepth {
		return nil, fmt.Errorf("sitemap recursion exceeded max depth for %q", sourceURL)
	}
	sourceURL = strings.TrimSpace(sourceURL)
	if sourceURL == "" {
		return nil, errors.New("sitemap url is empty")
	}
	if _, seen := visited[sourceURL]; seen {
		return nil, nil
	}
	visited[sourceURL] = struct{}{}

	raw, err := a.fetchRawSitemap(ctx, sourceURL)
	if err != nil {
		return nil, err
	}
	if looksLikeHTML(raw) {
		return nil, fmt.Errorf("sitemap endpoint returned html, not xml: %q", sourceURL)
	}

	var urlset urlSetXML
	if err := xml.Unmarshal([]byte(raw), &urlset); err == nil && len(urlset.URLs) > 0 {
		urls := make([]string, 0, len(urlset.URLs))
		seen := make(map[string]struct{}, len(urlset.URLs))
		for _, entry := range urlset.URLs {
			loc := strings.TrimSpace(entry.Loc)
			if loc == "" {
				continue
			}
			if _, exists := seen[loc]; exists {
				continue
			}
			seen[loc] = struct{}{}
			urls = append(urls, loc)
		}
		return urls, nil
	}

	var sitemapIndex sitemapIndexXML
	if err := xml.Unmarshal([]byte(raw), &sitemapIndex); err == nil && len(sitemapIndex.Sitemaps) > 0 {
		allURLs := make([]string, 0)
		seen := make(map[string]struct{})
		for _, sitemap := range sitemapIndex.Sitemaps {
			nestedURL := strings.TrimSpace(sitemap.Loc)
			if nestedURL == "" {
				continue
			}
			nestedURLs, nestedErr := a.fetchSitemapURLs(ctx, nestedURL, depth+1, visited)
			if nestedErr != nil {
				return nil, nestedErr
			}
			for _, pageURL := range nestedURLs {
				if _, exists := seen[pageURL]; exists {
					continue
				}
				seen[pageURL] = struct{}{}
				allURLs = append(allURLs, pageURL)
			}
		}
		sort.Strings(allURLs)
		return allURLs, nil
	}

	root, locs, parseErr := parseRootAndLocs(raw)
	if parseErr == nil && len(locs) > 0 {
		if strings.EqualFold(root, "sitemapindex") || likelySitemapIndex(locs) {
			allURLs := make([]string, 0)
			seen := make(map[string]struct{})
			for _, nestedURL := range locs {
				nestedURLs, nestedErr := a.fetchSitemapURLs(ctx, nestedURL, depth+1, visited)
				if nestedErr != nil {
					return nil, nestedErr
				}
				for _, pageURL := range nestedURLs {
					if _, exists := seen[pageURL]; exists {
						continue
					}
					seen[pageURL] = struct{}{}
					allURLs = append(allURLs, pageURL)
				}
			}
			sort.Strings(allURLs)
			return allURLs, nil
		}

		unique := make([]string, 0, len(locs))
		seen := make(map[string]struct{}, len(locs))
		for _, loc := range locs {
			if _, exists := seen[loc]; exists {
				continue
			}
			seen[loc] = struct{}{}
			unique = append(unique, loc)
		}
		return unique, nil
	}

	return nil, fmt.Errorf("unsupported sitemap xml format for %q", sourceURL)
}

func (a *GSCAdapter) fetchRawSitemap(ctx context.Context, sitemapURL string) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, sitemapURL, nil)
	if err != nil {
		return "", fmt.Errorf("build sitemap request: %w", err)
	}

	response, err := a.httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("fetch sitemap: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return "", fmt.Errorf("fetch sitemap status=%d body=%q", response.StatusCode, strings.TrimSpace(string(body)))
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxSitemapBytes))
	if err != nil {
		return "", fmt.Errorf("read sitemap body: %w", err)
	}
	return string(body), nil
}

func parseRootAndLocs(raw string) (string, []string, error) {
	decoder := xml.NewDecoder(strings.NewReader(raw))
	root := ""
	locs := make([]string, 0)
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", nil, err
		}

		switch element := token.(type) {
		case xml.StartElement:
			if root == "" {
				root = strings.TrimSpace(element.Name.Local)
			}
			if strings.EqualFold(element.Name.Local, "loc") {
				var value string
				if err := decoder.DecodeElement(&value, &element); err != nil {
					return "", nil, err
				}
				value = strings.TrimSpace(value)
				if value != "" {
					locs = append(locs, value)
				}
			}
		}
	}
	return root, locs, nil
}

func likelySitemapIndex(locs []string) bool {
	if len(locs) == 0 {
		return false
	}
	matches := 0
	for _, loc := range locs {
		l := strings.ToLower(strings.TrimSpace(loc))
		if strings.HasSuffix(l, ".xml") || strings.HasSuffix(l, ".xml.gz") || strings.Contains(l, "sitemap") {
			matches++
		}
	}
	return matches == len(locs)
}

func looksLikeHTML(body string) bool {
	trimmed := strings.TrimSpace(strings.ToLower(body))
	return strings.HasPrefix(trimmed, "<!doctype html") || strings.HasPrefix(trimmed, "<html")
}
