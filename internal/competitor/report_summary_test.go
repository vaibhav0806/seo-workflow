package competitor

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildReportSummaryKeepsOnlyReviewableFields(t *testing.T) {
	summary := Summary{
		GeneratedAtUTC: "2026-05-04T22:15:00Z",
		WindowDays:     30,
		OurSite:        SiteSnapshot{RecentURLCount: 23},
		Competitors: []SiteSnapshot{
			{Name: "vercel", RecentURLCount: 200},
			{Name: "lovable", RecentURLCount: 200},
		},
		Opportunities: []Opportunity{
			{Title: "CreateOS should cover \"Rapid Prototyping & MVP\"", ImpactScore: 86, Competitor: "replit", Theme: "vibecoding", WhyItMatters: "Speed-to-market demand.", WhatToDo: "Create a use-case page.", Evidence: []string{"https://replit.com/usecases/rapid-prototyping"}},
			{Title: "CreateOS should cover \"SaaS Alternatives\"", ImpactScore: 84, Competitor: "lovable", Theme: "comparison", WhyItMatters: "High-intent alternatives demand.", WhatToDo: "Create a comparison page.", Evidence: []string{"https://lovable.dev/guides/alternatives"}},
			{Title: "CreateOS should cover \"Workflow Automation\"", ImpactScore: 77, Competitor: "lovable", Theme: "general", WhyItMatters: "Business workflow demand.", WhatToDo: "Create a workflow guide."},
		},
		ContentPlan: []ContentRecommendation{
			{Priority: 1, Opportunity: "CreateOS should cover \"Rapid Prototyping & MVP\"", PageType: "use-case landing page", SuggestedSlug: "/use-cases/rapid-prototyping-mvp", SuggestedTitle: "Rapid Prototyping & MVP with CreateOS", TargetIntent: "persona/use-case evaluation", Pillar: "AI app-building use cases", Draft: &BlogDraft{Route: "/use-cases/rapid-prototyping-mvp", Title: "Rapid Prototyping & MVP with CreateOS", BodyMarkdown: "# Draft"}},
			{Priority: 2, Opportunity: "CreateOS should cover \"SaaS Alternatives\"", PageType: "comparison page", SuggestedSlug: "/compare/saas-alternatives", SuggestedTitle: "SaaS Alternatives: CreateOS Comparison Guide", TargetIntent: "commercial evaluation", Pillar: "AI builder comparisons"},
		},
		Warnings: []string{"example warning"},
		Debug: DebugSummary{
			SkippedTopics: []SkippedTopicDebug{
				{Competitor: "vercel", Topic: "AI Agent Development", Theme: "ai", Reason: "covered-by-createos", PageCount: 2, EvidenceCount: 2},
			},
		},
	}

	report := BuildReportSummary(summary, 2)

	require.Equal(t, "Competitor SEO Report - 2026-05-04 22:15 UTC", report.Title)
	require.Equal(t, 30, report.WindowDays)
	require.Equal(t, 2, report.CompetitorCount)
	require.Len(t, report.TopOpportunities, 2)
	require.Equal(t, "Rapid Prototyping & MVP", report.TopOpportunities[0].Topic)
	require.Equal(t, "/use-cases/rapid-prototyping-mvp", report.RecommendedContent[0].SuggestedSlug)
	require.NotNil(t, report.RecommendedContent[0].Draft)
	require.Equal(t, "# Draft", report.RecommendedContent[0].Draft.BodyMarkdown)
	require.Len(t, report.SkippedTopics, 1)
	require.Equal(t, "covered-by-createos", report.SkippedTopics[0].Reason)
}
