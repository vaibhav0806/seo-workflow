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
			_, _ = w.Write([]byte(`{"html_url":"https://github.com/NodeOps-app/createos-content/pull/1"}`))
		default:
			http.Error(w, "unexpected "+r.Method+" "+r.URL.Path, http.StatusTeapot)
		}
	}))
	defer server.Close()

	oldBase := githubAPIBase
	githubAPIBase = server.URL
	defer func() { githubAPIBase = oldBase }()

	publisher := NewGitHubPublisher("ghp_test", "NodeOps-app/createos-content", "main", "vaibhav0806")
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
	require.Contains(t, prBody["body"], "@vaibhav0806 please review and take the next action on Multica.")
	require.Contains(t, prBody["body"], "`blogs/test-post.md`")
}
