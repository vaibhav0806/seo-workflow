package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/nodeops/seo-workflow/internal/competitor"
	"github.com/nodeops/seo-workflow/internal/config"
	"github.com/nodeops/seo-workflow/internal/scan"
)

type oneshotReport struct {
	GeneratedAtUTC string                 `json:"generatedAtUtc"`
	Property       string                 `json:"property"`
	Repo           string                 `json:"repo"`
	DryRun         bool                   `json:"dryRun"`
	Summary        scan.Summary           `json:"summary"`
	UnknownBuckets []unknownBucketSummary `json:"unknownBuckets"`
}

type unknownBucketSummary struct {
	CoverageState  string   `json:"coverageState"`
	PageFetchState string   `json:"pageFetchState"`
	Count          int      `json:"count"`
	SampleURLs     []string `json:"sampleUrls"`
}

type unknownKey struct {
	coverage string
	fetch    string
}

type unknownAggregate struct {
	count int
	urls  []string
}

func logDetailedSummary(summary scan.Summary) {
	log.Printf("detailed findings: count=%d", len(summary.Findings))
	for _, finding := range summary.Findings {
		log.Printf(
			"finding url=%q bucket=%q coverage_state=%q page_fetch_state=%q in_sitemap=%t",
			finding.URL,
			finding.Bucket,
			finding.CoverageState,
			finding.PageFetchState,
			finding.InSitemap,
		)
	}
}

func buildUnknownSummary(findings []scan.Finding) []unknownBucketSummary {
	agg := map[unknownKey]*unknownAggregate{}
	for _, finding := range findings {
		if finding.Bucket != "unknown" {
			continue
		}
		key := unknownKey{
			coverage: strings.TrimSpace(finding.CoverageState),
			fetch:    strings.TrimSpace(finding.PageFetchState),
		}
		entry, exists := agg[key]
		if !exists {
			entry = &unknownAggregate{count: 0, urls: []string{}}
			agg[key] = entry
		}
		entry.count++
		if len(entry.urls) < 5 {
			entry.urls = append(entry.urls, finding.URL)
		}
	}

	result := make([]unknownBucketSummary, 0, len(agg))
	for key, entry := range agg {
		result = append(result, unknownBucketSummary{
			CoverageState:  key.coverage,
			PageFetchState: key.fetch,
			Count:          entry.count,
			SampleURLs:     entry.urls,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Count == result[j].Count {
			if result[i].CoverageState == result[j].CoverageState {
				return result[i].PageFetchState < result[j].PageFetchState
			}
			return result[i].CoverageState < result[j].CoverageState
		}
		return result[i].Count > result[j].Count
	})

	return result
}

func writeOneshotReport(cfg *config.Config, summary scan.Summary) error {
	path := strings.TrimSpace(cfg.OneshotReportPath)
	if path == "" {
		return nil
	}

	report := oneshotReport{
		GeneratedAtUTC: time.Now().UTC().Format(time.RFC3339),
		Property:       cfg.ScanProperty,
		Repo:           cfg.ScanRepo,
		DryRun:         cfg.DryRun,
		Summary:        summary,
		UnknownBuckets: buildUnknownSummary(summary.Findings),
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal oneshot report: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write oneshot report to %q: %w", path, err)
	}
	log.Printf("oneshot report written: path=%q", path)
	return nil
}

func logCompetitorSummary(summary competitor.Summary) {
	log.Printf(
		"competitor summary: window_days=%d our_recent=%d competitors=%d opportunities=%d warnings=%d",
		summary.WindowDays,
		summary.OurSite.RecentURLCount,
		len(summary.Competitors),
		len(summary.Opportunities),
		len(summary.Warnings),
	)
	for _, competitorSnapshot := range summary.Competitors {
		if competitorSnapshot.Error != "" {
			log.Printf("competitor=%q error=%q", competitorSnapshot.Name, competitorSnapshot.Error)
			continue
		}
		log.Printf(
			"competitor=%q total_urls=%d recent_urls=%d top_themes=%v",
			competitorSnapshot.Name,
			competitorSnapshot.TotalURLs,
			competitorSnapshot.RecentURLCount,
			topThemeCounts(competitorSnapshot.ThemeCounts, 5),
		)
	}
	for idx, topic := range summary.ExtractedTopics {
		log.Printf(
			"topic_%d competitor=%q name=%q pages=%d why=%q evidence=%v",
			idx+1,
			topic.Competitor,
			topic.Name,
			topic.PageCount,
			topic.WhyItMatters,
			topic.EvidenceURLs,
		)
	}

	for idx, opportunity := range summary.Opportunities {
		log.Printf(
			"opportunity_%d title=%q score=%d type=%q competitor=%q theme=%q why=%q",
			idx+1,
			opportunity.Title,
			opportunity.ImpactScore,
			opportunity.OpportunityType,
			opportunity.Competitor,
			opportunity.Theme,
			opportunity.WhyItMatters,
		)
	}

	for _, warning := range summary.Warnings {
		log.Printf("warning=%q", warning)
	}
}

func writeCompetitorReport(cfg *config.Config, summary competitor.Summary) error {
	path := strings.TrimSpace(cfg.CompetitorReportPath)
	if path == "" {
		return nil
	}
	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal competitor report: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write competitor report to %q: %w", path, err)
	}
	log.Printf("competitor report written: path=%q", path)
	return nil
}

func topThemeCounts(themeCounts map[string]int, limit int) []string {
	type item struct {
		name  string
		count int
	}
	items := make([]item, 0, len(themeCounts))
	for name, count := range themeCounts {
		items = append(items, item{name: name, count: count})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].count == items[j].count {
			return items[i].name < items[j].name
		}
		return items[i].count > items[j].count
	})
	if len(items) > limit {
		items = items[:limit]
	}
	result := make([]string, 0, len(items))
	for _, entry := range items {
		result = append(result, fmt.Sprintf("%s:%d", entry.name, entry.count))
	}
	return result
}
