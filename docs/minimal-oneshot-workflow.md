# Minimal One-Shot Workflow

For competitor opportunity mode, see [`docs/competitor-oneshot-workflow.md`](./competitor-oneshot-workflow.md).

This runs a single real scan for one GSC property and one GitHub repo.
The worker automatically loads `.env` from repo root (if present).

## 1) Required env vars

```bash
export WORKER_MODE=oneshot
export SCAN_PROPERTY='sc-domain:example.com'
export SCAN_REPO='owner/repo'
export SCAN_SITEMAP_URL='https://example.com/sitemap.xml'
export GSC_ACCESS_TOKEN='ya29....'
```

## 2) Optional env vars

```bash
# true by default; if false, GitHub token is required and a PR is opened
export WORKER_DRY_RUN=true

# required only when WORKER_DRY_RUN=false
export GITHUB_TOKEN='ghp_...'

# optional tuning
export GITHUB_BASE_BRANCH='main'
export GITHUB_SITEMAP_PATH='public/sitemap.xml'
export SCAN_QPM=600
export GSC_LOOKBACK_DAYS=7
export GSC_ROW_LIMIT=1000
export GSC_HTTP_TIMEOUT_SEC=30
export ONESHOT_REPORT_PATH=oneshot-report.json
```

## 3) Run

```bash
./scripts/setup-env.sh
make smoke-oneshot
```

Or directly:

```bash
go run ./cmd/worker
```

## 4) Expected output

- Startup line showing property/repo/dry-run.
- Completion summary with:
  - total scanned URLs
  - bucket counts
  - PR URL (or `dry-run://...` URL when dry-run is enabled)
- Per-URL finding logs with bucket + coverage/pageFetch state.
- Optional JSON report file when `ONESHOT_REPORT_PATH` is set.
