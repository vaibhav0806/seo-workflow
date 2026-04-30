package competitor

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestClassifyThemes(t *testing.T) {
	themes := classifyThemes("https://example.com/blog/ai-agents-mcp-security")
	require.Contains(t, themes, "ai")
	require.Contains(t, themes, "agents")
	require.Contains(t, themes, "mcp")
	require.Contains(t, themes, "security")
}

func TestDeriveOpportunitiesPrefersTopicSpecificSignals(t *testing.T) {
	ours := SiteSnapshot{
		Name:           "createos",
		RecentURLCount: 2,
		RecentURLs: []SitemapEntry{
			{URL: "https://createos.sh/blog/agent-workflows", ThemeTags: []string{"agents"}},
		},
	}
	competitor := SiteSnapshot{
		Name:           "vercel",
		RecentURLCount: 12,
		RecentURLs: []SitemapEntry{
			{URL: "https://vercel.com/blog/agent-workflows", ThemeTags: []string{"agents"}},
			{URL: "https://vercel.com/blog/agent-workflows-for-teams", ThemeTags: []string{"agents"}},
			{URL: "https://vercel.com/blog/security-incident-update", ThemeTags: []string{"security"}},
		},
	}

	results := deriveOpportunities(ours, []SiteSnapshot{competitor})
	require.NotEmpty(t, results)

	var hasTopicGap bool
	var hasIncident bool
	for _, result := range results {
		if result.OpportunityType == "topic-gap" && strings.Contains(result.Title, "agent workflows") {
			hasTopicGap = true
		}
		if result.OpportunityType == "incident-response" {
			hasIncident = true
		}
		require.GreaterOrEqual(t, result.ImpactScore, 1)
		require.LessOrEqual(t, result.ImpactScore, 100)
	}

	require.True(t, hasTopicGap)
	require.False(t, hasIncident)
}

func TestDeriveTopicOpportunitiesSkipsCoveredCreateOSThemes(t *testing.T) {
	ours := SiteSnapshot{
		Name: "createos",
		RecentURLs: []SitemapEntry{
			{URL: "https://createos.sh/blogs/agentic-deployments-ai-agents-createos", Title: "Agentic deployments for AI agents with CreateOS"},
		},
	}
	topics := []TopicSummary{
		{
			Competitor:           "vercel",
			Name:                 "AI gateway integrations with coding agents",
			PageCount:            8,
			RepresentativeTitles: []string{"Coding Agents · AI Gateway"},
			EvidenceURLs:         []string{"https://vercel.com/docs/ai-gateway/coding-agents"},
			WhyItMatters:         "Vercel is building agent infrastructure intent.",
		},
		{
			Competitor:           "lovable",
			Name:                 "Template gallery for SaaS apps",
			PageCount:            6,
			RepresentativeTitles: []string{"SaaS templates"},
			EvidenceURLs:         []string{"https://lovable.dev/templates/apps/saas"},
			WhyItMatters:         "Lovable is capturing ready-to-build SaaS intent.",
		},
	}

	opportunities := deriveTopicOpportunities(ours, topics)

	require.Len(t, opportunities, 1)
	require.Equal(t, "CreateOS should cover \"Template gallery for SaaS apps\"", opportunities[0].Title)
	require.Equal(t, "llm-topic-gap", opportunities[0].OpportunityType)
}

func TestDeriveTopicOpportunitiesSkipsSingleKeywordCoveredTopic(t *testing.T) {
	ours := SiteSnapshot{
		Name: "createos",
		RecentURLs: []SitemapEntry{
			{URL: "https://createos.sh/blogs/mcp-server-setup", Title: "MCP server setup"},
		},
	}
	topics := []TopicSummary{
		{
			Competitor:   "vercel",
			Name:         "MCP",
			PageCount:    6,
			EvidenceURLs: []string{"https://vercel.com/docs/mcp"},
			WhyItMatters: "MCP demand is rising.",
		},
		{
			Competitor:   "vercel",
			Name:         "Template gallery for SaaS apps",
			PageCount:    6,
			EvidenceURLs: []string{"https://vercel.com/templates/saas"},
			WhyItMatters: "Template demand is rising.",
		},
	}

	opportunities := deriveTopicOpportunities(ours, topics)
	require.Len(t, opportunities, 1)
	require.Equal(t, "CreateOS should cover \"Template gallery for SaaS apps\"", opportunities[0].Title)
}

func TestDeriveTopicOpportunitiesAvoidsSubstringFalsePositive(t *testing.T) {
	ours := SiteSnapshot{
		Name: "createos",
		RecentURLs: []SitemapEntry{
			{URL: "https://createos.sh/blogs/capital-structure-basics", Title: "Capital structure basics"},
		},
	}
	topics := []TopicSummary{
		{
			Competitor:   "lovable",
			Name:         "API integrations",
			PageCount:    4,
			EvidenceURLs: []string{"https://lovable.dev/docs/api/integrations"},
			WhyItMatters: "Integration intent.",
		},
	}

	opportunities := deriveTopicOpportunities(ours, topics)
	require.Len(t, opportunities, 1)
	require.Equal(t, "CreateOS should cover \"API integrations\"", opportunities[0].Title)
}

func TestDeriveTopicOpportunitiesScoresLessCoveredTopicHigher(t *testing.T) {
	ours := SiteSnapshot{
		Name: "createos",
		RecentURLs: []SitemapEntry{
			{URL: "https://createos.sh/blogs/ai-agent-deployment-guide", Title: "AI agent deployment guide"},
		},
	}
	topics := []TopicSummary{
		{
			Competitor:   "vercel",
			Name:         "AI agent deployment",
			PageCount:    6,
			EvidenceURLs: []string{"https://vercel.com/docs/ai/agent/deployment"},
			WhyItMatters: "Covered-ish topic.",
		},
		{
			Competitor:   "vercel",
			Name:         "MCP security hardening",
			PageCount:    6,
			EvidenceURLs: []string{"https://vercel.com/docs/mcp/security"},
			WhyItMatters: "Less covered topic.",
		},
	}

	opportunities := deriveTopicOpportunities(ours, topics)
	require.Len(t, opportunities, 1)
	require.Equal(t, "CreateOS should cover \"MCP security hardening\"", opportunities[0].Title)

	coveredScore := scoreLLMTopicGap(6, 1, 3)
	uncoveredScore := scoreLLMTopicGap(6, 3, 3)
	require.Greater(t, uncoveredScore, coveredScore)
}

func TestInferDateFromURL(t *testing.T) {
	dt := inferDateFromURL("https://example.com/blog/2026/04/29/agent-release")
	require.NotNil(t, dt)
	require.Equal(t, "2026-04-29", dt.Format("2006-01-02"))

	dt = inferDateFromURL("https://example.com/blog/agent-release-2026-04-15")
	require.NotNil(t, dt)
	require.Equal(t, "2026-04-15", dt.Format("2006-01-02"))

	dt = inferDateFromURL("https://example.com/docs/agent-release")
	require.Nil(t, dt)
}

func TestBuildSnapshotSkipsUndatedEntries(t *testing.T) {
	windowStart := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	entries := []rawSitemapEntry{
		{URL: "https://createos.sh/blog/agents"},
		{URL: "https://createos.sh/blog/2026/04/21/agents"},
	}

	snapshot := buildSnapshot("createos", "https://createos.sh/sitemap.xml", entries, windowStart)
	require.Equal(t, 2, snapshot.TotalURLs)
	require.Equal(t, 1, snapshot.RecentURLCount)
}
