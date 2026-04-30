package oneshot

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nodeops/seo-workflow/internal/pr"
)

const (
	githubAPIBase = "https://api.github.com"
)

type GitHubOpener struct {
	httpClient    *http.Client
	token         string
	repoFullName  string
	baseBranch    string
	sitemapPath   string
	dryRun        bool
	headBranch    string
	commitMessage string
}

type githubRepoResponse struct {
	DefaultBranch string `json:"default_branch"`
}

type githubRefResponse struct {
	Object struct {
		SHA string `json:"sha"`
	} `json:"object"`
}

type githubContentResponse struct {
	SHA string `json:"sha"`
}

type githubPRResponse struct {
	HTMLURL string `json:"html_url"`
}

func NewGitHubOpener(
	token string,
	repo string,
	baseBranch string,
	sitemapPath string,
	dryRun bool,
) *GitHubOpener {
	return &GitHubOpener{
		httpClient:    &http.Client{Timeout: 30 * time.Second},
		token:         token,
		repoFullName:  repo,
		baseBranch:    strings.TrimSpace(baseBranch),
		sitemapPath:   sitemapPath,
		dryRun:        dryRun,
		headBranch:    "seo/fix-sitemap-404",
		commitMessage: "fix(seo): remove 404 URLs from sitemap",
	}
}

func (o *GitHubOpener) Open(ctx context.Context, property string, plan pr.PullRequestPlan) (string, error) {
	if o.dryRun {
		return fmt.Sprintf("dry-run://%s?property=%s&branch=%s", o.repoFullName, url.QueryEscape(property), url.QueryEscape(plan.Branch)), nil
	}

	owner, repo, err := splitRepo(o.repoFullName)
	if err != nil {
		return "", err
	}

	baseBranch := o.baseBranch
	if baseBranch == "" {
		baseBranch, err = o.fetchDefaultBranch(ctx, owner, repo)
		if err != nil {
			return "", err
		}
	}

	baseSHA, err := o.fetchBranchSHA(ctx, owner, repo, baseBranch)
	if err != nil {
		return "", err
	}

	headBranch := plan.Branch
	if strings.TrimSpace(headBranch) == "" {
		headBranch = o.headBranch
	}
	if err := o.createOrUpdateBranch(ctx, owner, repo, headBranch, baseSHA); err != nil {
		return "", err
	}

	var sitemapContent string
	for _, file := range plan.Files {
		if strings.TrimSpace(file.Path) == o.sitemapPath {
			sitemapContent = file.Content
			break
		}
	}
	if sitemapContent == "" {
		return "", fmt.Errorf("plan missing %q file edit", o.sitemapPath)
	}

	sha, _ := o.fetchContentSHA(ctx, owner, repo, o.sitemapPath, headBranch)
	if err := o.putContent(ctx, owner, repo, o.sitemapPath, headBranch, sha, sitemapContent); err != nil {
		return "", err
	}

	prURL, err := o.createPullRequest(ctx, owner, repo, baseBranch, headBranch, plan)
	if err != nil {
		return "", err
	}
	return prURL, nil
}

func splitRepo(repo string) (string, string, error) {
	parts := strings.Split(strings.TrimSpace(repo), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid SCAN_REPO %q, expected owner/repo", repo)
	}
	return parts[0], parts[1], nil
}

func (o *GitHubOpener) fetchDefaultBranch(ctx context.Context, owner string, repo string) (string, error) {
	var parsed githubRepoResponse
	if err := o.requestJSON(ctx, http.MethodGet, fmt.Sprintf("%s/repos/%s/%s", githubAPIBase, owner, repo), nil, &parsed); err != nil {
		return "", fmt.Errorf("fetch default branch: %w", err)
	}
	if parsed.DefaultBranch == "" {
		return "", fmt.Errorf("repository %s/%s returned empty default branch", owner, repo)
	}
	return parsed.DefaultBranch, nil
}

