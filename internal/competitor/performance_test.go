package competitor

import (
	"path/filepath"
	"testing"

	"github.com/nodeops/seo-workflow/internal/gsc"
	"github.com/stretchr/testify/require"
)

func TestApplyPerformanceSignalsChangesNewBlogToRefresh(t *testing.T) {
	plan := []ContentRecommendation{{
		SuggestedTitle: "How to Deploy an AI Agent", PrimaryKeyword: "deploy ai agent",
		SuggestedSlug: "/blogs/deploy-ai-agent", Decision: ContentDecisionCreate,
	}}
	report := gsc.PerformanceReport{Opportunities: []gsc.SearchOpportunity{{
		Type: gsc.OpportunityStrikingDistance, Query: "deploy ai agent",
		Page: "https://createos.sh/blogs/deploy-an-ai-agent/", Score: 80,
	}}}

	updated := applyPerformanceSignals(plan, report)

	require.Equal(t, ContentDecisionRefresh, updated[0].Decision)
	require.Equal(t, "/blogs/deploy-an-ai-agent", updated[0].ExistingRoute)
	require.Len(t, updated[0].SearchSignals, 1)
}

func TestLoadConfiguredPerformanceComparesLatestSnapshots(t *testing.T) {
	path := filepath.Join(t.TempDir(), "performance.json")
	previous := gsc.PerformanceSnapshot{Rows: []gsc.PerformanceMetric{{Query: "mcp hosting", Page: "/blogs/mcp-hosting", Clicks: 8, Impressions: 40, Position: 5}}}
	current := gsc.PerformanceSnapshot{Rows: []gsc.PerformanceMetric{{Query: "mcp hosting", Page: "/blogs/mcp-hosting", Clicks: 4, Impressions: 40, Position: 6}}}
	require.NoError(t, gsc.SavePerformanceState(path, gsc.PerformanceState{Snapshots: []gsc.PerformanceSnapshot{previous, current}}))

	report, warnings := loadConfiguredPerformance(path)

	require.Empty(t, warnings)
	require.Equal(t, gsc.OpportunityDeclining, report.Opportunities[0].Type)
}
