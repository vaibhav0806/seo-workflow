package competitor

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/nodeops/seo-workflow/internal/config"
)

var githubInventoryAPIBase = "https://api.github.com"

type githubInventoryPullRequest struct {
	Number int `json:"number"`
	Head   struct {
		SHA string `json:"sha"`
	} `json:"head"`
}

type githubInventoryPullRequestFile struct {
	Filename string `json:"filename"`
	Status   string `json:"status"`
}

type githubInventoryContent struct {
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
}

type ContentDecision string

const (
	ContentDecisionCreate      ContentDecision = "create"
	ContentDecisionRefresh     ContentDecision = "refresh"
	ContentDecisionConsolidate ContentDecision = "consolidate"
	ContentDecisionSkip        ContentDecision = "skip"
)

type ExistingContent struct {
	Path              string   `json:"path"`
	Route             string   `json:"route"`
	Title             string   `json:"title"`
	Description       string   `json:"description,omitempty"`
	Author            string   `json:"author,omitempty"`
	PrimaryKeyword    string   `json:"primaryKeyword,omitempty"`
	SecondaryKeywords []string `json:"secondaryKeywords,omitempty"`
	UniquePerspective string   `json:"uniquePerspective,omitempty"`
	TargetFunnel      string   `json:"targetFunnel,omitempty"`
	Headings          []string `json:"headings,omitempty"`
	InternalLinks     []string `json:"internalLinks,omitempty"`
	BodyMarkdown      string   `json:"-"`
}

type ContentInventory struct {
	Pages []ExistingContent `json:"pages"`
}

type CannibalizationFinding struct {
	Keyword string   `json:"keyword"`
	Routes  []string `json:"routes"`
}

type InventoryReport struct {
	ExistingCount   int                      `json:"existingCount"`
	Cannibalization []CannibalizationFinding `json:"cannibalization,omitempty"`
}

type contentFrontmatter struct {
	Title               string   `yaml:"title"`
	Slug                string   `yaml:"slug"`
	Description         string   `yaml:"description"`
	Author              string   `yaml:"author"`
	PrimaryKeyword      string   `yaml:"primary_keyword"`
	SecondaryKeywords   []string `yaml:"secondary_keywords"`
	UniquePerspective   string   `yaml:"unique_perspective"`
	TargetFunnel        string   `yaml:"target_funnel"`
	InternalLinkTargets []any    `yaml:"internal_link_targets"`
}

var markdownHeadingPattern = regexp.MustCompile(`(?m)^#{1,6}\s+(.+?)\s*$`)
var markdownLinkPattern = regexp.MustCompile(`\[[^\]]+\]\((/[^)\s]+)\)`)

func LoadContentInventoryFromDir(root string) (ContentInventory, error) {
	blogsDir := filepath.Join(strings.TrimSpace(root), "blogs")
	entries, err := os.ReadDir(blogsDir)
	if err != nil {
		return ContentInventory{}, fmt.Errorf("read content inventory %q: %w", blogsDir, err)
	}
	pages := make([]ExistingContent, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
			continue
		}
		path := filepath.Join(blogsDir, entry.Name())
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return ContentInventory{}, fmt.Errorf("read content file %q: %w", path, readErr)
		}
		page, parseErr := parseExistingContent("blogs/"+entry.Name(), string(raw))
		if parseErr != nil {
			return ContentInventory{}, fmt.Errorf("parse content file %q: %w", path, parseErr)
		}
		pages = append(pages, page)
	}
	sort.Slice(pages, func(i, j int) bool { return pages[i].Route < pages[j].Route })
	return ContentInventory{Pages: pages}, nil
}

