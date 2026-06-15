package contentrepo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGitHubPublisherPublishesBlogPR(t *testing.T) {
	requests := []struct {
		Method string
		Path   string
		Body   map[string]any
	}{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		requests = append(requests, struct {
			Method string
			Path   string
			Body   map[string]any
		}{Method: r.Method, Path: r.URL.Path, Body: body})

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/NodeOps-app/createos-content/contents/README.md":
			_, _ = w.Write([]byte(`{"encoding":"base64","content":"YmxvZ3MvCnRpdGxlOgpzbHVnOgpkZXNjcmlwdGlvbjoKYXV0aG9yOgpyZWFkX3RpbWU6CmNvdmVyOgpwdWJsaXNoZWRfYXQ6CmRlc3RpbmF0aW9uCg=="}`))
		case r.Method == http.MethodGet && r.URL.Path == "/repos/NodeOps-app/createos-content/contents/blogs/test-post.md":
			http.Error(w, "not found", http.StatusNotFound)
		case r.Method == http.MethodGet && r.URL.Path == "/repos/NodeOps-app/createos-content/git/ref/heads/main":
			_, _ = w.Write([]byte(`{"object":{"sha":"base-sha"}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/repos/NodeOps-app/createos-content/git/refs":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodPut && r.URL.Path == "/repos/NodeOps-app/createos-content/contents/blogs/test-post.md":
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodPost && r.URL.Path == "/repos/NodeOps-app/createos-content/pulls":
			_, _ = w.Write([]byte(`{"html_url":"https://github.com/NodeOps-app/createos-content/pull/1","number":1}`))
		case r.Method == http.MethodPost && r.URL.Path == "/repos/NodeOps-app/createos-content/issues/1/assignees":
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodPost && r.URL.Path == "/repos/NodeOps-app/createos-content/pulls/1/requested_reviewers":
			_, _ = w.Write([]byte(`{}`))
		default:
			http.Error(w, "unexpected "+r.Method+" "+r.URL.Path, http.StatusTeapot)
		}
	}))
	defer server.Close()

	oldBase := githubAPIBase
	githubAPIBase = server.URL
	defer func() { githubAPIBase = oldBase }()

	publisher := NewGitHubPublisher("ghp_test", "NodeOps-app/createos-content", "main", "navedux")
	publisher.httpClient = server.Client()
	result, err := publisher.Publish(context.Background(), BlogPost{
		Title:        "Test Post",
		Slug:         "test-post",
		Description:  "Description",
		Author:       "CreateOS",
		ReadTime:     "3 min",
		Tags:         []string{"createos"},
		Cover:        "https://example.com/cover.png",
		PublishedAt:  time.Date(2026, 5, 12, 8, 0, 0, 0, time.UTC),
		Destination:  "both",
		BodyMarkdown: "# Test Post\n\nBody",
	}, "competitor-report.json")

	require.NoError(t, err)
	require.Equal(t, "https://github.com/NodeOps-app/createos-content/pull/1", result.PullRequestURL)
	require.Equal(t, "blogs/test-post.md", result.FilePath)

	var putBody map[string]any
	var prBody map[string]any
	for _, request := range requests {
		if request.Method == http.MethodPut {
			putBody = request.Body
		}
		if request.Method == http.MethodPost && strings.HasSuffix(request.Path, "/pulls") {
			prBody = request.Body
		}
	}
	require.Equal(t, "add blog: test-post", putBody["message"])
	require.Contains(t, prBody["body"], "@navedux please review this generated CreateOS blog")
	require.Contains(t, prBody["body"], "Validate the generated cover image before merge.")
	require.Contains(t, prBody["body"], "`blogs/test-post.md`")

	var assigneeBody map[string]any
	var reviewerBody map[string]any
	for _, request := range requests {
		switch {
		case request.Method == http.MethodPost && strings.HasSuffix(request.Path, "/issues/1/assignees"):
			assigneeBody = request.Body
		case request.Method == http.MethodPost && strings.HasSuffix(request.Path, "/pulls/1/requested_reviewers"):
			reviewerBody = request.Body
		}
	}
	require.Equal(t, []any{"navedux"}, assigneeBody["assignees"])
	require.Equal(t, []any{"navedux"}, reviewerBody["reviewers"])
}

func TestGitHubPublisherPublishesRiskyPRWithRiskReviewInBody(t *testing.T) {
	requests := []struct {
		Method string
		Path   string
		Body   map[string]any
	}{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		requests = append(requests, struct {
			Method string
			Path   string
			Body   map[string]any
		}{Method: r.Method, Path: r.URL.Path, Body: body})

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/NodeOps-app/createos-content/contents/README.md":
			_, _ = w.Write([]byte(`{"encoding":"base64","content":"YmxvZ3MvCnRpdGxlOgpzbHVnOgpkZXNjcmlwdGlvbjoKYXV0aG9yOgpyZWFkX3RpbWU6CmNvdmVyOgpwdWJsaXNoZWRfYXQ6CmRlc3RpbmF0aW9uCg=="}`))
		case r.Method == http.MethodGet && r.URL.Path == "/repos/NodeOps-app/createos-content/contents/blogs/risky-post.md":
			http.Error(w, "not found", http.StatusNotFound)
		case r.Method == http.MethodGet && r.URL.Path == "/repos/NodeOps-app/createos-content/git/ref/heads/main":
			_, _ = w.Write([]byte(`{"object":{"sha":"base-sha"}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/repos/NodeOps-app/createos-content/git/refs":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodPut && r.URL.Path == "/repos/NodeOps-app/createos-content/contents/blogs/risky-post.md":
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodPost && r.URL.Path == "/repos/NodeOps-app/createos-content/pulls":
			_, _ = w.Write([]byte(`{"html_url":"https://github.com/NodeOps-app/createos-content/pull/9","number":9}`))
		default:
			http.Error(w, "unexpected "+r.Method+" "+r.URL.Path, http.StatusTeapot)
		}
	}))
	defer server.Close()

	oldBase := githubAPIBase
	githubAPIBase = server.URL
	defer func() { githubAPIBase = oldBase }()

	publisher := NewGitHubPublisher("ghp_test", "NodeOps-app/createos-content", "main", "")
	publisher.httpClient = server.Client()
	result, err := publisher.PublishWithOptions(context.Background(), BlogPost{
		Title:        "Risky Post",
		Slug:         "risky-post",
		Description:  "Description",
		Author:       "CreateOS",
		ReadTime:     "3 min",
		Cover:        "https://example.com/cover.png",
		PublishedAt:  time.Date(2026, 6, 15, 8, 0, 0, 0, time.UTC),
		Destination:  "createos",
		BodyMarkdown: "# Risky Post\n\nBody",
	}, PublishOptions{
		SourceReportPath:    "competitor-report.json",
		TitlePrefix:         "RISKY: ",
		PRBodyExtraMarkdown: "## SEO Risk Review: HIGH\n\nRisk proof.",
	})

	require.NoError(t, err)
	require.Equal(t, 9, result.PullRequestNumber)
	var prBody map[string]any
	for _, request := range requests {
		if request.Method == http.MethodPost && strings.HasSuffix(request.Path, "/pulls") {
			prBody = request.Body
		}
	}
	require.Equal(t, "RISKY: Add CreateOS SEO blog: Risky Post", prBody["title"])
	require.Contains(t, prBody["body"], "## SEO Risk Review: HIGH")
	require.Contains(t, prBody["body"], "Risk proof.")
}

