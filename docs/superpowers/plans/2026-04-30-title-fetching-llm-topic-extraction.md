# Title Fetching and LLM Topic Extraction Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the competitor workflow use real page titles and LLM-extracted themes as the primary source of competitor opportunities.

**Architecture:** Keep sitemap discovery as the URL source, then enrich recent URLs with fetched HTML titles before opportunity generation. Add a separate LLM topic extraction pass that batches enriched pages by competitor, asks OpenRouter/Kimi for 5-8 concrete themes, compares those themes against CreateOS themes, and emits the primary opportunities. Keep the current slug analyzer as a fallback/debug signal.

**Tech Stack:** Go 1.25, standard `net/http` + `html` parsing via tokenizer from `golang.org/x/net/html`, existing OpenRouter client pattern, existing `internal/competitor` report types.

---

## File Structure

- Modify `internal/competitor/run.go`: add `Title` to `SitemapEntry`, orchestrate title enrichment before opportunity generation.
- Create `internal/competitor/title_fetcher.go`: fetch page HTML and extract `<title>` / `og:title` with timeout and byte limits.
- Create `internal/competitor/title_fetcher_test.go`: unit tests for title extraction and fetch failure behavior with stub transport.
- Modify `internal/competitor/openrouter.go`: add topic extraction request/response types and a focused `extractTopicsWithOpenRouter` function.
- Modify `internal/competitor/analyzer.go`: add theme comparison and opportunity generation from extracted topics; keep existing slug-derived opportunities as fallback.
- Modify `internal/competitor/analyzer_test.go`: assert LLM topic opportunities beat slug-only noise and compare against our themes.
- Modify `cmd/worker/report.go`: log extracted topic themes and their evidence titles/URLs.
- Modify `docs/competitor-oneshot-workflow.md`: document that title + LLM topics are primary, slug analyzer is fallback.
- Modify `.env.example`: add title/LLM tuning envs if config knobs are added.
- Modify `internal/config/config.go` and `internal/config/config_test.go`: only if we add env knobs such as `COMPETITOR_TITLE_LIMIT` or `COMPETITOR_TITLE_TIMEOUT_SEC`.

---

### Task 1: Add Page Titles To Snapshots

**Files:**
- Modify: `internal/competitor/run.go`
- Test: `internal/competitor/analyzer_test.go`

- [ ] **Step 1: Add `Title` field to `SitemapEntry`**

Change the struct in `internal/competitor/run.go`:

```go
type SitemapEntry struct {
	URL       string   `json:"url"`
	Title     string   `json:"title,omitempty"`
	LastMod   *string  `json:"lastMod,omitempty"`
	ThemeTags []string `json:"themeTags,omitempty"`
}
```

- [ ] **Step 2: Run compile check**

Run:

```bash
GOCACHE=/tmp/go-build-cache GOMODCACHE=/tmp/go-mod-cache go test ./internal/competitor -count=1
```

Expected: PASS. This is only a schema expansion.

- [ ] **Step 3: Commit**

```bash
git add internal/competitor/run.go
git commit -m "feat: add titles to competitor sitemap entries"
```

---

### Task 2: Implement Title Fetcher

**Files:**
- Create: `internal/competitor/title_fetcher.go`
- Create: `internal/competitor/title_fetcher_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/competitor/title_fetcher_test.go`:

```go
package competitor

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractTitlePrefersOGTitle(t *testing.T) {
	html := `<html><head><title>Fallback Title</title><meta property="og:title" content="Coding Agents · AI Gateway"></head></html>`

	title, err := extractPageTitle(strings.NewReader(html))

	require.NoError(t, err)
	require.Equal(t, "Coding Agents · AI Gateway", title)
}

func TestExtractTitleFallsBackToTitleTag(t *testing.T) {
	html := `<html><head><title>Claude Code · AI Gateway</title></head></html>`

	title, err := extractPageTitle(strings.NewReader(html))

	require.NoError(t, err)
	require.Equal(t, "Claude Code · AI Gateway", title)
}

func TestTitleFetcherReturnsEmptyOnHTTPError(t *testing.T) {
	fetcher := &TitleFetcher{
		httpClient: &http.Client{Transport: titleStubTransport{
			status: http.StatusTooManyRequests,
			body:   "rate limited",
		}},
	}

	title, err := fetcher.FetchTitle(context.Background(), "https://vercel.com/docs/ai-gateway/coding-agents")

	require.NoError(t, err)
	require.Empty(t, title)
}

type titleStubTransport struct {
	status int
	body   string
}

func (t titleStubTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.String() == "" {
		return nil, fmt.Errorf("empty url")
	}
	return &http.Response{
		StatusCode: t.status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(t.body)),
		Request:    req,
	}, nil
}
```

