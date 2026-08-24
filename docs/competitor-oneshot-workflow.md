# Competitor One-Shot Workflow

This workflow separates title discovery from drafting and publishing. The discovery run compares recent sitemap changes for CreateOS vs competitors, ranks focused blog opportunities, and writes a human approval queue. It never generates a draft or opens a content PR.

The repository also includes the reusable `.codex/skills/createos-blog-opportunities` skill for running and reviewing this flow from Codex.

When `OPENROUTER_API_KEY` is set, the primary flow is:
1. Fetch sitemap URLs for each competitor.
2. Fetch page titles for recent competitor URLs.
3. Extract concrete themes from title+URL evidence with OpenRouter/Kimi.
4. Compare extracted competitor themes with CreateOS coverage.
5. Emit topic-gap opportunities.

The slug analyzer remains a fallback when title fetching or LLM topic extraction is unavailable.

## Supported competitors

- Vercel (`https://vercel.com/sitemap.xml`)
- Lovable (`https://lovable.dev/sitemap.xml`)
- Replit (`https://replit.com/sitemap.xml`)
- Emergent (`https://emergent.sh/sitemap.xml`)
- StackAI (`https://www.stackai.com/sitemap.xml`)
- Lyzr (`https://www.lyzr.ai/sitemap.xml`)

## 1) Required env vars

```bash
export WORKER_MODE=oneshot-competitor
export OUR_SITEMAP_URL='https://createos.sh/sitemap.xml'
```

## 2) Optional env vars

```bash
# default: 30
export COMPETITOR_WINDOW_DAYS=30

# default: 30
export COMPETITOR_HTTP_TIMEOUT_SEC=30

# write JSON report to disk
export COMPETITOR_REPORT_PATH='competitor-report.json'

# optional local state file for date-less competitor sitemap tracking
# first run establishes a baseline; later runs treat newly observed URLs as fresh
export COMPETITOR_STATE_PATH='tmp/competitor-state.json'

# full createos-content inventory used before drafting; local checkout is fastest
# when omitted, the configured GitHub content repo is downloaded using GITHUB_TOKEN
export CONTENT_INVENTORY_PATH='../createos-content'

# when GITHUB_TOKEN is set, open content PRs are merged into the inventory so
# pending posts and pillar refreshes cannot be recommended as new duplicates
export GITHUB_TOKEN='github_pat_...'

# state written by WORKER_MODE=oneshot and consumed by competitor recommendations
export SEO_PERFORMANCE_STATE_PATH='tmp/search-performance.json'

# optional future manual/provider export for AI citation observations
export AEO_OBSERVATIONS_PATH='tmp/aeo-observations.csv'

# optional OpenRouter/Kimi topic extraction (primary opportunity flow when set)
export OPENROUTER_API_KEY='sk-or-...'
export OPENROUTER_MODEL='moonshotai/kimi-k2'

# used by WORKER_MODE=approved-content after a title is approved
# examples: qwen/qwen3.6-flash, qwen/qwen3.6-27b, moonshotai/kimi-k2.5
export OPENROUTER_DRAFT_MODEL='qwen/qwen3.6-flash'

# optional draft generation controls
# default draft limit is 1; increase when you want more content PR drafts per run
export OPENROUTER_DRAFT_TIMEOUT_SEC='360'
export COMPETITOR_CONTENT_DRAFT_LIMIT='1'

# discovery writes this queue; edit approvalStatus to approved or rejected
export CONTENT_APPROVAL_PATH='content-approvals.json'

# optional manual content seed; when set, this title is drafted first and the
# normal competitor opportunities remain available as fallback candidates
export CONTENT_MANUAL_TITLE='Top AI Agent Platforms for Enterprise Teams in 2026'

# optional manual seed controls
# supported themes include comparison, usecases, enterprise, security,
# integrations, workflow, agents, ai, vibecoding, deployment
export CONTENT_MANUAL_THEME='comparison'
export CONTENT_MANUAL_SLUG='top-ai-agent-platforms-enterprise-teams'
export CONTENT_MANUAL_ANGLE='Rank enterprise agent platforms through a CreateOS deployment-control lens.'
export CONTENT_MANUAL_COMPETITOR='lyzr'
export CONTENT_MANUAL_EVIDENCE_URLS='https://www.lyzr.ai/blog/agents,https://www.stack-ai.com/blog/platforms'

# generated covers use the local CreateOS design-system renderer
# recommended public image hosting for generated covers
# Cloudinary has a free plan; create a cloud and API key, then set:
export CLOUDINARY_CLOUD_NAME='your-cloud-name'
export CLOUDINARY_API_KEY='123456789'
export CLOUDINARY_API_SECRET='cloudinary-secret'
export CLOUDINARY_UPLOAD_FOLDER='createos/blog-covers'

# legacy fallback only: use when files committed under covers/ in the content
# repo are served by a public CDN. Do not point this at private GitHub raw URLs.
export CONTENT_COVER_ASSET_BASE_URL='https://public-cdn.example.com'

# content repo PR publishing
export CONTENT_REPO='NodeOps-app/createos-content'
export CONTENT_BASE_BRANCH='main'
```

