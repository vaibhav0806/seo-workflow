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
	urlInspectionEndpoint = "https://searchconsole.googleapis.com/v1/urlInspection/index:inspect"
	maxSitemapDepth       = 4
	maxSitemapBytes       = 20 << 20
)

var searchAnalyticsEndpoint = "https://searchconsole.googleapis.com/webmasters/v3/sites/%s/searchAnalytics/query"

type GSCAdapter struct {
	httpClient   *http.Client
	accessToken  string
	sitemapURL   string
	lookbackDays int
	rowLimit     int

	mu            sync.RWMutex
	sitemapURLSet map[string]struct{}
	performance   gsc.PerformanceSnapshot
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
		Clicks      float64  `json:"clicks"`
		Impressions float64  `json:"impressions"`
		CTR         float64  `json:"ctr"`
		Position    float64  `json:"position"`
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
	rows, startDate, endDate, err := a.querySearchPerformance(ctx, property)
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	a.performance = gsc.PerformanceSnapshot{
		GeneratedAtUTC: time.Now().UTC().Format(time.RFC3339),
		StartDate:      startDate,
		EndDate:        endDate,
		Rows:           append([]gsc.PerformanceMetric(nil), rows...),
	}
	a.mu.Unlock()
	return aggregatePerformanceURLs(rows), nil
}

func (a *GSCAdapter) querySearchPerformance(ctx context.Context, property string) ([]gsc.PerformanceMetric, string, string, error) {
	today := time.Now().UTC()
	endDate := today.AddDate(0, 0, -2)
	if a.lookbackDays == 1 {
		endDate = today.AddDate(0, 0, -1)
	}
	startDate := endDate.AddDate(0, 0, -(a.lookbackDays - 1))

	startDateValue := startDate.Format("2006-01-02")
	endDateValue := endDate.Format("2006-01-02")
	metrics := make([]gsc.PerformanceMetric, 0)
	for startRow := 0; ; startRow += a.rowLimit {
		payload, err := json.Marshal(map[string]any{
			"startDate":  startDateValue,
			"endDate":    endDateValue,
			"dimensions": []string{"date", "query", "page"},
			"rowLimit":   a.rowLimit,
			"startRow":   startRow,
			"type":       "web",
		})
		if err != nil {
			return nil, "", "", fmt.Errorf("marshal search analytics request: %w", err)
		}

		endpoint := fmt.Sprintf(searchAnalyticsEndpoint, url.PathEscape(property))
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
		if err != nil {
			return nil, "", "", fmt.Errorf("build search analytics request: %w", err)
		}
		request.Header.Set("Authorization", "Bearer "+a.accessToken)
		request.Header.Set("Content-Type", "application/json")

		response, err := a.httpClient.Do(request)
		if err != nil {
			return nil, "", "", fmt.Errorf("execute search analytics request: %w", err)
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
			response.Body.Close()
			return nil, "", "", fmt.Errorf("search analytics api status=%d body=%q", response.StatusCode, strings.TrimSpace(string(body)))
		}

		var parsed searchAnalyticsResponse
		decodeErr := json.NewDecoder(response.Body).Decode(&parsed)
		response.Body.Close()
		if decodeErr != nil {
			return nil, "", "", fmt.Errorf("decode search analytics response: %w", decodeErr)
		}
		for _, row := range parsed.Rows {
			if len(row.Keys) < 3 {
				continue
			}
			metric := gsc.PerformanceMetric{
				Date: strings.TrimSpace(row.Keys[0]), Query: strings.TrimSpace(row.Keys[1]), Page: strings.TrimSpace(row.Keys[2]),
				Clicks: row.Clicks, Impressions: row.Impressions, CTR: row.CTR, Position: row.Position,
			}
			if metric.Query != "" && metric.Page != "" {
				metrics = append(metrics, metric)
			}
		}
		if len(parsed.Rows) < a.rowLimit {
			break
		}
	}
	return metrics, startDateValue, endDateValue, nil
}

func (a *GSCAdapter) PerformanceSnapshot() gsc.PerformanceSnapshot {
	a.mu.RLock()
	defer a.mu.RUnlock()
	snapshot := a.performance
	snapshot.Rows = append([]gsc.PerformanceMetric(nil), snapshot.Rows...)
	return snapshot
}

func aggregatePerformanceURLs(rows []gsc.PerformanceMetric) []gsc.URLMetric {
	impressions := make(map[string]float64)
	for _, row := range rows {
		page := strings.TrimSpace(row.Page)
		if page != "" {
			impressions[page] += row.Impressions
		}
	}
	metrics := make([]gsc.URLMetric, 0, len(impressions))
	for page, total := range impressions {
		metrics = append(metrics, gsc.URLMetric{URL: page, Impressions: int64(total)})
	}
	sort.Slice(metrics, func(i, j int) bool {
		if metrics[i].Impressions == metrics[j].Impressions {
			return metrics[i].URL < metrics[j].URL
		}
		return metrics[i].Impressions > metrics[j].Impressions
	})
	return metrics
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
