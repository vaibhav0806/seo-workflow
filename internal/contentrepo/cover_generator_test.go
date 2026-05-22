package contentrepo

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCoverPromptAutoMapsPrototypeTopicsToGreenLushStyle(t *testing.T) {
	post := BlogPost{
		Title:        "Rapid Prototyping MVP Development",
		Description:  "How teams move from idea to working MVP without losing context.",
		Slug:         "rapid-prototyping-mvp-development",
		Tags:         []string{"vibecoding", "mvp"},
		Author:       "CreateOS",
		Cover:        "https://example.com/cover.png",
		PublishedAt:  time.Date(2026, 5, 19, 8, 30, 0, 0, time.UTC),
		Destination:  DestinationCreateOS,
		BodyMarkdown: "# Rapid Prototyping\n\nBody.",
	}

	prompt := coverPrompt(post, "auto")

	require.Contains(t, prompt, "Cover style: green-lush")
	require.Contains(t, prompt, "Green Lush Aesthetic")
	require.Contains(t, prompt, "lush, fuzzy green moss")
	require.Contains(t, prompt, "Frutiger Aero and Solarpunk")
	require.Contains(t, prompt, "sketch-like light trails")
	require.Contains(t, prompt, "no readable text")
	require.NotContains(t, prompt, "[ARTICLE-SPECIFIC")
}

func TestCoverPromptAutoMapsEnterpriseTopicsToSoftTechFurryStyle(t *testing.T) {
	post := BlogPost{
		Title:        "Enterprise Security Governance",
		Description:  "How teams govern AI app development without fragmenting execution.",
		Slug:         "enterprise-security-governance",
		Tags:         []string{"security", "enterprise"},
		Author:       "CreateOS",
		Cover:        "https://example.com/cover.png",
		PublishedAt:  time.Date(2026, 5, 19, 8, 30, 0, 0, time.UTC),
		Destination:  DestinationCreateOS,
		BodyMarkdown: "# Enterprise Security Governance\n\nBody.",
	}

	prompt := coverPrompt(post, "auto")

	require.Contains(t, prompt, "Cover style: soft-tech-furry")
	require.Contains(t, prompt, "Soft-Tech Furry Aesthetic")
	require.Contains(t, prompt, "soft, minimal white fur")
	require.Contains(t, prompt, "No roots, no flowers")
	require.Contains(t, prompt, "policy gates")
	require.Contains(t, prompt, "protected workflow layers")
	require.Contains(t, prompt, "no readable text")
	require.NotContains(t, prompt, "lush, fuzzy green moss")
}

func TestCoverPromptAutoMapsComparisonTopicsToSurrealDreamscapeStyle(t *testing.T) {
	post := BlogPost{
		Title:        "Best Lovable Alternatives for Product Teams",
		Description:  "Compare AI app builders and choose the right route for your team.",
		Slug:         "lovable-alternatives-product-teams",
		Tags:         []string{"comparison", "alternatives"},
		Author:       "CreateOS",
		Cover:        "https://example.com/cover.png",
		PublishedAt:  time.Date(2026, 5, 19, 8, 30, 0, 0, time.UTC),
		Destination:  DestinationCreateOS,
		BodyMarkdown: "# Alternatives\n\nBody.",
	}

	prompt := coverPrompt(post, "auto")

	require.Contains(t, prompt, "Cover style: surreal-dreamscape")
	require.Contains(t, prompt, "Surreal Dreamscape Aesthetic")
	require.Contains(t, prompt, "lavender purple, warm coral orange, and soft pink hues")
	require.Contains(t, prompt, "two or three winding paths")
	require.Contains(t, prompt, "no readable text")
}

func TestCoverPromptHonorsExplicitStyleOverride(t *testing.T) {
	post := BlogPost{
		Title:       "Enterprise Security Governance",
		Description: "Security and governance for AI app development.",
		Slug:        "enterprise-security-governance",
		Tags:        []string{"security"},
	}

	prompt := coverPrompt(post, "green-lush")

	require.Contains(t, prompt, "Cover style: green-lush")
	require.Contains(t, prompt, "Green Lush Aesthetic")
	require.NotContains(t, prompt, "Soft-Tech Furry Aesthetic")
}

func TestSelectedCoverStyleFallbackIsStableBySlug(t *testing.T) {
	post := BlogPost{
		Title:       "Build Better Internal Tools",
		Description: "A general guide for product teams.",
		Slug:        "build-better-internal-tools",
	}

	first := selectedCoverStyle(post, "auto")
	second := selectedCoverStyle(post, "auto")

	require.Contains(t, []string{"green-lush", "soft-tech-furry", "surreal-dreamscape"}, first.ID)
	require.Equal(t, first.ID, second.ID)
}

func TestGeneratedCoverFromDataURLAllowsEmptyAssetBaseURL(t *testing.T) {
	post := BlogPost{Slug: "test-post"}
	content := []byte("cover-bytes")
	dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(content)

	cover, err := generatedCoverFromDataURL(post, dataURL, "")

	require.NoError(t, err)
	require.Empty(t, cover.URL)
	require.Equal(t, "covers/test-post.png", cover.Asset.Path)
	require.Equal(t, content, cover.Asset.Content)
}

func TestGeneratedCoverFromDataURLBuildsURLWithAssetBaseURL(t *testing.T) {
	post := BlogPost{Slug: "test-post"}
	content := []byte("cover-bytes")
	dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(content)

	cover, err := generatedCoverFromDataURL(post, dataURL, "https://cdn.example.com/createos-content/")

	require.NoError(t, err)
	require.Equal(t, "https://cdn.example.com/createos-content/covers/test-post.png", cover.URL)
	require.Equal(t, "covers/test-post.png", cover.Asset.Path)
	require.Equal(t, content, cover.Asset.Content)
}