func LoadContentInventory(ctx context.Context, localPath string, token string, repo string, branch string) (ContentInventory, error) {
	if strings.TrimSpace(localPath) != "" {
		return LoadContentInventoryFromDir(localPath)
	}
	parts := strings.Split(strings.Trim(strings.TrimSpace(repo), "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return ContentInventory{}, fmt.Errorf("invalid content repo %q", repo)
	}
	if strings.TrimSpace(token) == "" {
		return ContentInventory{}, fmt.Errorf("github token is required to load remote content inventory")
	}
	branch = strings.TrimSpace(branch)
	if branch == "" {
		branch = "main"
	}
	endpoint := fmt.Sprintf("%s/repos/%s/%s/tarball/%s", githubInventoryAPIBase, url.PathEscape(parts[0]), url.PathEscape(parts[1]), url.PathEscape(branch))
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return ContentInventory{}, fmt.Errorf("build content inventory request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	request.Header.Set("Accept", "application/vnd.github+json")
	client := &http.Client{Timeout: 60 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return ContentInventory{}, fmt.Errorf("fetch content inventory: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return ContentInventory{}, fmt.Errorf("fetch content inventory status=%d body=%q", response.StatusCode, strings.TrimSpace(string(body)))
	}
	return loadContentInventoryArchive(response.Body)
}

func LoadContentInventoryWithOpenPullRequests(ctx context.Context, localPath string, token string, repo string, branch string) (ContentInventory, error) {
	inventory, err := LoadContentInventory(ctx, localPath, token, repo, branch)
	if err != nil {
		return ContentInventory{}, err
	}
	if strings.TrimSpace(token) == "" {
		return inventory, nil
	}
	owner, repoName, err := splitInventoryRepo(repo)
	if err != nil {
		return inventory, err
	}
	branch = strings.TrimSpace(branch)
	if branch == "" {
		branch = "main"
	}

	var pulls []githubInventoryPullRequest
	pullsURL := fmt.Sprintf("%s/repos/%s/%s/pulls?state=open&base=%s&per_page=100", githubInventoryAPIBase, url.PathEscape(owner), url.PathEscape(repoName), url.QueryEscape(branch))
	if err := requestGitHubInventoryJSON(ctx, token, pullsURL, &pulls); err != nil {
		return inventory, fmt.Errorf("fetch open content pull requests: %w", err)
	}
	pagesByPath := make(map[string]ExistingContent, len(inventory.Pages))
	for _, page := range inventory.Pages {
		pagesByPath[page.Path] = page
	}
	for _, pull := range pulls {
		if pull.Number == 0 || strings.TrimSpace(pull.Head.SHA) == "" {
			continue
		}
		var files []githubInventoryPullRequestFile
		filesURL := fmt.Sprintf("%s/repos/%s/%s/pulls/%d/files?per_page=100", githubInventoryAPIBase, url.PathEscape(owner), url.PathEscape(repoName), pull.Number)
		if err := requestGitHubInventoryJSON(ctx, token, filesURL, &files); err != nil {
			return inventory, fmt.Errorf("fetch content pull request #%d files: %w", pull.Number, err)
		}
		for _, file := range files {
			path := filepath.ToSlash(strings.TrimSpace(file.Filename))
			if !strings.HasPrefix(path, "blogs/") || !strings.HasSuffix(strings.ToLower(path), ".md") {
				continue
			}
			if strings.EqualFold(strings.TrimSpace(file.Status), "removed") {
				delete(pagesByPath, path)
				continue
			}
			contentURL := fmt.Sprintf("%s/repos/%s/%s/contents/%s?ref=%s", githubInventoryAPIBase, url.PathEscape(owner), url.PathEscape(repoName), pathEscapeSegments(path), url.QueryEscape(pull.Head.SHA))
			var content githubInventoryContent
			if err := requestGitHubInventoryJSON(ctx, token, contentURL, &content); err != nil {
				return inventory, fmt.Errorf("fetch content pull request #%d file %q: %w", pull.Number, path, err)
			}
			raw, err := decodeGitHubInventoryContent(content)
			if err != nil {
				return inventory, fmt.Errorf("decode content pull request #%d file %q: %w", pull.Number, path, err)
			}
			page, err := parseExistingContent(path, raw)
			if err != nil {
				return inventory, fmt.Errorf("parse content pull request #%d file %q: %w", pull.Number, path, err)
			}
			pagesByPath[path] = page
		}
	}
	pages := make([]ExistingContent, 0, len(pagesByPath))
	for _, page := range pagesByPath {
		pages = append(pages, page)
	}
	sort.Slice(pages, func(i, j int) bool { return pages[i].Route < pages[j].Route })
	return ContentInventory{Pages: pages}, nil
}

func splitInventoryRepo(repo string) (string, string, error) {
	parts := strings.Split(strings.Trim(strings.TrimSpace(repo), "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid content repo %q", repo)
	}
	return parts[0], parts[1], nil
}

func requestGitHubInventoryJSON(ctx context.Context, token string, requestURL string, out any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	request.Header.Set("Accept", "application/vnd.github+json")
	response, err := (&http.Client{Timeout: 60 * time.Second}).Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("github status=%d body=%q", response.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 8<<20)).Decode(out); err != nil {
		return err
	}
	return nil
}

func decodeGitHubInventoryContent(content githubInventoryContent) (string, error) {
	if !strings.EqualFold(strings.TrimSpace(content.Encoding), "base64") {
		return "", fmt.Errorf("unsupported github content encoding %q", content.Encoding)
	}
	encoded := strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == ' ' || r == '\t' {
			return -1
		}
		return r
	}, content.Content)
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func pathEscapeSegments(path string) string {
	parts := strings.Split(filepath.ToSlash(path), "/")
	for idx := range parts {
		parts[idx] = url.PathEscape(parts[idx])
	}
	return strings.Join(parts, "/")
}

func loadContentInventoryArchive(reader io.Reader) (ContentInventory, error) {
	gzipReader, err := gzip.NewReader(reader)
	if err != nil {
		return ContentInventory{}, fmt.Errorf("open content inventory archive: %w", err)
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	pages := make([]ExistingContent, 0)
	for {
		header, nextErr := tarReader.Next()
		if nextErr == io.EOF {
			break
		}
		if nextErr != nil {
			return ContentInventory{}, fmt.Errorf("read content inventory archive: %w", nextErr)
		}
		name := filepath.ToSlash(header.Name)
		marker := "/blogs/"
		idx := strings.Index(name, marker)
		if header.Typeflag != tar.TypeReg || idx < 0 || !strings.HasSuffix(strings.ToLower(name), ".md") {
			continue
		}
		raw, readErr := io.ReadAll(io.LimitReader(tarReader, 4<<20))
		if readErr != nil {
			return ContentInventory{}, fmt.Errorf("read archived content %q: %w", name, readErr)
		}
		path := "blogs/" + name[idx+len(marker):]
		page, parseErr := parseExistingContent(path, string(raw))
		if parseErr != nil {
			return ContentInventory{}, fmt.Errorf("parse archived content %q: %w", name, parseErr)
		}
		pages = append(pages, page)
	}
	sort.Slice(pages, func(i, j int) bool { return pages[i].Route < pages[j].Route })
	return ContentInventory{Pages: pages}, nil
}

func parseExistingContent(path string, raw string) (ExistingContent, error) {
	frontmatter, body, err := splitFrontmatter(raw)
	if err != nil {
		return ExistingContent{}, err
	}
	var metadata contentFrontmatter
	if err := yaml.Unmarshal([]byte(frontmatter), &metadata); err != nil {
		return ExistingContent{}, fmt.Errorf("decode frontmatter: %w", err)
	}
	slug := strings.Trim(strings.TrimSpace(metadata.Slug), "/")
	if slug == "" {
		slug = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	links := contentLinkTargets(metadata.InternalLinkTargets)
	for _, match := range markdownLinkPattern.FindAllStringSubmatch(body, -1) {
		if len(match) > 1 {
			links = append(links, match[1])
		}
	}
	headings := make([]string, 0)
	for _, match := range markdownHeadingPattern.FindAllStringSubmatch(body, -1) {
		if len(match) > 1 {
			headings = append(headings, strings.TrimSpace(match[1]))
		}
	}
	return ExistingContent{
		Path:              path,
		Route:             "/blogs/" + slug,
		Title:             strings.TrimSpace(metadata.Title),
		Description:       strings.TrimSpace(metadata.Description),
		Author:            strings.TrimSpace(metadata.Author),
		PrimaryKeyword:    strings.TrimSpace(metadata.PrimaryKeyword),
		SecondaryKeywords: uniqueNonEmpty(metadata.SecondaryKeywords),
		UniquePerspective: strings.TrimSpace(metadata.UniquePerspective),
		TargetFunnel:      strings.TrimSpace(metadata.TargetFunnel),
		Headings:          uniqueNonEmpty(headings),
		InternalLinks:     uniqueNonEmpty(links),
		BodyMarkdown:      strings.TrimSpace(body),
	}, nil
}

func contentLinkTargets(values []any) []string {
	links := make([]string, 0, len(values))
	for _, value := range values {
		switch typed := value.(type) {
		case string:
			links = append(links, typed)
		case map[string]any:
			if rawURL, ok := typed["url"].(string); ok {
				links = append(links, rawURL)
			}
		}
	}
	return links
}

func splitFrontmatter(raw string) (string, string, error) {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	if !strings.HasPrefix(normalized, "---\n") {
		return "", "", fmt.Errorf("missing opening frontmatter delimiter")
	}
	end := strings.Index(normalized[4:], "\n---\n")
	if end < 0 {
		return "", "", fmt.Errorf("missing closing frontmatter delimiter")
	}
	end += 4
	return normalized[4:end], normalized[end+5:], nil
}

func DecideContentPlan(plan []ContentRecommendation, inventory ContentInventory) ([]ContentRecommendation, InventoryReport) {
	out := append([]ContentRecommendation(nil), plan...)
	groups := inventoryKeywordGroups(inventory)
	report := InventoryReport{ExistingCount: len(inventory.Pages)}
	for keyword, pages := range groups {
		if keyword == "" || len(pages) < 2 {
			continue
		}
		routes := make([]string, 0, len(pages))
		for _, page := range pages {
			routes = append(routes, page.Route)
		}
		sort.Strings(routes)
		report.Cannibalization = append(report.Cannibalization, CannibalizationFinding{Keyword: pages[0].PrimaryKeyword, Routes: routes})
	}
	sort.Slice(report.Cannibalization, func(i, j int) bool { return report.Cannibalization[i].Keyword < report.Cannibalization[j].Keyword })

	for idx := range out {
		matches := matchingInventoryPages(out[idx], inventory.Pages)
		switch len(matches) {
		case 0:
			out[idx].Decision = ContentDecisionCreate
			out[idx].DecisionReason = "No existing CreateOS blog owns this keyword or intent."
		case 1:
			out[idx].Decision = ContentDecisionRefresh
			out[idx].ExistingRoute = matches[0].Route
			out[idx].DecisionReason = "An existing CreateOS blog already owns this keyword or intent."
			out[idx].Draft = nil
		default:
			out[idx].Decision = ContentDecisionConsolidate
			out[idx].ExistingRoute = matches[0].Route
			out[idx].DecisionReason = fmt.Sprintf("%d existing CreateOS blogs overlap this keyword or intent.", len(matches))
			out[idx].Draft = nil
		}
	}
	return out, report
}

func applyConfiguredInventory(ctx context.Context, cfg *config.Config, plan []ContentRecommendation) ([]ContentRecommendation, InventoryReport, []string) {
	if cfg == nil {
		return markPlanForCreation(plan), InventoryReport{}, []string{"content inventory skipped: config is nil"}
	}
	if strings.TrimSpace(cfg.ContentInventoryPath) == "" && strings.TrimSpace(cfg.GitHubToken) == "" {
		return markPlanForCreation(plan), InventoryReport{}, []string{"content inventory skipped: set CONTENT_INVENTORY_PATH or GITHUB_TOKEN"}
	}
	inventory, err := LoadContentInventoryWithOpenPullRequests(ctx, cfg.ContentInventoryPath, cfg.GitHubToken, cfg.ContentRepo, cfg.ContentBaseBranch)
	if err != nil {
		if len(inventory.Pages) == 0 {
			return markPlanForCreation(plan), InventoryReport{}, []string{fmt.Sprintf("content inventory skipped: %v", err)}
		}
		decided, report := DecideContentPlan(plan, inventory)
		return decided, report, []string{fmt.Sprintf("open pull request inventory skipped: %v", err)}
	}
	decided, report := DecideContentPlan(plan, inventory)
	return decided, report, nil
}

func markPlanForCreation(plan []ContentRecommendation) []ContentRecommendation {
	out := append([]ContentRecommendation(nil), plan...)
	for idx := range out {
		if out[idx].Decision == "" {
			out[idx].Decision = ContentDecisionCreate
			out[idx].DecisionReason = "Content inventory was unavailable; human duplicate review is required."
		}
	}
	return out
}

func inventoryKeywordGroups(inventory ContentInventory) map[string][]ExistingContent {
	groups := map[string][]ExistingContent{}
	for _, page := range inventory.Pages {
		keyword := normalizeKeyword(page.PrimaryKeyword)
		if keyword == "" {
			continue
		}
		groups[keyword] = append(groups[keyword], page)
	}
	return groups
}

func matchingInventoryPages(recommendation ContentRecommendation, pages []ExistingContent) []ExistingContent {
	targets := []string{recommendation.PrimaryKeyword, recommendation.SuggestedTitle, strings.TrimPrefix(recommendation.SuggestedSlug, "/blogs/")}
	matches := make([]ExistingContent, 0)
	for _, page := range pages {
		candidates := []string{page.PrimaryKeyword, page.Title, strings.TrimPrefix(page.Route, "/blogs/")}
		candidates = append(candidates, page.SecondaryKeywords...)
		if keywordSetsOverlap(targets, candidates) {
			matches = append(matches, page)
		}
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].Route < matches[j].Route })
	return matches
}

func keywordSetsOverlap(targets []string, candidates []string) bool {
	for _, target := range targets {
		targetNormalized := normalizeKeyword(target)
		if targetNormalized == "" {
			continue
		}
		for _, candidate := range candidates {
			candidateNormalized := normalizeKeyword(candidate)
			if candidateNormalized == "" {
				continue
			}
			if targetNormalized == candidateNormalized || tokenJaccard(targetNormalized, candidateNormalized) >= 0.8 {
				return true
			}
			targetTopic := canonicalTopicKeyword(targetNormalized)
			candidateTopic := canonicalTopicKeyword(candidateNormalized)
			if len(strings.Fields(targetTopic)) >= 2 && len(strings.Fields(candidateTopic)) >= 2 &&
				(targetTopic == candidateTopic || tokenJaccard(targetTopic, candidateTopic) >= 0.8) {
				return true
			}
		}
	}
	return false
}

var contentIntentModifiers = map[string]struct{}{
	"architecture":   {},
	"deployment":     {},
	"framework":      {},
	"frameworks":     {},
	"guide":          {},
	"infrastructure": {},
	"operations":     {},
	"production":     {},
	"strategy":       {},
	"workflow":       {},
	"workflows":      {},
}

func canonicalTopicKeyword(value string) string {
	tokens := filteredTokens(value)
	out := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if _, modifier := contentIntentModifiers[token]; modifier {
			continue
		}
		out = append(out, token)
	}
	return strings.Join(out, " ")
}

func normalizeKeyword(value string) string {
	return strings.Join(filteredTokens(strings.ToLower(value)), " ")
}

func tokenJaccard(left string, right string) float64 {
	leftSet := tokenSet(filteredTokens(left))
	rightSet := tokenSet(filteredTokens(right))
	if len(leftSet) == 0 || len(rightSet) == 0 {
		return 0
	}
	intersection := 0
	union := map[string]struct{}{}
	for token := range leftSet {
		union[token] = struct{}{}
		if _, exists := rightSet[token]; exists {
			intersection++
		}
	}
	for token := range rightSet {
		union[token] = struct{}{}
	}
	return float64(intersection) / float64(len(union))
}

func uniqueNonEmpty(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
