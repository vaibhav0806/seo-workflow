package contentrepo

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/color"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/image/font/gofont/goregular"
)

func TestCoverHeadlineUsesAtMostEightWords(t *testing.T) {
	post := BlogPost{
		Title:        "How to build reliable production workflows with CreateOS today",
		Description:  "How teams govern AI app development without fragmenting execution.",
		Slug:         "enterprise-security-governance",
		Tags:         []string{"security", "enterprise"},
		Author:       "CreateOS",
		Cover:        "https://example.com/cover.png",
		PublishedAt:  time.Date(2026, 5, 19, 8, 30, 0, 0, time.UTC),
		Destination:  DestinationCreateOS,
		BodyMarkdown: "# Enterprise Security Governance\n\nBody.",
	}

	headline := coverHeadline(post)

	require.Equal(t, "How to build reliable production workflows with CreateOS", headline)
}

func TestRenderDesignSystemCoverBuilds1200By675PNG(t *testing.T) {
	post := BlogPost{
		Title: "How to build better workflows with CreateOS",
		Slug:  "better-workflows",
	}

	content, err := renderDesignSystemCover(post, goregular.TTF, goregular.TTF)
	require.NoError(t, err)

	config, format, err := image.DecodeConfig(bytes.NewReader(content))
	require.NoError(t, err)
	require.Equal(t, "png", format)
	require.Equal(t, 1200, config.Width)
	require.Equal(t, 675, config.Height)

	decoded, _, err := image.Decode(bytes.NewReader(content))
	require.NoError(t, err)
	require.Equal(t, color.RGBA{R: 1, G: 88, B: 165, A: 255}, color.RGBAModel.Convert(decoded.At(0, 0)))
	require.Equal(t, color.RGBA{R: 161, G: 225, B: 249, A: 255}, color.RGBAModel.Convert(decoded.At(0, 674)))
	require.Equal(t, color.RGBA{R: 255, G: 255, B: 255, A: 255}, color.RGBAModel.Convert(decoded.At(50, 50)))
	require.NotEqual(t, color.RGBAModel.Convert(decoded.At(0, 260)), color.RGBAModel.Convert(decoded.At(800, 260)))
}

func TestRenderDesignSystemCoverRejectsHeadlineThatExceedsTemplateWidth(t *testing.T) {
	post := BlogPost{Title: strings.Repeat("W", 100), Slug: "wide-headline"}

	_, err := renderDesignSystemCover(post, goregular.TTF, goregular.TTF)

	require.ErrorContains(t, err, "exceeds 638px at 82px")
}

func TestFetchCoverFontCachesVerifiedContent(t *testing.T) {
	fontContent := []byte("font-content")
	digest := sha256.Sum256(fontContent)
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount++
		_, _ = w.Write(fontContent)
	}))
	defer server.Close()

	first, err := fetchCoverFont(context.Background(), server.URL, hex.EncodeToString(digest[:]))
	require.NoError(t, err)
	second, err := fetchCoverFont(context.Background(), server.URL, hex.EncodeToString(digest[:]))
	require.NoError(t, err)

	require.Equal(t, fontContent, first)
	require.Equal(t, fontContent, second)
	require.Equal(t, 1, requestCount)
}

func TestGeneratedDesignSystemCoverAllowsEmptyAssetBaseURL(t *testing.T) {
	post := BlogPost{Slug: "test-post"}
	content := []byte("cover-bytes")

	cover := generatedDesignSystemCover(post, content, "")

	require.Empty(t, cover.URL)
	require.Equal(t, "covers/test-post.png", cover.Asset.Path)
	require.Equal(t, content, cover.Asset.Content)
}

func TestGeneratedDesignSystemCoverBuildsURLWithAssetBaseURL(t *testing.T) {
	post := BlogPost{Slug: "test-post"}
	content := []byte("cover-bytes")

	cover := generatedDesignSystemCover(post, content, "https://cdn.example.com/createos-content/")

	require.Equal(t, "https://cdn.example.com/createos-content/covers/test-post.png", cover.URL)
	require.Equal(t, "covers/test-post.png", cover.Asset.Path)
	require.Equal(t, content, cover.Asset.Content)
}