## 3) Discover titles

```bash
make smoke-competitor
```

or:

```bash
go run ./cmd/worker
```

Review `content-approvals.json`. Each candidate includes its stable approval ID, ranked priority, keyword/intent, competitor evidence, cannibalization decision, and internal cluster targets. Set `approvalStatus` to `approved` only for titles that should become new blogs. Approved `refresh` and `consolidate` recommendations remain editorial actions against existing pages and are not published as duplicate posts.

## 4) Draft, review, and open content PRs

```bash
export WORKER_MODE=approved-content
export OPENROUTER_API_KEY='sk-or-...'
export GITHUB_TOKEN='github_pat_...'
export COMPETITOR_REPORT_PATH='approved-content-report.json'
make publish-approved-blogs
```

The approved title is locked during generation. The PR includes automated SEO review for thin content, repetition, listicle methodology, contextual CreateOS links, and authoritative external sourcing.

## 5) Output

- Per-competitor sitemap stats and theme counts.
- Date-less competitor tracking when `COMPETITOR_STATE_PATH` is set.
  - This is the recommended mode for StackAI because its sitemap does not expose useful `<lastmod>` values.
  - The first run stores a baseline.
  - Later runs use `first_seen` freshness for newly observed URLs.
- Ranked opportunities with:
  - title
  - why it matters
  - what to do
  - how to execute
  - impact score (1-100)
- Optional JSON report file via `COMPETITOR_REPORT_PATH`.
- Full blog inventory and cannibalization findings via `inventoryReport`.
- Search-performance opportunities and query cannibalization via `searchPerformance` when `SEO_PERFORMANCE_STATE_PATH` is configured.
- Optional `refreshQueue` recommendations for volatile AI, comparison, workflow, integration, and pricing blogs.
- Optional `aeoReport.prompts` matrix for manual ChatGPT/Perplexity/Gemini citation checks.
- Optional content repo PR when GitHub/content repo env vars are set.
  - Generated blog frontmatter uses `destination: createos`.
  - Every publishable route is forced to `/blogs/<slug>`; existing commercial routes are never created or replaced by this workflow.
  - Existing keyword ownership produces a refresh/consolidate decision instead of a duplicate blog draft.
  - Generated cover images are uploaded to Cloudinary when Cloudinary env vars are configured.
  - Without Cloudinary, generated cover assets are committed under `covers/` only when `CONTENT_COVER_ASSET_BASE_URL` points at a public CDN.
  - Every generated PR includes an automated SEO risk review in the PR body.
  - High or critical SEO risk creates a `RISKY:` original PR plus a `MITIGATED:` companion PR. Humans should merge only one.
- Optional manual content seed via `CONTENT_MANUAL_TITLE`.
  - The workflow still fetches CreateOS and competitor sitemaps, reads CreateOS guidance docs, builds internal link candidates, generates the draft, generates/uploads the cover image, and opens the content PR through the same path.
  - Any `CONTENT_MANUAL_SLUG` is normalized to `/blogs/<slug>`.
  - Keep `COMPETITOR_CONTENT_DRAFT_LIMIT=1` for a single manual article, or raise it when you want automatic competitor-gap drafts as fallback candidates.
- Treat the report as a heuristic input, not a source of truth.
- Prioritize exact URL evidence and ignore low-specificity phrases.
