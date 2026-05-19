# Competitor One-Shot Workflow

This mode compares recent sitemap changes for CreateOS vs competitors and outputs actionable opportunities with impact scores.

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

# optional OpenRouter/Kimi topic extraction (primary opportunity flow when set)
export OPENROUTER_API_KEY='sk-or-...'
export OPENROUTER_MODEL='moonshotai/kimi-k2'

# optional separate model for generated blog/page drafts
# examples: qwen/qwen3.6-flash, qwen/qwen3.6-27b, moonshotai/kimi-k2.5
export OPENROUTER_DRAFT_MODEL='qwen/qwen3.6-flash'

# optional draft generation controls
# default draft limit is 1; increase when you want more content PR drafts per run
export OPENROUTER_DRAFT_TIMEOUT_SEC='360'
export COMPETITOR_CONTENT_DRAFT_LIMIT='1'

# optional generated cover image model
export OPENROUTER_COVER_MODEL='google/gemini-2.5-flash-image'

# required for generated cover images; should point at the public base URL
# where files committed under covers/ in the content repo will be served
export CONTENT_COVER_ASSET_BASE_URL='https://createos-content.example.com'

# content repo PR publishing
export GITHUB_TOKEN='github_pat_...'
export CONTENT_REPO='NodeOps-app/createos-content'
export CONTENT_BASE_BRANCH='main'
```

## 3) Run

```bash
make smoke-competitor
```

or:

```bash
go run ./cmd/worker
```

## 4) Output

- Per-competitor sitemap stats and theme counts.
- Ranked opportunities with:
  - title
  - why it matters
  - what to do
  - how to execute
  - impact score (1-100)
- Optional JSON report file via `COMPETITOR_REPORT_PATH`.
- Optional content repo PR when GitHub/content repo env vars are set.
  - Generated blog frontmatter uses `destination: createos`.
  - Generated cover assets are committed under `covers/` when cover generation succeeds.
- Treat the report as a heuristic input, not a source of truth.
- Prioritize exact URL evidence and ignore low-specificity phrases.
