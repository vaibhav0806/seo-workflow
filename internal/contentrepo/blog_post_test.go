package contentrepo

import (
	"testing"
	"time"

	"github.com/nodeops/seo-workflow/internal/competitor"
	"github.com/stretchr/testify/require"
)

func TestBuildBlogPostUsesContentRepoFrontmatterContract(t *testing.T) {
	generatedAt := time.Date(2026, 5, 12, 8, 30, 1, 0, time.UTC)
	post, err := BuildBlogPost(competitor.ContentRecommendation{
		Theme:          "vibecoding",
		PageType:       "use-case landing page",
		Pillar:         "AI app-building use cases",
		ContentAngle:   "Build MVPs without context switching.",
		SuggestedTitle: "Fallback Title",
		Draft: &competitor.BlogDraft{
			Route:           "/use-cases/rapid-prototyping-mvp",
			Title:           "From Idea to Working MVP",
			MetaDescription: "Learn how CreateOS helps teams move from concept to working MVP without losing context across tools.",
			BodyMarkdown:    "# From Idea to Working MVP\n\nCreateOS keeps the work in one flow.",
		},
	}, generatedAt, "CreateOS", "https://example.com/cover.png")

	require.NoError(t, err)
	require.Equal(t, "rapid-prototyping-mvp", post.Slug)
	require.Equal(t, "blogs/rapid-prototyping-mvp.md", post.FilePath())
	require.Equal(t, "3 min", post.ReadTime)
	require.Equal(t, []string{"createos", "vibecoding", "AI app-building use cases", "use-case landing page"}, post.Tags)

	markdown := post.Markdown()
	require.Contains(t, markdown, `title: "From Idea to Working MVP"`)
	require.Contains(t, markdown, "slug: rapid-prototyping-mvp")
	require.Contains(t, markdown, `author: "CreateOS"`)
	require.Contains(t, markdown, `cover: "https://example.com/cover.png"`)
	require.Contains(t, markdown, `published_at: "2026-05-12T08:30:01.000Z"`)
	require.Contains(t, markdown, "destination: createos")
	require.Contains(t, markdown, "# From Idea to Working MVP")
}

func TestBuildBlogPostLocksManualRecommendationTitle(t *testing.T) {
	generatedAt := time.Date(2026, 6, 15, 9, 30, 1, 0, time.UTC)
	post, err := BuildBlogPost(competitor.ContentRecommendation{
		Opportunity:    "Manual content request: Top AI App Builders for Production-Ready Apps",
		Theme:          "comparison",
		PageType:       "comparison page",
		Pillar:         "AI builder comparisons",
		ContentAngle:   "Rank AI app builders for production readiness.",
		SuggestedTitle: "Top AI App Builders for Production-Ready Apps",
		Draft: &competitor.BlogDraft{
			Route:           "/blogs/ai-app-builders-production-ready-apps",
			Title:           "AI App Builders Ship Fast. Most Fall Apart in Production.",
			MetaDescription: "Compare AI app builders by production readiness.",
			BodyMarkdown:    "# AI App Builders Ship Fast. Most Fall Apart in Production.\n\nCreateOS keeps the work production-ready.",
		},
	}, generatedAt, "CreateOS", "https://example.com/cover.png")

	require.NoError(t, err)
	require.Equal(t, "Top AI App Builders for Production-Ready Apps", post.Title)
	require.Contains(t, post.Markdown(), `title: "Top AI App Builders for Production-Ready Apps"`)
}

func TestBuildBlogPostRequiresDraft(t *testing.T) {
	_, err := BuildBlogPost(competitor.ContentRecommendation{}, time.Now(), "CreateOS", "https://example.com/cover.png")

	require.Error(t, err)
	require.Contains(t, err.Error(), "no draft")
}
