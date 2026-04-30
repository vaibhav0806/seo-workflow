package competitor

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nodeops/seo-workflow/internal/config"
)

type CompetitorTarget struct {
	Name       string `json:"name"`
	SitemapURL string `json:"sitemapUrl"`
}

type SitemapEntry struct {
	URL       string   `json:"url"`
	Title     string   `json:"title,omitempty"`
	LastMod   *string  `json:"lastMod,omitempty"`
	ThemeTags []string `json:"themeTags,omitempty"`
}

type SiteSnapshot struct {
	Name           string         `json:"name"`
	SitemapURL     string         `json:"sitemapUrl"`
	TotalURLs      int            `json:"totalUrls"`
	RecentURLs     []SitemapEntry `json:"recentUrls"`
	RecentURLCount int            `json:"recentUrlCount"`
	ThemeCounts    map[string]int `json:"themeCounts"`
	Error          string         `json:"error,omitempty"`
}

type Opportunity struct {
	Title           string   `json:"title"`
	WhyItMatters    string   `json:"whyItMatters"`
	WhatToDo        string   `json:"whatToDo"`
	HowToExecute    []string `json:"howToExecute"`
	ImpactScore     int      `json:"impactScore"`
	Competitor      string   `json:"competitor"`
	Theme           string   `json:"theme"`
	OpportunityType string   `json:"opportunityType"`
	Evidence        []string `json:"evidence"`
}

type TopicSummary struct {
	Competitor           string   `json:"competitor"`
	Name                 string   `json:"name"`
	PageCount            int      `json:"pageCount"`
	RepresentativeTitles []string `json:"representativeTitles"`
	EvidenceURLs         []string `json:"evidenceUrls"`
	WhyItMatters         string   `json:"whyItMatters"`
}

type Summary struct {
	GeneratedAtUTC  string         `json:"generatedAtUtc"`
	WindowDays      int            `json:"windowDays"`
	WindowStartUTC  string         `json:"windowStartUtc"`
	OurSite         SiteSnapshot   `json:"ourSite"`
	Competitors     []SiteSnapshot `json:"competitors"`
	ExtractedTopics []TopicSummary `json:"extractedTopics,omitempty"`
	Opportunities   []Opportunity  `json:"opportunities"`
	Warnings        []string       `json:"warnings"`
	OpenRouterModel string         `json:"openRouterModel,omitempty"`
}

var defaultCompetitors = []CompetitorTarget{
	{Name: "vercel", SitemapURL: "https://vercel.com/sitemap.xml"},
	{Name: "lovable", SitemapURL: "https://lovable.dev/sitemap.xml"},
	{Name: "replit", SitemapURL: "https://replit.com/sitemap.xml"},
}

const (
	titleEnrichmentLimit       = 40
	titleEnrichmentConcurrency = 8
	titleEnrichmentTimeout     = 20 * time.Second
)

func Run(ctx context.Context, cfg *config.Config) (Summary, error) {
	if cfg == nil {
		return Summary{}, fmt.Errorf("competitor config is nil")
	}

	windowStart := time.Now().UTC().AddDate(0, 0, -cfg.CompetitorWindowDays)
	fetcher := NewSitemapFetcher(cfg.HTTPTimeoutSecs)
	titleFetcher := NewTitleFetcher(cfg.HTTPTimeoutSecs)
	warnings := make([]string, 0)

	ourEntries, err := fetcher.Fetch(ctx, cfg.OurSitemapURL)
	if err != nil {
		return Summary{}, fmt.Errorf("fetch our sitemap: %w", err)
	}
	ourSnapshot := buildSnapshot("createos", cfg.OurSitemapURL, ourEntries, windowStart)
	ourSnapshot, titleWarnings, err := enrichSnapshotTitles(ctx, titleFetcher, ourSnapshot, titleEnrichmentLimit)
	if err != nil {
		return Summary{}, fmt.Errorf("title enrichment failed for createos: %w", err)
	}
	warnings = append(warnings, titleWarnings...)

	competitorSnapshots := make([]SiteSnapshot, 0, len(defaultCompetitors))
	for _, target := range defaultCompetitors {
		entries, fetchErr := fetcher.Fetch(ctx, target.SitemapURL)
		if fetchErr != nil {
			if ctx.Err() != nil {
				return Summary{}, fmt.Errorf("fetch competitor sitemap %q: %w", target.Name, ctx.Err())
			}
			warnings = append(warnings, fmt.Sprintf("%s sitemap fetch failed: %v", target.Name, fetchErr))
			competitorSnapshots = append(competitorSnapshots, SiteSnapshot{
				Name:        target.Name,
				SitemapURL:  target.SitemapURL,
				ThemeCounts: map[string]int{},
				Error:       fetchErr.Error(),
			})
			continue
		}
		snapshot := buildSnapshot(target.Name, target.SitemapURL, entries, windowStart)
		snapshot, titleWarnings, err = enrichSnapshotTitles(ctx, titleFetcher, snapshot, titleEnrichmentLimit)
		if err != nil {
			return Summary{}, fmt.Errorf("title enrichment failed for %s: %w", target.Name, err)
		}
		warnings = append(warnings, titleWarnings...)
		competitorSnapshots = append(competitorSnapshots, snapshot)
	}

	opportunities := deriveOpportunities(ourSnapshot, competitorSnapshots)
	extractedTopics := []TopicSummary{}
	if cfg.OpenRouterAPIKey != "" {
		topics, topicErr := extractTopicsWithOpenRouter(ctx, cfg.OpenRouterAPIKey, cfg.OpenRouterModel, competitorSnapshots)
		if topicErr != nil {
			warnings = append(warnings, fmt.Sprintf("openrouter topic extraction skipped: %v", topicErr))
		} else {
			extractedTopics = topics
			topicOpportunities := deriveTopicOpportunities(ourSnapshot, topics)
			if len(topicOpportunities) > 0 {
				opportunities = topicOpportunities
			}
		}
	}

	sort.Slice(opportunities, func(i, j int) bool {
		if opportunities[i].ImpactScore == opportunities[j].ImpactScore {
			return strings.ToLower(opportunities[i].Title) < strings.ToLower(opportunities[j].Title)
		}
		return opportunities[i].ImpactScore > opportunities[j].ImpactScore
	})

	return Summary{
		GeneratedAtUTC:  time.Now().UTC().Format(time.RFC3339),
		WindowDays:      cfg.CompetitorWindowDays,
		WindowStartUTC:  windowStart.Format(time.RFC3339),
		OurSite:         ourSnapshot,
		Competitors:     competitorSnapshots,
		ExtractedTopics: extractedTopics,
		Opportunities:   opportunities,
		Warnings:        warnings,
		OpenRouterModel: strings.TrimSpace(cfg.OpenRouterModel),
	}, nil
}

