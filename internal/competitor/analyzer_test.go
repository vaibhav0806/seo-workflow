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
