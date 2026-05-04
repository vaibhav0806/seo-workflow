package competitor

import (
	"fmt"
	"strings"
	"time"
)

type ReportSummary struct {
	Title              string                        `json:"title"`
	GeneratedAtUTC     string                        `json:"generatedAtUtc"`
	WindowDays         int                           `json:"windowDays"`
	OurRecentURLCount  int                           `json:"ourRecentUrlCount"`
	CompetitorCount    int                           `json:"competitorCount"`
	OpportunityCount   int                           `json:"opportunityCount"`
	SkippedTopicCount  int                           `json:"skippedTopicCount"`
	WarningCount       int                           `json:"warningCount"`
	TopOpportunities   []ReportOpportunity           `json:"topOpportunities"`
	RecommendedContent []ReportContentRecommendation `json:"recommendedContent"`
	SkippedTopics      []ReportSkippedTopic          `json:"skippedTopics"`
	Warnings           []string                      `json:"warnings,omitempty"`
}

type ReportOpportunity struct {
	Priority   int      `json:"priority"`
	Topic      string   `json:"topic"`
	Score      int      `json:"score"`
	Competitor string   `json:"competitor"`
	Theme      string   `json:"theme"`
	Why        string   `json:"why"`
	WhatToDo   string   `json:"whatToDo"`
	Evidence   []string `json:"evidence,omitempty"`
}

type ReportContentRecommendation struct {
	Priority       int        `json:"priority"`
	PageType       string     `json:"pageType"`
	SuggestedSlug  string     `json:"suggestedSlug"`
	SuggestedTitle string     `json:"suggestedTitle"`
	TargetIntent   string     `json:"targetIntent"`
	Pillar         string     `json:"pillar"`
	Draft          *BlogDraft `json:"draft,omitempty"`
}

type ReportSkippedTopic struct {
	Competitor    string   `json:"competitor"`
	Topic         string   `json:"topic"`
	Theme         string   `json:"theme"`
	Reason        string   `json:"reason"`
	PageCount     int      `json:"pageCount"`
	EvidenceCount int      `json:"evidenceCount"`
	EvidenceURLs  []string `json:"evidenceUrls,omitempty"`
}

func BuildReportSummary(summary Summary, limit int) ReportSummary {
	if limit <= 0 {
		limit = 5
	}
	titleTime := summary.GeneratedAtUTC
	if parsed, err := time.Parse(time.RFC3339, summary.GeneratedAtUTC); err == nil {
		titleTime = parsed.UTC().Format("2006-01-02 15:04 UTC")
	}
	report := ReportSummary{
		Title:             fmt.Sprintf("Competitor SEO Report - %s", titleTime),
		GeneratedAtUTC:    summary.GeneratedAtUTC,
		WindowDays:        summary.WindowDays,
		OurRecentURLCount: summary.OurSite.RecentURLCount,
		CompetitorCount:   len(summary.Competitors),
		OpportunityCount:  len(summary.Opportunities),
		SkippedTopicCount: len(summary.Debug.SkippedTopics),
		WarningCount:      len(summary.Warnings),
		Warnings:          limitStrings(summary.Warnings, 5),
	}

	for idx, opportunity := range summary.Opportunities {
		if idx >= limit {
			break
		}
		report.TopOpportunities = append(report.TopOpportunities, ReportOpportunity{
			Priority:   idx + 1,
			Topic:      opportunityTopicName(opportunity.Title),
			Score:      opportunity.ImpactScore,
			Competitor: opportunity.Competitor,
			Theme:      opportunity.Theme,
			Why:        opportunity.WhyItMatters,
			WhatToDo:   opportunity.WhatToDo,
			Evidence:   limitStrings(opportunity.Evidence, 2),
		})
	}

	for idx, recommendation := range summary.ContentPlan {
		if idx >= limit {
			break
		}
		report.RecommendedContent = append(report.RecommendedContent, ReportContentRecommendation{
			Priority:       recommendation.Priority,
			PageType:       recommendation.PageType,
			SuggestedSlug:  recommendation.SuggestedSlug,
			SuggestedTitle: recommendation.SuggestedTitle,
			TargetIntent:   recommendation.TargetIntent,
			Pillar:         recommendation.Pillar,
			Draft:          recommendation.Draft,
		})
	}

	for idx, skipped := range summary.Debug.SkippedTopics {
		if idx >= limit {
			break
		}
		report.SkippedTopics = append(report.SkippedTopics, ReportSkippedTopic{
			Competitor:    skipped.Competitor,
			Topic:         skipped.Topic,
			Theme:         skipped.Theme,
			Reason:        skipped.Reason,
			PageCount:     skipped.PageCount,
			EvidenceCount: skipped.EvidenceCount,
			EvidenceURLs:  limitStrings(skipped.EvidenceURLs, 2),
		})
	}

	return report
}

func (report ReportSummary) PlainText() string {
	var b strings.Builder
	b.WriteString(report.Title)
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("Window: %d days | Competitors: %d | Opportunities: %d | Skipped topics: %d | Warnings: %d\n",
		report.WindowDays,
		report.CompetitorCount,
		report.OpportunityCount,
		report.SkippedTopicCount,
		report.WarningCount,
	))
	for _, opportunity := range report.TopOpportunities {
		b.WriteString(fmt.Sprintf("\n%d. %s (%d, %s/%s)\n", opportunity.Priority, opportunity.Topic, opportunity.Score, opportunity.Competitor, opportunity.Theme))
		b.WriteString(opportunity.WhatToDo)
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}