- [ ] **Step 2: Verify tests fail**

Run:

```bash
GOCACHE=/tmp/go-build-cache GOMODCACHE=/tmp/go-mod-cache go test ./internal/competitor -run 'TestExtractTitle|TestTitleFetcher' -count=1
```

Expected: FAIL because `TitleFetcher` and `extractPageTitle` do not exist.

- [ ] **Step 3: Implement title fetcher**

Create `internal/competitor/title_fetcher.go`:

```go
package competitor

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const maxTitleHTMLBytes = 1 << 20

type TitleFetcher struct {
	httpClient *http.Client
}

func NewTitleFetcher(timeoutSecs int) *TitleFetcher {
	return &TitleFetcher{httpClient: &http.Client{Timeout: time.Duration(timeoutSecs) * time.Second}}
}

func (f *TitleFetcher) FetchTitle(ctx context.Context, pageURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSpace(pageURL), nil)
	if err != nil {
		return "", fmt.Errorf("build title request: %w", err)
	}
	req.Header.Set("User-Agent", "seo-workflow/0.1")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch title: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", nil
	}

	title, err := extractPageTitle(io.LimitReader(resp.Body, maxTitleHTMLBytes))
	if err != nil {
		return "", nil
	}
	return strings.TrimSpace(title), nil
}

func extractPageTitle(r io.Reader) (string, error) {
	tokenizer := html.NewTokenizer(r)
	title := ""
	for {
		tt := tokenizer.Next()
		switch tt {
		case html.ErrorToken:
			if tokenizer.Err() == io.EOF {
				return strings.TrimSpace(title), nil
			}
			return "", tokenizer.Err()
		case html.StartTagToken, html.SelfClosingTagToken:
			token := tokenizer.Token()
			if token.Data == "meta" {
				if isOGTitle(token) {
					if content := attrValue(token, "content"); content != "" {
						return strings.TrimSpace(content), nil
					}
				}
				continue
			}
			if token.Data == "title" {
				if tokenizer.Next() == html.TextToken {
					title = tokenizer.Token().Data
				}
			}
		}
	}
}

func isOGTitle(token html.Token) bool {
	for _, attr := range token.Attr {
		if strings.EqualFold(attr.Key, "property") && strings.EqualFold(attr.Val, "og:title") {
			return true
		}
	}
	return false
}

func attrValue(token html.Token, key string) string {
	for _, attr := range token.Attr {
		if strings.EqualFold(attr.Key, key) {
			return attr.Val
		}
	}
	return ""
}
```

- [ ] **Step 4: Add dependency**

Run:

```bash
GOCACHE=/tmp/go-build-cache GOMODCACHE=/tmp/go-mod-cache go get golang.org/x/net/html
GOCACHE=/tmp/go-build-cache GOMODCACHE=/tmp/go-mod-cache go mod tidy
```

Expected: `go.mod` and `go.sum` include `golang.org/x/net`.

- [ ] **Step 5: Verify tests pass**

Run:

```bash
GOCACHE=/tmp/go-build-cache GOMODCACHE=/tmp/go-mod-cache go test ./internal/competitor -run 'TestExtractTitle|TestTitleFetcher' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/competitor/title_fetcher.go internal/competitor/title_fetcher_test.go
git commit -m "feat: fetch titles for competitor pages"
```

---

### Task 3: Enrich Recent URLs With Titles

**Files:**
- Modify: `internal/competitor/run.go`
- Test: `internal/competitor/analyzer_test.go`

- [ ] **Step 1: Add title enrichment helper**

Add below `buildSnapshot` in `internal/competitor/run.go`:

