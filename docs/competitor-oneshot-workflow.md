# Competitor One-Shot Workflow

This mode compares recent sitemap changes for CreateOS vs competitors and outputs actionable opportunities with impact scores.

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

# optional LLM refinement of opportunities
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