func enrichSnapshotTitles(ctx context.Context, fetcher *TitleFetcher, snapshot SiteSnapshot, limit int) (SiteSnapshot, []string, error) {
	if fetcher == nil || limit <= 0 {
		return snapshot, nil, nil
	}

	max := limit
	if len(snapshot.RecentURLs) < max {
		max = len(snapshot.RecentURLs)
	}
	if max == 0 {
		return snapshot, nil, nil
	}

	enrichCtx, cancel := context.WithTimeout(ctx, titleEnrichmentTimeout)
	defer cancel()

	workerCount := titleEnrichmentConcurrency
	if max < workerCount {
		workerCount = max
	}

	jobs := make(chan int, max)
	for idx := 0; idx < max; idx++ {
		jobs <- idx
	}
	close(jobs)

	warningsByIndex := make([]string, max)
	var firstParentErr error
	var mu sync.Mutex
	var wg sync.WaitGroup

	for worker := 0; worker < workerCount; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				if enrichCtx.Err() != nil {
					if ctx.Err() != nil {
						recordTitleEnrichmentError(&mu, &firstParentErr, ctx.Err())
					}
					continue
				}

				url := snapshot.RecentURLs[idx].URL
				title, err := fetcher.FetchTitle(enrichCtx, url)
				if err != nil {
					if isContextError(err) {
						if ctx.Err() != nil {
							recordTitleEnrichmentError(&mu, &firstParentErr, ctx.Err())
						}
					} else {
						warningsByIndex[idx] = fmt.Sprintf("title fetch failed for %s: %v", url, err)
					}
					continue
				}
				snapshot.RecentURLs[idx].Title = title
			}
		}()
	}
	wg.Wait()

	mu.Lock()
	err := firstParentErr
	mu.Unlock()
	if err != nil {
		return snapshot, nil, err
	}
	if ctx.Err() != nil {
		return snapshot, nil, ctx.Err()
	}

	warnings := make([]string, 0)
	if errors.Is(enrichCtx.Err(), context.DeadlineExceeded) {
		warnings = append(warnings, fmt.Sprintf("title enrichment timed out for %s; using partial titles", snapshot.Name))
	}
	for _, warning := range warningsByIndex {
		if warning == "" {
			continue
		}
		warnings = append(warnings, warning)
	}
	return snapshot, warnings, nil
}

func recordTitleEnrichmentError(mu *sync.Mutex, firstErr *error, err error) {
	if err == nil {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	if *firstErr == nil {
		*firstErr = err
	}
}

func isContextError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func buildSnapshot(name string, sitemapURL string, entries []rawSitemapEntry, windowStart time.Time) SiteSnapshot {
	recent := make([]SitemapEntry, 0)
	themeCounts := map[string]int{}

	for _, entry := range entries {
		if strings.TrimSpace(entry.URL) == "" {
			continue
		}
		if isJunkPath(name, entry.URL) {
			continue
		}
		themes := classifyThemes(entry.URL)
		effectiveLastMod := entry.LastMod
		if effectiveLastMod == nil {
			effectiveLastMod = inferDateFromURL(entry.URL)
		}
		if effectiveLastMod == nil {
			continue
		}
		if effectiveLastMod.Before(windowStart) {
			continue
		}
		for _, theme := range themes {
			themeCounts[theme]++
		}
		recentEntry := SitemapEntry{URL: entry.URL, ThemeTags: themes}
		v := effectiveLastMod.UTC().Format(time.RFC3339)
		recentEntry.LastMod = &v
		recent = append(recent, recentEntry)
	}

	sort.Slice(recent, func(i, j int) bool {
		left := recent[i].URL
		right := recent[j].URL
		if recent[i].LastMod != nil && recent[j].LastMod != nil && *recent[i].LastMod != *recent[j].LastMod {
			return *recent[i].LastMod > *recent[j].LastMod
		}
		if recent[i].LastMod != nil && recent[j].LastMod == nil {
			return true
		}
		if recent[i].LastMod == nil && recent[j].LastMod != nil {
			return false
		}
		return left < right
	})

	if len(recent) > 200 {
		recent = recent[:200]
	}

	return SiteSnapshot{
		Name:           name,
		SitemapURL:     sitemapURL,
		TotalURLs:      len(entries),
		RecentURLs:     recent,
		RecentURLCount: len(recent),
		ThemeCounts:    themeCounts,
	}
}