```go
func enrichSnapshotTitles(ctx context.Context, fetcher *TitleFetcher, snapshot SiteSnapshot, limit int) (SiteSnapshot, []string) {
	if fetcher == nil || limit <= 0 {
		return snapshot, nil
	}

	warnings := make([]string, 0)
	for idx := range snapshot.RecentURLs {
		if idx >= limit {
			break
		}
		title, err := fetcher.FetchTitle(ctx, snapshot.RecentURLs[idx].URL)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("title fetch failed for %s: %v", snapshot.RecentURLs[idx].URL, err))
			continue
		}
		snapshot.RecentURLs[idx].Title = title
	}
	return snapshot, warnings
}
```

- [ ] **Step 2: Wire title enrichment in `Run`**

In `Run`, after `ourSnapshot := buildSnapshot(...)`, add:

```go
titleFetcher := NewTitleFetcher(cfg.HTTPTimeoutSecs)
ourSnapshot, titleWarnings := enrichSnapshotTitles(ctx, titleFetcher, ourSnapshot, 40)
warnings = append(warnings, titleWarnings...)
```

In the competitor loop, after `buildSnapshot`, replace the append with:

```go
snapshot := buildSnapshot(target.Name, target.SitemapURL, entries, windowStart)
snapshot, titleWarnings := enrichSnapshotTitles(ctx, titleFetcher, snapshot, 40)
warnings = append(warnings, titleWarnings...)
competitorSnapshots = append(competitorSnapshots, snapshot)
```

- [ ] **Step 3: Run competitor tests**

Run:

```bash
GOCACHE=/tmp/go-build-cache GOMODCACHE=/tmp/go-mod-cache go test ./internal/competitor -count=1
```

Expected: PASS. Existing tests use direct snapshots and are unaffected.

- [ ] **Step 4: Commit**

```bash
git add internal/competitor/run.go
git commit -m "feat: enrich competitor snapshots with page titles"
```

---

### Task 4: Add LLM Topic Extraction Types

**Files:**
- Modify: `internal/competitor/run.go`
- Modify: `internal/competitor/openrouter.go`

- [ ] **Step 1: Add topic summary types**

Add to `internal/competitor/run.go`:

```go
type TopicSummary struct {
	Competitor    string   `json:"competitor"`
	Name          string   `json:"name"`
	PageCount     int      `json:"pageCount"`
	RepresentativeTitles []string `json:"representativeTitles"`
	EvidenceURLs   []string `json:"evidenceUrls"`
	WhyItMatters   string   `json:"whyItMatters"`
}
```

Add to `Summary`:

```go
ExtractedTopics []TopicSummary `json:"extractedTopics,omitempty"`
```

- [ ] **Step 2: Add LLM response shape**

Add to `internal/competitor/openrouter.go`:

```go
type llmTopicOutput struct {
	Topics []TopicSummary `json:"topics"`
}
```

- [ ] **Step 3: Compile check**

Run:

```bash
GOCACHE=/tmp/go-build-cache GOMODCACHE=/tmp/go-mod-cache go test ./internal/competitor -count=1
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/competitor/run.go internal/competitor/openrouter.go
git commit -m "feat: add competitor topic summary model"
```

---

### Task 5: Implement OpenRouter Topic Extractor

**Files:**
- Modify: `internal/competitor/openrouter.go`
- Test: `internal/competitor/openrouter_test.go`

- [ ] **Step 1: Write test for JSON extraction**

Create or extend `internal/competitor/openrouter_test.go`:

```go
package competitor

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseTopicOutput(t *testing.T) {
	raw := `{"topics":[{"competitor":"vercel","name":"AI gateway integrations with coding agents","pageCount":8,"representativeTitles":["Coding Agents · AI Gateway"],"evidenceUrls":["https://vercel.com/docs/ai-gateway/coding-agents"],"whyItMatters":"Vercel is positioning AI Gateway as infrastructure for agent builders."}]}`

	var out llmTopicOutput
	err := json.Unmarshal([]byte(raw), &out)

	require.NoError(t, err)
	require.Len(t, out.Topics, 1)
	require.Equal(t, "AI gateway integrations with coding agents", out.Topics[0].Name)
}
```

- [ ] **Step 2: Implement `extractTopicsWithOpenRouter`**

Add to `internal/competitor/openrouter.go`:

```go
func extractTopicsWithOpenRouter(
	ctx context.Context,
	apiKey string,
	model string,
	competitors []SiteSnapshot,
) ([]TopicSummary, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, nil
	}
	if strings.TrimSpace(model) == "" {
		model = "moonshotai/kimi-k2"
	}

	input := buildTopicPromptInput(competitors, 40)
	inputBytes, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshal topic prompt input: %w", err)
	}

	systemPrompt := "You are an SEO strategist. Return only strict JSON with key topics[]."
	userPrompt := "Group these recent competitor pages into 5-8 concrete themes per competitor. Use page titles as primary evidence and URLs as secondary evidence. Ignore locale, FAQ, career, gallery, legal, and generic company pages. Each topic must include competitor, name, pageCount, representativeTitles, evidenceUrls, and whyItMatters. JSON only. Data: " + string(inputBytes)

	requestBody := openRouterRequest{
		Model:       model,
		Temperature: 0.1,
		Messages: []openRouterMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	}
	payload, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("marshal topic request: %w", err)
	}

	client := &http.Client{Timeout: 90 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openRouterEndpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build topic request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute topic request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("openrouter topic status=%d body=%q", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed openRouterResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decode topic response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return nil, fmt.Errorf("openrouter topic response had no choices")
	}

	clean := extractJSONObject(parsed.Choices[0].Message.Content)
	var out llmTopicOutput
	if err := json.Unmarshal([]byte(clean), &out); err != nil {
		return nil, fmt.Errorf("unmarshal topic output: %w", err)
	}
	return normalizeTopicSummaries(out.Topics), nil
}
```

- [ ] **Step 3: Add prompt input helpers**

Add to `internal/competitor/openrouter.go`:

```go
type topicPromptPage struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}

type topicPromptCompetitor struct {
	Name  string            `json:"name"`
	Pages []topicPromptPage `json:"pages"`
}

func buildTopicPromptInput(competitors []SiteSnapshot, limit int) []topicPromptCompetitor {
	result := make([]topicPromptCompetitor, 0, len(competitors))
	for _, competitor := range competitors {
		pages := make([]topicPromptPage, 0)
		for _, entry := range competitor.RecentURLs {
			if len(pages) >= limit {
				break
			}
			if strings.TrimSpace(entry.Title) == "" {
				continue
			}
			pages = append(pages, topicPromptPage{URL: entry.URL, Title: entry.Title})
		}
		if len(pages) == 0 {
			continue
		}
		result = append(result, topicPromptCompetitor{Name: competitor.Name, Pages: pages})
	}
	return result
}

func normalizeTopicSummaries(topics []TopicSummary) []TopicSummary {
	result := make([]TopicSummary, 0, len(topics))
	for _, topic := range topics {
		topic.Name = strings.TrimSpace(topic.Name)
		topic.Competitor = strings.TrimSpace(topic.Competitor)
		topic.WhyItMatters = strings.TrimSpace(topic.WhyItMatters)
		if topic.Name == "" || topic.Competitor == "" {
			continue
		}
		if len(topic.RepresentativeTitles) > 5 {
			topic.RepresentativeTitles = topic.RepresentativeTitles[:5]
		}
		if len(topic.EvidenceURLs) > 5 {
			topic.EvidenceURLs = topic.EvidenceURLs[:5]
		}
		result = append(result, topic)
	}
	return result
}
```

- [ ] **Step 4: Run tests**

Run:

```bash
GOCACHE=/tmp/go-build-cache GOMODCACHE=/tmp/go-mod-cache go test ./internal/competitor -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/competitor/openrouter.go internal/competitor/openrouter_test.go
git commit -m "feat: extract competitor themes with openrouter"
```

---

### Task 6: Generate Opportunities From Extracted Topics

**Files:**
- Modify: `internal/competitor/analyzer.go`
- Test: `internal/competitor/analyzer_test.go`

- [ ] **Step 1: Add topic comparison function**

Add to `internal/competitor/analyzer.go`:

