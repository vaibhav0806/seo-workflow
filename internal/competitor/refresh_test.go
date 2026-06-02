package competitor

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildRefreshRecommendationsPrioritizesVolatileComparisonContent(t *testing.T) {
	recommendations := buildRefreshRecommendations([]ContentRecommendation{
		{
			Priority:       1,
			SuggestedSlug:  "/compare/best-ai-agent-builders",
			SuggestedTitle: "Best AI Agent Builders",
			Theme:          "comparison",
			SourceEvidence: []string{"https://www.stackai.com/insights/best-ai-agent-building-platforms-in-2026"},
		},
	})

	require.Len(t, recommendations, 1)
	require.Equal(t, "/compare/best-ai-agent-builders", recommendations[0].Path)
	require.Equal(t, "monthly", recommendations[0].Cadence)
	require.GreaterOrEqual(t, recommendations[0].PriorityScore, 80)
}

func TestBuildRefreshRecommendationsSkipsLowVolatilityGeneralContent(t *testing.T) {
	recommendations := buildRefreshRecommendations([]ContentRecommendation{
		{
			Priority:       1,
			SuggestedSlug:  "/blogs/company-update",
			SuggestedTitle: "Company Update",
			Theme:          "general",
		},
	})

	require.Empty(t, recommendations)
}
