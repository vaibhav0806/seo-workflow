package contentrepo

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCoverPromptUsesCreateOSPastelNatureTechStyle(t *testing.T) {
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

	prompt := coverPrompt(post)

	require.Contains(t, prompt, "bright surreal 3D landscape")
	require.Contains(t, prompt, "soft pastel terrain")
	require.Contains(t, prompt, "moss, vines, flowers")
	require.Contains(t, prompt, "glassy futuristic technology objects")
	require.Contains(t, prompt, "policy gates")
	require.Contains(t, prompt, "protected workflow layers")
	require.Contains(t, prompt, "no readable text")
	require.NotContains(t, prompt, "dark graphite")
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
