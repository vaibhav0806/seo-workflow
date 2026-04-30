# SEO Workflow Phase 1 (Core Scan Loop + `sitemap_404` PR) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a production-grade Phase 1 slice that can run a weekly scan, classify URL indexing state deterministically, and open a `sitemap_404` pull request.

**Architecture:** This implementation uses a Go monorepo layout with clear internal packages for config, GSC discovery, inspection orchestration, classification, and PR generation. The first vertical slice is intentionally narrow: discover URLs, inspect and classify them, and produce one mechanical PR type (`sitemap_404`). External dependencies (GSC and GitHub) are abstracted behind interfaces with test doubles to keep logic deterministic and unit-testable.

**Tech Stack:** Go 1.24, `testing` + `testify`, `pgx` (schema bootstrap only), `golang.org/x/time/rate`, GitHub App API client (interface-first in this phase).

---

## Scope Check

`FEATURE.md` defines multiple independent subsystems (onboarding OAuth, scan loop, PR engine, notifications, dashboard, competitor watcher). This plan intentionally covers only **Phase 1 core scan loop + first PR bucket**. Execute follow-up plans separately for:

1. OAuth onboarding + token lifecycle
2. Dashboard + notification delivery
3. Competitor watcher + suggestion PR flow

## File Structure

- Create: `go.mod` — module definition and dependencies
- Create: `Makefile` — local developer commands (`test`, `lint`, `run-worker`)
- Create: `cmd/worker/main.go` — weekly scan runner entrypoint
- Create: `internal/config/config.go` — environment-backed runtime config
- Create: `internal/config/config_test.go` — config contract tests
- Create: `internal/classifier/classifier.go` — deterministic state-to-bucket mapping
- Create: `internal/classifier/classifier_test.go` — table-driven classifier tests
- Create: `internal/gsc/discovery.go` — sitemap + search analytics merge logic
- Create: `internal/gsc/discovery_test.go` — dedupe and ordering tests
- Create: `internal/ratelimit/property_limiter.go` — per-property QPM limiter
- Create: `internal/ratelimit/property_limiter_test.go` — limiter behavior tests
- Create: `internal/worker/interfaces.go` — scan dependency interfaces
- Create: `internal/worker/inspect_job.go` — URL inspection orchestration
- Create: `internal/worker/inspect_job_test.go` — orchestration unit tests
- Create: `internal/pr/sitemap404.go` — PR plan builder for sitemap 404 cleanup
- Create: `internal/pr/sitemap404_test.go` — PR plan generation tests
- Create: `internal/scan/service.go` — end-to-end phase-1 scan orchestration
- Create: `internal/scan/service_test.go` — scan slice integration tests
- Create: `db/migrations/0001_phase1_core_tables.sql` — scan/finding/pr persistence schema
- Create: `internal/db/schema_test.go` — migration shape test

### Task 1: Bootstrap Module and Runtime Config

**Files:**
- Create: `go.mod`
- Create: `Makefile`
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

- [ ] **Step 1: Write the failing config tests**

```go
// internal/config/config_test.go
package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadReturnsErrorWhenRequiredEnvMissing(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("REDIS_URL", "")
	t.Setenv("GSC_CLIENT_ID", "")
	t.Setenv("GSC_CLIENT_SECRET", "")
	t.Setenv("GITHUB_APP_ID", "")
	t.Setenv("GITHUB_INSTALLATION_ID", "")

	_, err := Load()
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing required env")
}

func TestLoadBuildsConfigFromEnv(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost:5432/seo")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("GSC_CLIENT_ID", "cid")
	t.Setenv("GSC_CLIENT_SECRET", "csecret")
	t.Setenv("GITHUB_APP_ID", "1")
	t.Setenv("GITHUB_INSTALLATION_ID", "2")
	t.Setenv("SCAN_QPM", "600")

	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, 600, cfg.ScanQPM)
	require.Equal(t, "postgres://localhost:5432/seo", cfg.DatabaseURL)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config -run TestLoad -v`
Expected: FAIL with `undefined: Load`

- [ ] **Step 3: Write minimal config implementation**

