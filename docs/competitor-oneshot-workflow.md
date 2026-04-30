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
- Treat the report as a heuristic input, not a source of truth.
- Prioritize exact URL evidence and ignore low-specificity phrases.
