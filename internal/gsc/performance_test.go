package gsc

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAnalyzePerformanceFindsStrikingDistanceAndLowCTR(t *testing.T) {
	current := PerformanceSnapshot{Rows: []PerformanceMetric{
		{Date: "2026-08-20", Query: "deploy ai agent", Page: "https://createos.sh/blogs/deploy-ai-agent", Clicks: 1, Impressions: 100, CTR: 0.01, Position: 8},
	}}

	report := AnalyzePerformance(current, PerformanceSnapshot{})

	require.Contains(t, performanceOpportunityTypes(report.Opportunities), OpportunityStrikingDistance)
	require.Contains(t, performanceOpportunityTypes(report.Opportunities), OpportunityLowCTR)
}

func TestAnalyzePerformanceFindsQueryCannibalization(t *testing.T) {
	current := PerformanceSnapshot{Rows: []PerformanceMetric{
		{Query: "ai agent governance", Page: "https://createos.sh/blogs/ai-agent-governance", Impressions: 30, Position: 7},
		{Query: "ai agent governance", Page: "https://createos.sh/blogs/governance-that-works", Impressions: 20, Position: 10},
	}}

	report := AnalyzePerformance(current, PerformanceSnapshot{})

	require.Len(t, report.Cannibalization, 1)
	require.Equal(t, "ai agent governance", report.Cannibalization[0].Query)
	require.ElementsMatch(t, []string{
		"https://createos.sh/blogs/ai-agent-governance",
		"https://createos.sh/blogs/governance-that-works",
	}, report.Cannibalization[0].Pages)
}

func TestAnalyzePerformanceFindsClickDeclineAgainstPreviousSnapshot(t *testing.T) {
	previous := PerformanceSnapshot{Rows: []PerformanceMetric{
		{Query: "mcp server hosting", Page: "https://createos.sh/blogs/mcp-server-hosting", Clicks: 20, Impressions: 100, Position: 5},
	}}
	current := PerformanceSnapshot{Rows: []PerformanceMetric{
		{Query: "mcp server hosting", Page: "https://createos.sh/blogs/mcp-server-hosting", Clicks: 10, Impressions: 100, Position: 6},
	}}

	report := AnalyzePerformance(current, previous)

	require.Contains(t, performanceOpportunityTypes(report.Opportunities), OpportunityDeclining)
}

func TestPerformanceStatePersistsHistoryAndKeepsEightSnapshots(t *testing.T) {
	path := filepath.Join(t.TempDir(), "performance.json")
	state := PerformanceState{}
	for index := 0; index < 10; index++ {
		state = AppendPerformanceSnapshot(state, PerformanceSnapshot{GeneratedAtUTC: string(rune('A' + index))})
	}
	require.NoError(t, SavePerformanceState(path, state))

	loaded, err := LoadPerformanceState(path)

	require.NoError(t, err)
	require.Len(t, loaded.Snapshots, 8)
	require.Equal(t, "C", loaded.Snapshots[0].GeneratedAtUTC)
	require.Equal(t, "J", loaded.Snapshots[7].GeneratedAtUTC)
}

func performanceOpportunityTypes(opportunities []SearchOpportunity) []OpportunityType {
	types := make([]OpportunityType, 0, len(opportunities))
	for _, opportunity := range opportunities {
		types = append(types, opportunity.Type)
	}
	return types
}