```go
// internal/config/config.go
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	DatabaseURL          string
	RedisURL             string
	GSCClientID          string
	GSCClientSecret      string
	GitHubAppID          string
	GitHubInstallationID string
	ScanQPM              int
}

func Load() (Config, error) {
	cfg := Config{
		DatabaseURL:          os.Getenv("DATABASE_URL"),
		RedisURL:             os.Getenv("REDIS_URL"),
		GSCClientID:          os.Getenv("GSC_CLIENT_ID"),
		GSCClientSecret:      os.Getenv("GSC_CLIENT_SECRET"),
		GitHubAppID:          os.Getenv("GITHUB_APP_ID"),
		GitHubInstallationID: os.Getenv("GITHUB_INSTALLATION_ID"),
		ScanQPM:              600,
	}

	if raw := os.Getenv("SCAN_QPM"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil {
			return Config{}, fmt.Errorf("invalid SCAN_QPM: %w", err)
		}
		cfg.ScanQPM = v
	}

	missing := []string{}
	if cfg.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if cfg.RedisURL == "" {
		missing = append(missing, "REDIS_URL")
	}
	if cfg.GSCClientID == "" {
		missing = append(missing, "GSC_CLIENT_ID")
	}
	if cfg.GSCClientSecret == "" {
		missing = append(missing, "GSC_CLIENT_SECRET")
	}
	if cfg.GitHubAppID == "" {
		missing = append(missing, "GITHUB_APP_ID")
	}
	if cfg.GitHubInstallationID == "" {
		missing = append(missing, "GITHUB_INSTALLATION_ID")
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required env: %s", strings.Join(missing, ","))
	}

	return cfg, nil
}
```

- [ ] **Step 4: Add module and local task runner**

```go
// go.mod
module github.com/nodeops/seo-workflow

go 1.24

require github.com/stretchr/testify v1.10.0
```

```makefile
# Makefile
.PHONY: test

test:
	go test ./... -count=1
```

- [ ] **Step 5: Run tests to verify pass**

Run: `make test`
Expected: PASS with `ok   github.com/nodeops/seo-workflow/internal/config`

- [ ] **Step 6: Commit**

```bash
git add go.mod Makefile internal/config/config.go internal/config/config_test.go
git commit -m "chore: bootstrap module and config loader"
```

### Task 2: Implement Deterministic Classifier

**Files:**
- Create: `internal/classifier/classifier.go`
- Create: `internal/classifier/classifier_test.go`

- [ ] **Step 1: Write failing table-driven tests for bucket mapping**

```go
// internal/classifier/classifier_test.go
package classifier

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClassify(t *testing.T) {
	tests := []struct {
		name   string
		signal InspectionSignal
		want   Bucket
	}{
		{
			name: "sitemap 404",
			signal: InspectionSignal{CoverageState: "Not found (404)", InSitemap: true},
			want: Sitemap404,
		},
		{
			name: "canonical mismatch",
			signal: InspectionSignal{CoverageState: "Duplicate, Google chose different canonical"},
			want: CanonicalMismatch,
		},
		{
			name: "quality bucket",
			signal: InspectionSignal{CoverageState: "Crawled - currently not indexed"},
			want: QualityOrDup,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Classify(tc.signal)
			require.Equal(t, tc.want, got)
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/classifier -run TestClassify -v`
Expected: FAIL with `undefined: InspectionSignal`

- [ ] **Step 3: Implement classifier constants and switch mapping**

