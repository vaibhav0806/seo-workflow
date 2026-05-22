package main

import (
	"context"
	"errors"
	"testing"

	"github.com/nodeops/seo-workflow/internal/competitor"
	"github.com/nodeops/seo-workflow/internal/config"
	"github.com/nodeops/seo-workflow/internal/contentrepo"
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

func TestApplyGeneratedCoverUploadsToCloudinaryWhenUploaderProvided(t *testing.T) {
	post := &contentrepo.BlogPost{
		Slug:  "test-post",
		Cover: "https://example.com/default.png",
	}
	coverContent := []byte("cover-bytes")
	cover := contentrepo.GeneratedCover{
		URL: "https://cdn.example.com/covers/test-post.png",
		Asset: contentrepo.CoverAsset{
			Path:    "covers/test-post.png",
			Content: coverContent,
		},
	}
	uploader := &stubCoverUploader{
		result: contentrepo.CoverUploadResult{
			URL: "https://res.cloudinary.com/demo/image/upload/test-post.png",
		},
	}

	assets := applyGeneratedCover(context.Background(), post, cover, uploader)

	require.Empty(t, assets)
	require.Equal(t, "https://res.cloudinary.com/demo/image/upload/test-post.png", post.Cover)
	require.Len(t, uploader.received, 1)
	require.Equal(t, contentrepo.CoverUploadAsset{
		Slug:        "test-post",
		Path:        "covers/test-post.png",
		Content:     coverContent,
		ContentType: "image/png",
	}, uploader.received[0])
}

func TestApplyGeneratedCoverFallsBackToRepoAssetURLWithoutUploader(t *testing.T) {
	post := &contentrepo.BlogPost{
		Slug:  "test-post",
		Cover: "https://example.com/default.png",
	}
	coverContent := []byte("cover-bytes")
	cover := contentrepo.GeneratedCover{
		URL: "https://cdn.example.com/covers/test-post.png",
		Asset: contentrepo.CoverAsset{
			Path:    "covers/test-post.png",
			Content: coverContent,
		},
	}

	assets := applyGeneratedCover(context.Background(), post, cover, nil)

	require.Equal(t, "https://cdn.example.com/covers/test-post.png", post.Cover)
	require.Equal(t, []contentrepo.CoverAsset{
		{Path: "covers/test-post.png", Content: coverContent},
	}, assets)
}

func TestApplyGeneratedCoverKeepsDefaultCoverWhenUploaderFails(t *testing.T) {
	post := &contentrepo.BlogPost{
		Slug:  "test-post",
		Cover: "https://example.com/default.png",
	}
	cover := contentrepo.GeneratedCover{
		URL: "https://cdn.example.com/covers/test-post.png",
		Asset: contentrepo.CoverAsset{
			Path:    "covers/test-post.png",
			Content: []byte("cover-bytes"),
		},
	}
	uploader := &stubCoverUploader{err: errors.New("upload failed")}

	assets := applyGeneratedCover(context.Background(), post, cover, uploader)

	require.Empty(t, assets)
	require.Equal(t, "https://example.com/default.png", post.Cover)
	require.Len(t, uploader.received, 1)
}

func TestNewCoverUploaderFromConfigRequiresAllCloudinaryCredentials(t *testing.T) {
	require.Nil(t, newCoverUploaderFromConfig(&config.Config{
		CloudinaryAPIKey:       "cloudinary-key",
		CloudinaryAPISecret:    "cloudinary-secret",
		CloudinaryUploadFolder: "createos/blog-covers",
	}))
	require.Nil(t, newCoverUploaderFromConfig(&config.Config{
		CloudinaryCloudName:    "demo-cloud",
		CloudinaryAPISecret:    "cloudinary-secret",
		CloudinaryUploadFolder: "createos/blog-covers",
	}))
	require.Nil(t, newCoverUploaderFromConfig(&config.Config{
		CloudinaryCloudName:    "demo-cloud",
		CloudinaryAPIKey:       "cloudinary-key",
		CloudinaryUploadFolder: "createos/blog-covers",
	}))
	require.NotNil(t, newCoverUploaderFromConfig(&config.Config{
		CloudinaryCloudName:    "demo-cloud",
		CloudinaryAPIKey:       "cloudinary-key",
		CloudinaryAPISecret:    "cloudinary-secret",
		CloudinaryUploadFolder: "createos/blog-covers",
	}))
}

func TestShouldGenerateCoverWithCloudinaryCredentialsAndEmptyAssetBaseURL(t *testing.T) {
	cfg := &config.Config{
		OpenRouterAPIKey:       "openrouter-key",
		OpenRouterCoverModel:   "image-model",
		CloudinaryCloudName:    "demo-cloud",
		CloudinaryAPIKey:       "cloudinary-key",
		CloudinaryAPISecret:    "cloudinary-secret",
		CloudinaryUploadFolder: "createos/blog-covers",
	}

	require.True(t, shouldGenerateCover(cfg, newCoverUploaderFromConfig(cfg)))
}

func TestShouldGenerateCoverWithAssetBaseURLAndNoCloudinary(t *testing.T) {
	cfg := &config.Config{
		OpenRouterAPIKey:         "openrouter-key",
		OpenRouterCoverModel:     "image-model",
		ContentCoverAssetBaseURL: "https://cdn.example.com/createos-content",
		CloudinaryUploadFolder:   "createos/blog-covers",
	}

	require.True(t, shouldGenerateCover(cfg, newCoverUploaderFromConfig(cfg)))
}

func TestShouldGenerateCoverRequiresPublishTarget(t *testing.T) {
	cfg := &config.Config{
		OpenRouterAPIKey:       "openrouter-key",
		OpenRouterCoverModel:   "image-model",
		CloudinaryUploadFolder: "createos/blog-covers",
	}

	require.False(t, shouldGenerateCover(cfg, newCoverUploaderFromConfig(cfg)))
}

type stubCoverUploader struct {
	result   contentrepo.CoverUploadResult
	err      error
	received []contentrepo.CoverUploadAsset
}

func (uploader *stubCoverUploader) UploadCover(_ context.Context, asset contentrepo.CoverUploadAsset) (contentrepo.CoverUploadResult, error) {
	uploader.received = append(uploader.received, asset)
	if uploader.err != nil {
		return contentrepo.CoverUploadResult{}, uploader.err
	}
	return uploader.result, nil
}