func TestGitHubPublisherValidateCanPublishRejectsExistingContentWithoutMutating(t *testing.T) {
	requests := []struct {
		Method string
		Path   string
	}{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, struct {
			Method string
			Path   string
		}{Method: r.Method, Path: r.URL.Path})

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/NodeOps-app/createos-content/contents/README.md":
			_, _ = w.Write([]byte(`{"encoding":"base64","content":"YmxvZ3MvCnRpdGxlOgpzbHVnOgpkZXNjcmlwdGlvbjoKYXV0aG9yOgpyZWFkX3RpbWU6CmNvdmVyOgpwdWJsaXNoZWRfYXQ6CmRlc3RpbmF0aW9uCg=="}`))
		case r.Method == http.MethodGet && r.URL.Path == "/repos/NodeOps-app/createos-content/contents/blogs/test-post.md":
			_, _ = w.Write([]byte(`{"sha":"existing-sha"}`))
		default:
			http.Error(w, "unexpected "+r.Method+" "+r.URL.Path, http.StatusTeapot)
		}
	}))
	defer server.Close()

	oldBase := githubAPIBase
	githubAPIBase = server.URL
	defer func() { githubAPIBase = oldBase }()

	publisher := NewGitHubPublisher("ghp_test", "NodeOps-app/createos-content", "main", "")
	publisher.httpClient = server.Client()
	err := publisher.ValidateCanPublish(context.Background(), BlogPost{
		Title:        "Test Post",
		Slug:         "test-post",
		Description:  "Description",
		Author:       "CreateOS",
		ReadTime:     "3 min",
		Cover:        "https://example.com/cover.png",
		PublishedAt:  time.Date(2026, 5, 12, 8, 0, 0, 0, time.UTC),
		Destination:  "both",
		BodyMarkdown: "# Test Post\n\nBody",
	})

	require.EqualError(t, err, "content file already exists on main: blogs/test-post.md")
	for _, request := range requests {
		require.Equal(t, http.MethodGet, request.Method, "preflight must not mutate content repo at %s", request.Path)
	}
}
