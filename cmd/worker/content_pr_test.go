package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
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

func TestWriteCompetitorContentPullRequestPreflightsBeforeCoverGeneration(t *testing.T) {
	oldTransport := http.DefaultTransport
	var openRouterCalls int
	var cloudinaryCalls int
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Host {
		case "api.github.com":
			return githubDuplicateContentResponse(r)
		case "openrouter.ai":
			openRouterCalls++
			return jsonResponse(http.StatusOK, `{"choices":[{"message":{"images":[{"image_url":{"url":"data:image/png;base64,Y292ZXI="}}]}}]}`), nil
		case "api.cloudinary.com":
			cloudinaryCalls++
			return jsonResponse(http.StatusOK, `{"secure_url":"https://res.cloudinary.com/demo/image/upload/v1/createos/blog-covers/test-post.png","public_id":"createos/blog-covers/test-post","format":"png","bytes":5}`), nil
		default:
			return nil, fmt.Errorf("unexpected request to %s", r.URL.String())
		}
	})
	defer func() { http.DefaultTransport = oldTransport }()

	err := writeCompetitorContentPullRequest(context.Background(), &config.Config{
		GitHubToken:            "ghp_test",
		ContentRepo:            "NodeOps-app/createos-content",
		ContentBaseBranch:      "main",
		ContentAuthor:          "CreateOS",
		ContentCoverURL:        "https://example.com/default-cover.png",
		OpenRouterAPIKey:       "openrouter-key",
		OpenRouterCoverModel:   "image-model",
		CloudinaryCloudName:    "demo-cloud",
		CloudinaryAPIKey:       "cloudinary-key",
		CloudinaryAPISecret:    "cloudinary-secret",
		CloudinaryUploadFolder: "createos/blog-covers",
	}, competitor.Summary{
		GeneratedAtUTC: "2026-05-22T10:30:00Z",
		ContentPlan: []competitor.ContentRecommendation{{
			SuggestedTitle: "Test Post",
			Draft: &competitor.BlogDraft{
				Route:           "/blog/test-post",
				Title:           "Test Post",
				MetaDescription: "Description",
				BodyMarkdown:    "# Test Post\n\nBody",
			},
		}},
	})

	require.EqualError(t, err, "content file already exists on main: blogs/test-post.md")
	require.Zero(t, openRouterCalls, "cover generation must not run after duplicate content preflight fails")
	require.Zero(t, cloudinaryCalls, "cover upload must not run after duplicate content preflight fails")
}

func TestWriteCompetitorContentPullRequestSkipsDuplicateDraftAndPublishesNext(t *testing.T) {
	oldTransport := http.DefaultTransport
	var blogWrites []string
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "api.github.com" {
			return nil, fmt.Errorf("unexpected request to %s", r.URL.String())
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/NodeOps-app/createos-content/contents/README.md":
			return jsonResponse(http.StatusOK, `{"encoding":"base64","content":"YmxvZ3MvCnRpdGxlOgpzbHVnOgpkZXNjcmlwdGlvbjoKYXV0aG9yOgpyZWFkX3RpbWU6CmNvdmVyOgpwdWJsaXNoZWRfYXQ6CmRlc3RpbmF0aW9uCg=="}`), nil
		case r.Method == http.MethodGet && r.URL.Path == "/repos/NodeOps-app/createos-content/contents/blogs/test-post.md":
			return jsonResponse(http.StatusOK, `{"sha":"existing-sha"}`), nil
		case r.Method == http.MethodGet && r.URL.Path == "/repos/NodeOps-app/createos-content/contents/blogs/next-post.md":
			return jsonResponse(http.StatusNotFound, `{"message":"Not Found"}`), nil
		case r.Method == http.MethodGet && r.URL.Path == "/repos/NodeOps-app/createos-content/git/ref/heads/main":
			return jsonResponse(http.StatusOK, `{"object":{"sha":"base-sha"}}`), nil
		case r.Method == http.MethodPost && r.URL.Path == "/repos/NodeOps-app/createos-content/git/refs":
			return jsonResponse(http.StatusCreated, `{}`), nil
		case r.Method == http.MethodPut && r.URL.Path == "/repos/NodeOps-app/createos-content/contents/blogs/next-post.md":
			blogWrites = append(blogWrites, r.URL.Path)
			return jsonResponse(http.StatusOK, `{"content":{"sha":"new-sha"}}`), nil
		case r.Method == http.MethodPost && r.URL.Path == "/repos/NodeOps-app/createos-content/pulls":
			return jsonResponse(http.StatusCreated, `{"html_url":"https://github.com/NodeOps-app/createos-content/pull/50","number":50}`), nil
		default:
			return jsonResponse(http.StatusTeapot, `{"message":"unexpected github request"}`), nil
		}
	})
	defer func() { http.DefaultTransport = oldTransport }()

	err := writeCompetitorContentPullRequest(context.Background(), &config.Config{
		GitHubToken:       "ghp_test",
		ContentRepo:       "NodeOps-app/createos-content",
		ContentBaseBranch: "main",
		ContentAuthor:     "CreateOS",
		ContentCoverURL:   "https://example.com/default-cover.png",
	}, competitor.Summary{
		GeneratedAtUTC: "2026-05-22T10:30:00Z",
		ContentPlan: []competitor.ContentRecommendation{
			{
				SuggestedTitle: "Test Post",
				Draft: &competitor.BlogDraft{
					Route:           "/blogs/test-post",
					Title:           "Test Post",
					MetaDescription: "Description",
					BodyMarkdown:    "# Test Post\n\nBody",
				},
			},
			{
				SuggestedTitle: "Next Post",
				Draft: &competitor.BlogDraft{
					Route:           "/blogs/next-post",
					Title:           "Next Post",
					MetaDescription: "Description",
					BodyMarkdown:    "# Next Post\n\nBody",
				},
			},
		},
	})

	require.NoError(t, err)
	require.Equal(t, []string{"/repos/NodeOps-app/createos-content/contents/blogs/next-post.md"}, blogWrites)
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

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}

func githubDuplicateContentResponse(r *http.Request) (*http.Response, error) {
	if r.Method != http.MethodGet {
		return jsonResponse(http.StatusTeapot, `{"message":"unexpected mutation"}`), nil
	}
	switch r.URL.Path {
	case "/repos/NodeOps-app/createos-content/contents/README.md":
		return jsonResponse(http.StatusOK, `{"encoding":"base64","content":"YmxvZ3MvCnRpdGxlOgpzbHVnOgpkZXNjcmlwdGlvbjoKYXV0aG9yOgpyZWFkX3RpbWU6CmNvdmVyOgpwdWJsaXNoZWRfYXQ6CmRlc3RpbmF0aW9uCg=="}`), nil
	case "/repos/NodeOps-app/createos-content/contents/blogs/test-post.md":
		return jsonResponse(http.StatusOK, `{"sha":"existing-sha"}`), nil
	default:
		return jsonResponse(http.StatusTeapot, `{"message":"unexpected github path"}`), nil
	}
}

func jsonResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
