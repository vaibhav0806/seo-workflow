package competitor

import (
	"testing"

	"github.com/nodeops/seo-workflow/internal/config"
	"github.com/stretchr/testify/require"
)

func TestManualContentRecommendationFromConfigUsesTitleAndDefaults(t *testing.T) {
	recommendation, ok := manualContentRecommendationFromConfig(&config.Config{
		ManualContentTitle: "Top AI Agent Platforms for Enterprise Teams",
	})

	require.True(t, ok)
	require.Equal(t, 1, recommendation.Priority)
	require.Equal(t, "Manual content request: Top AI Agent Platforms for Enterprise Teams", recommendation.Opportunity)
	require.Equal(t, "manual", recommendation.Competitor)
	require.Equal(t, "enterprise", recommendation.Theme)
	require.Equal(t, "solution landing page", recommendation.PageType)
	require.Equal(t, "/blogs/ai-agent-platforms-for-enterprise-teams", recommendation.SuggestedSlug)
	require.Equal(t, "Top AI Agent Platforms for Enterprise Teams", recommendation.SuggestedTitle)
	require.Equal(t, "enterprise trust evaluation", recommendation.TargetIntent)
	require.Equal(t, "Enterprise AI development", recommendation.Pillar)
	require.NotEmpty(t, recommendation.ContentAngle)
	require.Len(t, recommendation.ClusterPages, 3)
}

func TestManualContentRecommendationFromConfigUsesOverrides(t *testing.T) {
	recommendation, ok := manualContentRecommendationFromConfig(&config.Config{
		ManualContentTitle:        "Top AI Agent Platforms for Enterprise Teams",
		ManualContentTheme:        "comparison",
		ManualContentSlug:         "blogs/top-ai-agent-platforms",
		ManualContentAngle:        "Compare enterprise agent platforms through CreateOS deployment control.",
		ManualContentCompetitor:   "lyzr",
		ManualContentEvidenceURLs: []string{"https://www.lyzr.ai/blog/agents", "https://www.stack-ai.com/blog/platforms"},
	})

	require.True(t, ok)
	require.Equal(t, "lyzr", recommendation.Competitor)
	require.Equal(t, "comparison", recommendation.Theme)
	require.Equal(t, "comparison page", recommendation.PageType)
	require.Equal(t, "/blogs/top-ai-agent-platforms", recommendation.SuggestedSlug)
	require.Equal(t, "Compare enterprise agent platforms through CreateOS deployment control.", recommendation.ContentAngle)
	require.Equal(t, []string{"https://www.lyzr.ai/blog/agents", "https://www.stack-ai.com/blog/platforms"}, recommendation.SourceEvidence)
}

func TestManualContentRecommendationFromConfigSkipsEmptyTitle(t *testing.T) {
	recommendation, ok := manualContentRecommendationFromConfig(&config.Config{})

	require.False(t, ok)
	require.Empty(t, recommendation)
}

func TestManualContentSummaryBuildsOnlyTheRequestedRecommendation(t *testing.T) {
	summary, err := ManualContentSummary(&config.Config{
		ManualContentTitle: "MCP Server Security",
		ManualContentTheme: "security",
	})

	require.NoError(t, err)
	require.Len(t, summary.ContentPlan, 1)
	require.Equal(t, "MCP Server Security", summary.ContentPlan[0].SuggestedTitle)
	require.Empty(t, summary.Competitors)
}

func TestPrependContentRecommendationRenumbersPriorities(t *testing.T) {
	manual := ContentRecommendation{Priority: 99, SuggestedTitle: "Manual"}
	recommendations := []ContentRecommendation{
		{Priority: 1, SuggestedTitle: "Automatic 1"},
		{Priority: 2, SuggestedTitle: "Automatic 2"},
	}

	out := prependContentRecommendation(manual, recommendations)

	require.Len(t, out, 3)
	require.Equal(t, "Manual", out[0].SuggestedTitle)
	require.Equal(t, 1, out[0].Priority)
	require.Equal(t, 2, out[1].Priority)
	require.Equal(t, 3, out[2].Priority)
}

func TestEnrichManualContentEvidenceKeepsManualAndAddsRelevantCompetitorURLs(t *testing.T) {
	manual := ContentRecommendation{
		SuggestedTitle: "Top AI Agent Platforms for Enterprise Teams",
		Competitor:     "lyzr",
		Theme:          "comparison",
		ContentAngle:   "Compare enterprise agent platforms.",
		SuggestedSlug:  "/blogs/ai-agent-platforms",
		TargetIntent:   "commercial evaluation",
		Opportunity:    "Manual content request",
		Pillar:         "AI builder comparisons",
		PageType:       "comparison page",
		ClusterPages:   nil,
		SourceEvidence: []string{"https://manual.example.com/source"},
	}
	opportunities := []Opportunity{
		{
			Title:      "CreateOS should cover \"AI agent platforms\"",
			Competitor: "lyzr",
			Theme:      "comparison",
			Evidence:   []string{"https://www.lyzr.ai/blog/agents", "https://manual.example.com/source"},
		},
	}
	topics := []TopicSummary{
		{
			Competitor:   "stackai",
			Name:         "enterprise agent platforms",
			EvidenceURLs: []string{"https://www.stack-ai.com/blog/platforms"},
		},
	}
	snapshots := []SiteSnapshot{
		{
			Name: "lyzr",
			RecentURLs: []SitemapEntry{
				{URL: "https://www.lyzr.ai/blog/enterprise-ai-agents", Title: "Enterprise AI Agents"},
			},
		},
	}

	out := enrichManualContentEvidence(manual, opportunities, topics, snapshots)

	require.Equal(t, []string{
		"https://manual.example.com/source",
		"https://www.lyzr.ai/blog/agents",
		"https://www.stack-ai.com/blog/platforms",
		"https://www.lyzr.ai/blog/enterprise-ai-agents",
	}, out.SourceEvidence)
}
