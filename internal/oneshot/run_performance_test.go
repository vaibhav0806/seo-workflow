package oneshot

import (
	"path/filepath"
	"testing"

	"github.com/nodeops/seo-workflow/internal/gsc"
	"github.com/stretchr/testify/require"
)

func TestUpdatePerformanceFeedbackUsesLastRunAndPersistsCurrent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "performance.json")
	previous := gsc.PerformanceSnapshot{Rows: []gsc.PerformanceMetric{{Query: "deploy ai agent", Page: "/blogs/deploy-ai-agent", Clicks: 8, Impressions: 40, Position: 5}}}
	require.NoError(t, gsc.SavePerformanceState(path, gsc.PerformanceState{Snapshots: []gsc.PerformanceSnapshot{previous}}))
	current := gsc.PerformanceSnapshot{Rows: []gsc.PerformanceMetric{{Query: "deploy ai agent", Page: "/blogs/deploy-ai-agent", Clicks: 4, Impressions: 40, Position: 6}}}

	report, err := updatePerformanceFeedback(path, current)

	require.NoError(t, err)
	require.NotNil(t, report.Previous)
	require.Equal(t, gsc.OpportunityDeclining, report.Opportunities[0].Type)
	state, err := gsc.LoadPerformanceState(path)
	require.NoError(t, err)
	require.Len(t, state.Snapshots, 2)
}
