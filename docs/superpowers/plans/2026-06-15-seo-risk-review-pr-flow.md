# SEO Risk Review PR Flow Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create every generated content PR, mark risky drafts with evidence in the PR body, and automatically create a mitigated companion PR for high/critical SEO risk.

**Architecture:** Add a deterministic SEO risk analyzer in `internal/contentrepo`, extend GitHub publishing with PR body options and PR numbers, then update the worker to publish the original post with risk review and publish a mitigated post when risk is high. The AI remains advisory: it creates drafts, while deterministic checks expose risk and humans decide what to merge.

**Tech Stack:** Go, existing GitHub REST publisher, existing `contentrepo.BlogPost` markdown/frontmatter model, existing worker tests with HTTP stubs.

---

### Task 1: SEO Risk Analyzer

**Files:**
- Create: `internal/contentrepo/seo_risk.go`
- Test: `internal/contentrepo/seo_risk_test.go`

- [ ] Write failing tests for high-risk keyword repetition, missing methodology in listicles, and risk markdown rendering.
- [ ] Run `go test ./internal/contentrepo -run 'TestAssessSEORisk|TestSEORiskReportMarkdown'` and verify failure.
- [ ] Implement `SEORiskReport`, `SEORiskFinding`, `AssessSEORisk`, `MitigateSEORisk`, and markdown rendering.
- [ ] Run targeted tests and verify pass.

### Task 2: Publish Options

**Files:**
- Modify: `internal/contentrepo/github_publisher.go`
- Modify tests: `internal/contentrepo/github_publisher_test.go`

- [ ] Write failing test proving PR bodies include risk review and PR titles can use `RISKY:` / `MITIGATED:` prefixes.
- [ ] Run targeted test and verify failure.
- [ ] Add `PublishOptions`, `PublishWithOptions`, `PRBodyExtraMarkdown`, `TitlePrefix`, `RelatedPullRequestURL`, and return PR number in `PublishResult`.
- [ ] Run targeted tests and verify pass.

### Task 3: Worker Risk Flow

**Files:**
- Modify: `cmd/worker/content_pr.go`
- Modify tests: `cmd/worker/content_pr_test.go`

- [ ] Write failing test proving a high-risk draft creates two PRs: original risky PR and mitigated PR.
- [ ] Run targeted test and verify failure.
- [ ] Update worker publish flow to assess risk after cover generation, publish original PR with risk review, and publish mitigated companion PR for high/critical risk.
- [ ] Run targeted tests and verify pass.

### Task 4: Verification

**Files:**
- Existing repo tests

- [ ] Run `gofmt` on edited Go files.
- [ ] Run `go test ./...`.
- [ ] Confirm no unrelated files are staged.