```go
func deriveTopicOpportunities(ours SiteSnapshot, topics []TopicSummary) []Opportunity {
	ourTitleText := strings.ToLower(joinEntryTitlesAndURLs(ours.RecentURLs))
	opportunities := make([]Opportunity, 0)

	for _, topic := range topics {
		if topic.PageCount < 2 {
			continue
		}
		topicName := strings.ToLower(topic.Name)
		if themeCoveredByCreateOS(topicName, ourTitleText) {
			continue
		}

		opportunities = append(opportunities, Opportunity{
			Title:           fmt.Sprintf("CreateOS should cover %q", topic.Name),
			WhyItMatters:    topic.WhyItMatters,
			WhatToDo:        fmt.Sprintf("Ship one focused page or article for %s using the competitor evidence as the outline.", topic.Name),
			HowToExecute:    topicExecutionPlan(topic),
			ImpactScore:     clampImpact(55 + min(topic.PageCount, 8)*5),
			Competitor:      topic.Competitor,
			Theme:           phraseTheme(topic.Name),
			OpportunityType: "llm-topic-gap",
			Evidence:        topic.EvidenceURLs,
		})
	}
	return opportunities
}

func joinEntryTitlesAndURLs(entries []SitemapEntry) string {
	parts := make([]string, 0, len(entries)*2)
	for _, entry := range entries {
		parts = append(parts, entry.Title, entry.URL)
	}
	return strings.Join(parts, " ")
}

func themeCoveredByCreateOS(topicName string, ourText string) bool {
	keyTerms := filteredTokens(topicName)
	matches := 0
	for _, term := range keyTerms {
		if _, generic := genericPathTokens[term]; generic {
			continue
		}
		if strings.Contains(ourText, term) {
			matches++
		}
	}
	return matches >= 2
}

func topicExecutionPlan(topic TopicSummary) []string {
	slug := strings.ToLower(strings.ReplaceAll(topic.Name, " ", "-"))
	return []string{
		fmt.Sprintf("Create `/blogs/%s` or a dedicated landing page with this exact theme.", slug),
		"Use the representative competitor titles as H2 sections, but write CreateOS-specific examples.",
		"Link the page from homepage, docs, and relevant case studies, then request indexing.",
	}
}
```

- [ ] **Step 2: Add analyzer test**

Add to `internal/competitor/analyzer_test.go`:

```go
func TestDeriveTopicOpportunitiesSkipsCoveredCreateOSThemes(t *testing.T) {
	ours := SiteSnapshot{
		Name: "createos",
		RecentURLs: []SitemapEntry{
			{URL: "https://createos.sh/blogs/agentic-deployments-ai-agents-createos", Title: "Agentic deployments for AI agents with CreateOS"},
		},
	}
	topics := []TopicSummary{
		{
			Competitor: "vercel",
			Name: "AI gateway integrations with coding agents",
			PageCount: 8,
			RepresentativeTitles: []string{"Coding Agents · AI Gateway"},
			EvidenceURLs: []string{"https://vercel.com/docs/ai-gateway/coding-agents"},
			WhyItMatters: "Vercel is building agent infrastructure intent.",
		},
		{
			Competitor: "lovable",
			Name: "Template gallery for SaaS apps",
			PageCount: 6,
			RepresentativeTitles: []string{"SaaS templates"},
			EvidenceURLs: []string{"https://lovable.dev/templates/apps/saas"},
			WhyItMatters: "Lovable is capturing ready-to-build SaaS intent.",
		},
	}

	opportunities := deriveTopicOpportunities(ours, topics)

	require.Len(t, opportunities, 1)
	require.Equal(t, "CreateOS should cover \"Template gallery for SaaS apps\"", opportunities[0].Title)
	require.Equal(t, "llm-topic-gap", opportunities[0].OpportunityType)
}
```

- [ ] **Step 3: Run tests**

Run:

```bash
GOCACHE=/tmp/go-build-cache GOMODCACHE=/tmp/go-mod-cache go test ./internal/competitor -run TestDeriveTopicOpportunities -count=1
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/competitor/analyzer.go internal/competitor/analyzer_test.go
git commit -m "feat: generate opportunities from extracted topics"
```

---

### Task 7: Wire Extracted Topics Into Run

**Files:**
- Modify: `internal/competitor/run.go`
- Modify: `internal/competitor/openrouter.go`

- [ ] **Step 1: Use title + LLM topics as primary opportunities**

In `Run`, replace the current OpenRouter refine block with:

```go
opportunities := deriveOpportunities(ourSnapshot, competitorSnapshots)
extractedTopics := []TopicSummary{}
if cfg.OpenRouterAPIKey != "" {
	topics, topicErr := extractTopicsWithOpenRouter(ctx, cfg.OpenRouterAPIKey, cfg.OpenRouterModel, competitorSnapshots)
	if topicErr != nil {
		warnings = append(warnings, fmt.Sprintf("openrouter topic extraction skipped: %v", topicErr))
	} else {
		extractedTopics = topics
		topicOpportunities := deriveTopicOpportunities(ourSnapshot, topics)
		if len(topicOpportunities) > 0 {
			opportunities = topicOpportunities
		}
	}
}
```

