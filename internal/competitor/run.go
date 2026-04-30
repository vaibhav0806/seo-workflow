package competitor

import (
	"context"
	"fmt"
	"sort"
	"strings"
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

type Summary struct {
	GeneratedAtUTC  string         `json:"generatedAtUtc"`
	WindowDays      int            `json:"windowDays"`
	WindowStartUTC  string         `json:"windowStartUtc"`
	OurSite         SiteSnapshot   `json:"ourSite"`
	Competitors     []SiteSnapshot `json:"competitors"`
	Opportunities   []Opportunity  `json:"opportunities"`
	Warnings        []string       `json:"warnings"`
	OpenRouterModel string         `json:"openRouterModel,omitempty"`
}

var defaultCompetitors = []CompetitorTarget{
	{Name: "vercel", SitemapURL: "https://vercel.com/sitemap.xml"},
	{Name: "lovable", SitemapURL: "https://lovable.dev/sitemap.xml"},
	{Name: "replit", SitemapURL: "https://replit.com/sitemap.xml"},
}

func Run(ctx context.Context, cfg *config.Config) (Summary, error) {
	if cfg == nil {
		return Summary{}, fmt.Errorf("competitor config is nil")
	}

	windowStart := time.Now().UTC().AddDate(0, 0, -cfg.CompetitorWindowDays)
	fetcher := NewSitemapFetcher(cfg.HTTPTimeoutSecs)
	warnings := make([]string, 0)

	ourEntries, err := fetcher.Fetch(ctx, cfg.OurSitemapURL)
	if err != nil {
		return Summary{}, fmt.Errorf("fetch our sitemap: %w", err)
	}
	ourSnapshot := buildSnapshot("createos", cfg.OurSitemapURL, ourEntries, windowStart)

	competitorSnapshots := make([]SiteSnapshot, 0, len(defaultCompetitors))
	for _, target := range defaultCompetitors {
		entries, fetchErr := fetcher.Fetch(ctx, target.SitemapURL)
		if fetchErr != nil {
			warnings = append(warnings, fmt.Sprintf("%s sitemap fetch failed: %v", target.Name, fetchErr))
			competitorSnapshots = append(competitorSnapshots, SiteSnapshot{
				Name:        target.Name,
				SitemapURL:  target.SitemapURL,
				ThemeCounts: map[string]int{},
				Error:       fetchErr.Error(),
			})
			continue
		}
		competitorSnapshots = append(competitorSnapshots, buildSnapshot(target.Name, target.SitemapURL, entries, windowStart))
	}

	opportunities := deriveOpportunities(ourSnapshot, competitorSnapshots)
	if cfg.OpenRouterAPIKey != "" {
		refined, refineErr := refineWithOpenRouter(ctx, cfg.OpenRouterAPIKey, cfg.OpenRouterModel, ourSnapshot, competitorSnapshots, opportunities)
		if refineErr != nil {
			warnings = append(warnings, fmt.Sprintf("openrouter refinement skipped: %v", refineErr))
		} else {
			opportunities = refined
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
		Opportunities:   opportunities,
		Warnings:        warnings,
		OpenRouterModel: strings.TrimSpace(cfg.OpenRouterModel),
	}, nil
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