```go
// internal/classifier/classifier.go
package classifier

type Bucket string

const (
	Healthy            Bucket = "healthy"
	SitemapMissing     Bucket = "sitemap_missing_entry"
	SitemapRedirect    Bucket = "sitemap_redirect"
	Sitemap404         Bucket = "sitemap_404"
	CanonicalMismatch  Bucket = "canonical_mismatch"
	CanonicalMissing   Bucket = "canonical_missing"
	NoindexMisconfig   Bucket = "noindex_misconfig"
	RobotsMisconfig    Bucket = "robots_misconfig"
	QualityOrDup       Bucket = "quality_or_dup"
	CrawlBudget        Bucket = "crawl_budget"
	Soft404            Bucket = "soft_404"
	JSRendering        Bucket = "js_rendering"
	ServerError        Bucket = "server_error"
	BotBlocked         Bucket = "bot_blocked"
	Unknown            Bucket = "unknown"
)

type InspectionSignal struct {
	CoverageState string
	PageFetchState string
	InSitemap bool
}

func Classify(signal InspectionSignal) Bucket {
	switch {
	case signal.CoverageState == "Submitted and indexed":
		return Healthy
	case signal.CoverageState == "Indexed, not submitted in sitemap":
		return SitemapMissing
	case signal.CoverageState == "Page with redirect":
		return SitemapRedirect
	case signal.CoverageState == "Not found (404)" && signal.InSitemap:
		return Sitemap404
	case signal.CoverageState == "Duplicate, Google chose different canonical":
		return CanonicalMismatch
	case signal.CoverageState == "Duplicate without user-selected canonical":
		return CanonicalMissing
	case signal.CoverageState == "Excluded by 'noindex' tag" && signal.InSitemap:
		return NoindexMisconfig
	case signal.CoverageState == "Blocked by robots.txt" && signal.InSitemap:
		return RobotsMisconfig
	case signal.CoverageState == "Crawled - currently not indexed":
		return QualityOrDup
	case signal.CoverageState == "Discovered - currently not indexed":
		return CrawlBudget
	case signal.CoverageState == "Soft 404":
		return Soft404
	case signal.PageFetchState == "SOFT_404":
		return JSRendering
	case signal.PageFetchState == "SERVER_ERROR":
		return ServerError
	case signal.PageFetchState == "ACCESS_FORBIDDEN":
		return BotBlocked
	default:
		return Unknown
	}
}
```

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./internal/classifier -run TestClassify -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/classifier/classifier.go internal/classifier/classifier_test.go
git commit -m "feat: add deterministic indexing classifier"
```

### Task 3: Build URL Discovery Merge Logic

**Files:**
- Create: `internal/gsc/discovery.go`
- Create: `internal/gsc/discovery_test.go`

- [ ] **Step 1: Write failing tests for dedupe and impression ordering**

```go
// internal/gsc/discovery_test.go
package gsc

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMergeDiscoveries(t *testing.T) {
	sitemap := []string{"https://example.com/a", "https://example.com/b"}
	analytics := []URLMetric{
		{URL: "https://example.com/b", Impressions: 50},
		{URL: "https://example.com/c", Impressions: 100},
	}

	got := MergeDiscoveries(sitemap, analytics)
	want := []string{
		"https://example.com/c",
		"https://example.com/b",
		"https://example.com/a",
	}
	require.Equal(t, want, got)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/gsc -run TestMergeDiscoveries -v`
Expected: FAIL with `undefined: URLMetric`

- [ ] **Step 3: Implement merge function**

```go
// internal/gsc/discovery.go
package gsc

import "sort"

type URLMetric struct {
	URL         string
	Impressions int64
}

func MergeDiscoveries(sitemapURLs []string, analytics []URLMetric) []string {
	byURL := map[string]int64{}
	for _, u := range sitemapURLs {
		if u == "" {
			continue
		}
		if _, ok := byURL[u]; !ok {
			byURL[u] = 0
		}
	}
	for _, row := range analytics {
		if row.URL == "" {
			continue
		}
		if row.Impressions > byURL[row.URL] {
			byURL[row.URL] = row.Impressions
		}
	}

	urls := make([]string, 0, len(byURL))
	for u := range byURL {
		urls = append(urls, u)
	}

	sort.Slice(urls, func(i, j int) bool {
		li, lj := byURL[urls[i]], byURL[urls[j]]
		if li == lj {
			return urls[i] < urls[j]
		}
		return li > lj
	})

	return urls
}
```

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./internal/gsc -run TestMergeDiscoveries -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/gsc/discovery.go internal/gsc/discovery_test.go
git commit -m "feat: merge sitemap and analytics URL discovery"
```

### Task 4: Add Per-Property Rate Limiter

**Files:**
- Create: `internal/ratelimit/property_limiter.go`
- Create: `internal/ratelimit/property_limiter_test.go`

- [ ] **Step 1: Write failing tests for shared limiter reuse**

```go
// internal/ratelimit/property_limiter_test.go
package ratelimit

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWaitCreatesAndReusesPropertyLimiter(t *testing.T) {
	l := NewPropertyLimiter(600, 10)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	require.NoError(t, l.Wait(ctx, "sc-domain:example.com"))
	require.NoError(t, l.Wait(ctx, "sc-domain:example.com"))
	require.Equal(t, 1, l.Size())
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ratelimit -run TestWaitCreatesAndReusesPropertyLimiter -v`
Expected: FAIL with `undefined: NewPropertyLimiter`

- [ ] **Step 3: Implement limiter map**

```go
// internal/ratelimit/property_limiter.go
package ratelimit

import (
	"context"
	"sync"

	"golang.org/x/time/rate"
)

type PropertyLimiter struct {
	mu      sync.Mutex
	qpm     int
	burst   int
	buckets map[string]*rate.Limiter
}

func NewPropertyLimiter(qpm int, burst int) *PropertyLimiter {
	return &PropertyLimiter{
		qpm:     qpm,
		burst:   burst,
		buckets: map[string]*rate.Limiter{},
	}
}

func (p *PropertyLimiter) Wait(ctx context.Context, property string) error {
	limiter := p.get(property)
	return limiter.Wait(ctx)
}

func (p *PropertyLimiter) Size() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.buckets)
}

func (p *PropertyLimiter) get(property string) *rate.Limiter {
	p.mu.Lock()
	defer p.mu.Unlock()
	if l, ok := p.buckets[property]; ok {
		return l
	}
	perSecond := rate.Limit(float64(p.qpm) / 60.0)
	l := rate.NewLimiter(perSecond, p.burst)
	p.buckets[property] = l
	return l
}
```

- [ ] **Step 4: Update dependencies and run tests**

Run: `go get golang.org/x/time/rate && go test ./internal/ratelimit -run TestWaitCreatesAndReusesPropertyLimiter -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum internal/ratelimit/property_limiter.go internal/ratelimit/property_limiter_test.go
git commit -m "feat: add per-property URL inspection limiter"
```

### Task 5: Implement Inspection Worker Orchestration

**Files:**
- Create: `internal/worker/interfaces.go`
- Create: `internal/worker/inspect_job.go`
- Create: `internal/worker/inspect_job_test.go`

- [ ] **Step 1: Write failing worker tests with fakes**

```go
// internal/worker/inspect_job_test.go
package worker

import (
	"context"
	"testing"

	"github.com/nodeops/seo-workflow/internal/classifier"
	"github.com/stretchr/testify/require"
)

type fakeInspector struct{}

func (fakeInspector) InspectURL(context.Context, string, string) (classifier.InspectionSignal, error) {
	return classifier.InspectionSignal{CoverageState: "Not found (404)", InSitemap: true}, nil
}

type fakeLimiter struct{}

func (fakeLimiter) Wait(context.Context, string) error { return nil }

func TestRunInspectionJobClassifiesEachURL(t *testing.T) {
	urls := []string{"https://example.com/a", "https://example.com/b"}
	findings, err := RunInspectionJob(context.Background(), fakeInspector{}, fakeLimiter{}, "sc-domain:example.com", urls)
	require.NoError(t, err)
	require.Len(t, findings, 2)
	require.Equal(t, classifier.Sitemap404, findings[0].Bucket)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/worker -run TestRunInspectionJobClassifiesEachURL -v`
Expected: FAIL with `undefined: RunInspectionJob`

- [ ] **Step 3: Implement worker interfaces and orchestration**

```go
// internal/worker/interfaces.go
package worker

import (
	"context"

	"github.com/nodeops/seo-workflow/internal/classifier"
)

type Inspector interface {
	InspectURL(ctx context.Context, property string, url string) (classifier.InspectionSignal, error)
}

type PropertyWaiter interface {
	Wait(ctx context.Context, property string) error
}

type Finding struct {
	URL    string
	Bucket classifier.Bucket
}
```

```go
// internal/worker/inspect_job.go
package worker

import (
	"context"

	"github.com/nodeops/seo-workflow/internal/classifier"
)

func RunInspectionJob(ctx context.Context, inspector Inspector, limiter PropertyWaiter, property string, urls []string) ([]Finding, error) {
	findings := make([]Finding, 0, len(urls))
	for _, url := range urls {
		if err := limiter.Wait(ctx, property); err != nil {
			return nil, err
		}
		signal, err := inspector.InspectURL(ctx, property, url)
		if err != nil {
			return nil, err
		}
		findings = append(findings, Finding{URL: url, Bucket: classifier.Classify(signal)})
	}
	return findings, nil
}
```

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./internal/worker -run TestRunInspectionJobClassifiesEachURL -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/worker/interfaces.go internal/worker/inspect_job.go internal/worker/inspect_job_test.go
git commit -m "feat: orchestrate rate-limited URL inspection job"
```

### Task 6: Build `sitemap_404` PR Plan Generator

**Files:**
- Create: `internal/pr/sitemap404.go`
- Create: `internal/pr/sitemap404_test.go`

- [ ] **Step 1: Write failing tests for PR plan generation**

```go
// internal/pr/sitemap404_test.go
package pr

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildSitemap404PR(t *testing.T) {
	sitemap := `<urlset><url><loc>https://example.com/a</loc></url><url><loc>https://example.com/b</loc></url></urlset>`
	plan := BuildSitemap404PR([]string{"https://example.com/b"}, sitemap)

	require.Equal(t, "seo/fix-sitemap-404", plan.Branch)
	require.Equal(t, "fix(seo): remove 404 URLs from sitemap", plan.Title)
	require.Len(t, plan.Files, 1)
	require.NotContains(t, plan.Files[0].Content, "https://example.com/b")
	require.Contains(t, plan.Files[0].Content, "https://example.com/a")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/pr -run TestBuildSitemap404PR -v`
Expected: FAIL with `undefined: BuildSitemap404PR`

- [ ] **Step 3: Implement PR plan builder**

```go
// internal/pr/sitemap404.go
package pr

import "strings"

type FileEdit struct {
	Path    string
	Content string
}

type PullRequestPlan struct {
	Branch string
	Title  string
	Body   string
	Files  []FileEdit
}

func BuildSitemap404PR(notFoundURLs []string, currentSitemap string) PullRequestPlan {
	next := currentSitemap
	for _, u := range notFoundURLs {
		next = strings.ReplaceAll(next, "<url><loc>"+u+"</loc></url>", "")
	}

	return PullRequestPlan{
		Branch: "seo/fix-sitemap-404",
		Title:  "fix(seo): remove 404 URLs from sitemap",
		Body:   "This PR removes URLs reported as `Not found (404)` by Google Search Console from `sitemap.xml`.",
		Files: []FileEdit{
			{Path: "public/sitemap.xml", Content: next},
		},
	}
}
```

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./internal/pr -run TestBuildSitemap404PR -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/pr/sitemap404.go internal/pr/sitemap404_test.go
git commit -m "feat: generate sitemap 404 pull request plans"
```

### Task 7: Create Scan Service Vertical Slice

**Files:**
- Create: `internal/scan/service.go`
- Create: `internal/scan/service_test.go`
- Modify: `internal/worker/interfaces.go` (if new interfaces are required)

- [ ] **Step 1: Write failing service test for discovery → inspect → PR path**

```go
// internal/scan/service_test.go
package scan

import (
	"context"
	"testing"

	"github.com/nodeops/seo-workflow/internal/classifier"
	"github.com/nodeops/seo-workflow/internal/gsc"
	"github.com/nodeops/seo-workflow/internal/pr"
	"github.com/stretchr/testify/require"
)

type fakeDiscovery struct{}

func (fakeDiscovery) Discover(context.Context, string) ([]string, []gsc.URLMetric, error) {
	return []string{"https://example.com/a", "https://example.com/b"}, []gsc.URLMetric{{URL: "https://example.com/b", Impressions: 10}}, nil
}

type fakeInspector struct{}

func (fakeInspector) InspectURL(context.Context, string, string) (classifier.InspectionSignal, error) {
	return classifier.InspectionSignal{CoverageState: "Not found (404)", InSitemap: true}, nil
}

type fakeSitemapLoader struct{}

func (fakeSitemapLoader) Load(context.Context, string) (string, error) {
	return `<urlset><url><loc>https://example.com/a</loc></url><url><loc>https://example.com/b</loc></url></urlset>`, nil
}

type fakePROpener struct {
	last pr.PullRequestPlan
}

func (f *fakePROpener) Open(context.Context, string, pr.PullRequestPlan) (string, error) {
	f.last = pr.PullRequestPlan{Title: "fix(seo): remove 404 URLs from sitemap"}
	return "https://github.com/org/repo/pull/234", nil
}

func TestRunPropertyScanCreatesSitemap404PR(t *testing.T) {
	opener := &fakePROpener{}
	svc := NewService(fakeDiscovery{}, fakeInspector{}, fakeSitemapLoader{}, opener)

	summary, err := svc.RunPropertyScan(context.Background(), "sc-domain:example.com")
	require.NoError(t, err)
	require.Equal(t, 2, summary.TotalURLs)
	require.Equal(t, 2, summary.BucketCounts["sitemap_404"])
	require.Equal(t, "https://github.com/org/repo/pull/234", summary.PullRequestURL)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scan -run TestRunPropertyScanCreatesSitemap404PR -v`
Expected: FAIL with `undefined: NewService`

- [ ] **Step 3: Implement scan service orchestration**

```go
// internal/scan/service.go
package scan

import (
	"context"

	"github.com/nodeops/seo-workflow/internal/classifier"
	"github.com/nodeops/seo-workflow/internal/gsc"
	"github.com/nodeops/seo-workflow/internal/pr"
)

type DiscoveryClient interface {
	Discover(ctx context.Context, property string) ([]string, []gsc.URLMetric, error)
}

type Inspector interface {
	InspectURL(ctx context.Context, property string, url string) (classifier.InspectionSignal, error)
}

type SitemapLoader interface {
	Load(ctx context.Context, property string) (string, error)
}

type PROpener interface {
	Open(ctx context.Context, property string, plan pr.PullRequestPlan) (string, error)
}

type Service struct {
	discovery DiscoveryClient
	inspector Inspector
	sitemap   SitemapLoader
	opener    PROpener
}

type Summary struct {
	TotalURLs      int
	BucketCounts   map[string]int
	PullRequestURL string
}

func NewService(discovery DiscoveryClient, inspector Inspector, sitemap SitemapLoader, opener PROpener) *Service {
	return &Service{discovery: discovery, inspector: inspector, sitemap: sitemap, opener: opener}
}

func (s *Service) RunPropertyScan(ctx context.Context, property string) (Summary, error) {
	sitemapURLs, analyticsURLs, err := s.discovery.Discover(ctx, property)
	if err != nil {
		return Summary{}, err
	}
	urls := gsc.MergeDiscoveries(sitemapURLs, analyticsURLs)

	bucketCounts := map[string]int{}
	notFound := []string{}
	for _, u := range urls {
		signal, err := s.inspector.InspectURL(ctx, property, u)
		if err != nil {
			return Summary{}, err
		}
		bucket := classifier.Classify(signal)
		bucketCounts[string(bucket)]++
		if bucket == classifier.Sitemap404 {
			notFound = append(notFound, u)
		}
	}

	pullURL := ""
	if len(notFound) > 0 {
		rawSitemap, err := s.sitemap.Load(ctx, property)
		if err != nil {
			return Summary{}, err
		}
		plan := pr.BuildSitemap404PR(notFound, rawSitemap)
		pullURL, err = s.opener.Open(ctx, property, plan)
		if err != nil {
			return Summary{}, err
		}
	}

	return Summary{TotalURLs: len(urls), BucketCounts: bucketCounts, PullRequestURL: pullURL}, nil
}
```

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./internal/scan -run TestRunPropertyScanCreatesSitemap404PR -v`
Expected: PASS

- [ ] **Step 5: Run full test suite**

Run: `go test ./... -count=1`
Expected: PASS across `internal/config`, `internal/classifier`, `internal/gsc`, `internal/ratelimit`, `internal/worker`, `internal/pr`, `internal/scan`

- [ ] **Step 6: Commit**

```bash
git add internal/scan/service.go internal/scan/service_test.go
git commit -m "feat: add phase1 property scan orchestration"
```

### Task 8: Add Phase-1 Schema and Worker Entrypoint

**Files:**
- Create: `db/migrations/0001_phase1_core_tables.sql`
- Create: `internal/db/schema_test.go`
- Create: `cmd/worker/main.go`

- [ ] **Step 1: Write failing schema contract test**

```go
// internal/db/schema_test.go
package db

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPhase1MigrationContainsCoreTables(t *testing.T) {
	body, err := os.ReadFile("../../db/migrations/0001_phase1_core_tables.sql")
	require.NoError(t, err)
	sql := string(body)
	require.Contains(t, sql, "CREATE TABLE properties")
	require.Contains(t, sql, "CREATE TABLE scans")
	require.Contains(t, sql, "CREATE TABLE findings")
	require.Contains(t, sql, "CREATE TABLE pull_requests")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/db -run TestPhase1MigrationContainsCoreTables -v`
Expected: FAIL with `no such file or directory`

- [ ] **Step 3: Create migration and minimal worker main**

```sql
-- db/migrations/0001_phase1_core_tables.sql
CREATE TABLE properties (
  id BIGSERIAL PRIMARY KEY,
  gsc_property TEXT NOT NULL UNIQUE,
  repo_full_name TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE scans (
  id BIGSERIAL PRIMARY KEY,
  property_id BIGINT NOT NULL REFERENCES properties(id),
  started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  completed_at TIMESTAMPTZ,
  status TEXT NOT NULL CHECK (status IN ('running','success','failed'))
);

CREATE TABLE findings (
  id BIGSERIAL PRIMARY KEY,
  scan_id BIGINT NOT NULL REFERENCES scans(id),
  url TEXT NOT NULL,
  bucket TEXT NOT NULL,
  coverage_state TEXT NOT NULL,
  page_fetch_state TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE pull_requests (
  id BIGSERIAL PRIMARY KEY,
  scan_id BIGINT NOT NULL REFERENCES scans(id),
  bucket TEXT NOT NULL,
  branch_name TEXT NOT NULL,
  pr_url TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

```go
// cmd/worker/main.go
package main

import (
	"log"

	"github.com/nodeops/seo-workflow/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	log.Printf("worker booted (qpm=%d)", cfg.ScanQPM)
}
```

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./internal/db -run TestPhase1MigrationContainsCoreTables -v`
Expected: PASS

- [ ] **Step 5: Smoke run worker entrypoint**

Run: `DATABASE_URL=postgres://localhost REDIS_URL=redis://localhost GSC_CLIENT_ID=x GSC_CLIENT_SECRET=y GITHUB_APP_ID=1 GITHUB_INSTALLATION_ID=2 go run ./cmd/worker`
Expected: logs `worker booted (qpm=600)`

- [ ] **Step 6: Commit**

```bash
git add db/migrations/0001_phase1_core_tables.sql internal/db/schema_test.go cmd/worker/main.go
git commit -m "feat: add phase1 schema and worker entrypoint"
```

## Self-Review

### 1. Spec coverage

- Covered:
- Deterministic classifier (`FEATURE.md` “The classifier” section)
- URL discovery via sitemap + search analytics (`Phase 1` section)
- URL inspection loop + per-property rate limiting (`URL Inspection quota` section)
- One mechanical PR generator (`sitemap_404`) from Phase 1
- Not covered in this plan:
- OAuth onboarding and token rotation
- Slack/email digests
- Dashboard UI
- Competitor watcher

### 2. Placeholder scan

No `TBD`, `TODO`, “implement later”, or implicit “write tests” placeholders remain. Each code step includes concrete snippets and command assertions.

### 3. Type consistency

- `classifier.InspectionSignal` used consistently across worker and scan service.
- `pr.PullRequestPlan` used consistently in PR builder and scan service opener.
- Bucket names align with classifier constants and scan summary map keys.

## Follow-Up Plans (required to complete full `FEATURE.md`)

1. `docs/superpowers/plans/2026-04-30-seo-workflow-phase1-oauth-onboarding.md`
2. `docs/superpowers/plans/2026-04-30-seo-workflow-phase2-pr-notifications-dashboard.md`
3. `docs/superpowers/plans/2026-04-30-seo-workflow-phase3-competitor-watcher.md`

Plan complete and saved to `docs/superpowers/plans/2026-04-30-seo-workflow-phase1-core-scan.md`. Two execution options:

1. Subagent-Driven (recommended) - I dispatch a fresh subagent per task, review between tasks, fast iteration

2. Inline Execution - Execute tasks in this session using executing-plans, batch execution with checkpoints

Which approach?
