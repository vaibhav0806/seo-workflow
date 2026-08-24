package competitor

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildAEOPromptMatrixIncludesCommercialAndAnswerPrompts(t *testing.T) {
	prompts := buildAEOPromptMatrix([]ContentRecommendation{
		{Theme: "comparison", SuggestedTitle: "Best AI Agent Builders"},
		{Theme: "workflow", SuggestedTitle: "AI Workflow Automation"},
	})

	require.Len(t, prompts, 2)
	require.Equal(t, "commercial_listicle", prompts[0].Intent)
	require.Contains(t, prompts[0].Prompt, "best AI agent platforms")
	require.Equal(t, "workflow_answer", prompts[1].Intent)
}

func TestBuildAEOPromptMatrixDedupesThemes(t *testing.T) {
	prompts := buildAEOPromptMatrix([]ContentRecommendation{
		{Theme: "comparison", SuggestedTitle: "Best AI Agent Builders"},
		{Theme: "comparison", SuggestedTitle: "Best Vibe Coding Tools"},
	})

	require.Len(t, prompts, 1)
}