Add `ExtractedTopics: extractedTopics,` to the returned `Summary`.

- [ ] **Step 2: Run tests**

Run:

```bash
GOCACHE=/tmp/go-build-cache GOMODCACHE=/tmp/go-mod-cache go test ./internal/competitor -count=1
```

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/competitor/run.go
git commit -m "feat: use extracted topics as primary competitor opportunities"
```

---

### Task 8: Improve Logs And Report Output

**Files:**
- Modify: `cmd/worker/report.go`
- Modify: `docs/competitor-oneshot-workflow.md`

- [ ] **Step 1: Log extracted topics**

In `logCompetitorSummary`, after competitor snapshot logs, add:

```go
for idx, topic := range summary.ExtractedTopics {
	log.Printf(
		"topic_%d competitor=%q name=%q pages=%d why=%q evidence=%v",
		idx+1,
		topic.Competitor,
		topic.Name,
		topic.PageCount,
		topic.WhyItMatters,
		topic.EvidenceURLs,
	)
}
```

- [ ] **Step 2: Update docs**

In `docs/competitor-oneshot-workflow.md`, add:

```markdown
When `OPENROUTER_API_KEY` is set, the primary workflow is:

1. Fetch recent sitemap URLs.
2. Fetch page titles for recent URLs.
3. Ask OpenRouter/Kimi to group competitor titles into concrete themes.
4. Compare those themes against CreateOS titles and URLs.
5. Emit topic-gap opportunities with evidence titles and URLs.

The slug analyzer remains a fallback when title fetching or LLM extraction is unavailable.
```

- [ ] **Step 3: Run tests**

Run:

```bash
GOCACHE=/tmp/go-build-cache GOMODCACHE=/tmp/go-mod-cache go test ./... -count=1
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add cmd/worker/report.go docs/competitor-oneshot-workflow.md
git commit -m "docs: explain title-based competitor analysis"
```

---

### Task 9: Smoke Test The Workflow

**Files:**
- No code changes expected.

- [ ] **Step 1: Run competitor smoke**

Run:

```bash
make smoke-competitor
```

Expected:

- Logs include `topic_1`, `topic_2`, etc. when `OPENROUTER_API_KEY` is set.
- Opportunities use `type="llm-topic-gap"` when extraction succeeds.
- Existing slug opportunities appear only when OpenRouter is missing or extraction fails.
- Report JSON includes `recentUrls[].title` and `extractedTopics`.

- [ ] **Step 2: Inspect report**

Run:

```bash
rg -n '"title"|"extractedTopics"|"llm-topic-gap"|topic_' competitor-report.json
```

Expected:

- Page titles are present.
- Extracted topics are present.
- No generated opportunity is based only on path junk like `gallery life` or `about company`.

- [ ] **Step 3: Commit any final docs/test fixes**

```bash
git status --short
git add <changed-files>
git commit -m "chore: polish competitor topic extraction workflow"
```

Only commit if code/docs changed during smoke cleanup.

---

## Acceptance Criteria

- Recent competitor pages include fetched `title` values in `competitor-report.json`.
- OpenRouter/Kimi receives compact batches of title + URL evidence, not full snapshots.
- Extracted topics are visible in JSON and logs.
- Primary opportunities become `llm-topic-gap` when LLM extraction succeeds.
- Existing slug-derived opportunities remain as fallback when title fetch or OpenRouter fails.
- Generic path artifacts like locale FAQ, gallery, author, customer, and company pages do not drive the main report.
- Full test suite passes with:

```bash
GOCACHE=/tmp/go-build-cache GOMODCACHE=/tmp/go-mod-cache go test ./... -count=1
```

---

## Risks And Guardrails

- Title fetching can slow the workflow. Start with `40` pages per site and keep the HTML byte limit at `1 MiB`.
- Some competitors may block HTML requests. Treat missing titles as warnings and keep the slug fallback.
- LLM calls can timeout. Use compact prompt input and a `90s` OpenRouter timeout for topic extraction.
- Do not send `.env` or `competitor-report.json` to git.
- Do not treat LLM output as truth without evidence URLs. Every topic and opportunity must carry URLs.

