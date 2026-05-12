package main

import (
	"testing"

	"github.com/nodeops/seo-workflow/internal/competitor"
	"github.com/stretchr/testify/require"
)

func TestFirstDraftRecommendationReturnsFirstRecommendationWithBody(t *testing.T) {
	recommendation, ok := firstDraftRecommendation([]competitor.ContentRecommendation{
		{Priority: 1, Draft: nil},
		{Priority: 2, Draft: &competitor.BlogDraft{BodyMarkdown: "   "}},
		{Priority: 3, Draft: &competitor.BlogDraft{BodyMarkdown: "# Ready"}},
	})

	require.True(t, ok)
	require.Equal(t, 3, recommendation.Priority)
}