func (o *GitHubOpener) fetchBranchSHA(ctx context.Context, owner string, repo string, branch string) (string, error) {
	var parsed githubRefResponse
	if err := o.requestJSON(ctx, http.MethodGet, fmt.Sprintf("%s/repos/%s/%s/git/ref/heads/%s", githubAPIBase, owner, repo, url.PathEscape(branch)), nil, &parsed); err != nil {
		return "", fmt.Errorf("fetch branch sha for %q: %w", branch, err)
	}
	if parsed.Object.SHA == "" {
		return "", fmt.Errorf("branch %q has empty sha", branch)
	}
	return parsed.Object.SHA, nil
}

func (o *GitHubOpener) createOrUpdateBranch(ctx context.Context, owner string, repo string, branch string, sha string) error {
	payload := map[string]any{"ref": "refs/heads/" + branch, "sha": sha}
	err := o.requestJSON(ctx, http.MethodPost, fmt.Sprintf("%s/repos/%s/%s/git/refs", githubAPIBase, owner, repo), payload, nil)
	if err == nil {
		return nil
	}

	if !strings.Contains(err.Error(), "status=422") {
		return fmt.Errorf("create branch %q: %w", branch, err)
	}

	updatePayload := map[string]any{"sha": sha, "force": true}
	if updateErr := o.requestJSON(ctx, http.MethodPatch, fmt.Sprintf("%s/repos/%s/%s/git/refs/heads/%s", githubAPIBase, owner, repo, url.PathEscape(branch)), updatePayload, nil); updateErr != nil {
		return fmt.Errorf("update branch %q: %w", branch, updateErr)
	}
	return nil
}

func (o *GitHubOpener) fetchContentSHA(ctx context.Context, owner string, repo string, path string, branch string) (string, error) {
	requestURL := fmt.Sprintf("%s/repos/%s/%s/contents/%s?ref=%s", githubAPIBase, owner, repo, path, url.QueryEscape(branch))
	var parsed githubContentResponse
	if err := o.requestJSON(ctx, http.MethodGet, requestURL, nil, &parsed); err != nil {
		if strings.Contains(err.Error(), "status=404") {
			return "", nil
		}
		return "", fmt.Errorf("fetch content sha: %w", err)
	}
	return parsed.SHA, nil
}

func (o *GitHubOpener) putContent(ctx context.Context, owner string, repo string, path string, branch string, sha string, content string) error {
	payload := map[string]any{
		"message": o.commitMessage,
		"content": base64.StdEncoding.EncodeToString([]byte(content)),
		"branch":  branch,
	}
	if sha != "" {
		payload["sha"] = sha
	}

	if err := o.requestJSON(ctx, http.MethodPut, fmt.Sprintf("%s/repos/%s/%s/contents/%s", githubAPIBase, owner, repo, path), payload, nil); err != nil {
		return fmt.Errorf("put sitemap content: %w", err)
	}
	return nil
}

func (o *GitHubOpener) createPullRequest(ctx context.Context, owner string, repo string, baseBranch string, headBranch string, plan pr.PullRequestPlan) (string, error) {
	body := strings.TrimSpace(plan.Body)
	if body == "" {
		body = "Automated sitemap_404 cleanup from one-shot scan workflow."
	}

	payload := map[string]any{
		"title": plan.Title,
		"head":  headBranch,
		"base":  baseBranch,
		"body":  body,
	}

	var parsed githubPRResponse
	if err := o.requestJSON(ctx, http.MethodPost, fmt.Sprintf("%s/repos/%s/%s/pulls", githubAPIBase, owner, repo), payload, &parsed); err != nil {
		return "", fmt.Errorf("create pull request: %w", err)
	}
	if parsed.HTMLURL == "" {
		return "", fmt.Errorf("create pull request returned empty html_url")
	}
	return parsed.HTMLURL, nil
}

func (o *GitHubOpener) requestJSON(ctx context.Context, method string, requestURL string, payload any, out any) error {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal payload: %w", err)
		}
		body = bytes.NewReader(encoded)
	}

	request, err := http.NewRequestWithContext(ctx, method, requestURL, body)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	request.Header.Set("Authorization", "Bearer "+o.token)
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := o.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		responseBody, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("status=%d body=%q", response.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	if out == nil {
		io.Copy(io.Discard, response.Body)
		return nil
	}

	if err := json.NewDecoder(response.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}
