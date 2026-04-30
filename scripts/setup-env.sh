#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="$ROOT_DIR/.env"

prompt_default() {
  local var_name="$1"
  local prompt_text="$2"
  local default_value="$3"
  local current_value="${!var_name:-}"

  if [[ -n "$current_value" ]]; then
    default_value="$current_value"
  fi

  read -r -p "$prompt_text [$default_value]: " input
  if [[ -z "$input" ]]; then
    printf -v "$var_name" '%s' "$default_value"
  else
    printf -v "$var_name" '%s' "$input"
  fi
}

prompt_secret() {
  local var_name="$1"
  local prompt_text="$2"
  local current_value="${!var_name:-}"

  if [[ -n "$current_value" ]]; then
    read -r -s -p "$prompt_text [press Enter to keep existing value]: " input
    echo
    if [[ -z "$input" ]]; then
      return
    fi
    printf -v "$var_name" '%s' "$input"
    return
  fi

  read -r -s -p "$prompt_text: " input
  echo
  printf -v "$var_name" '%s' "$input"
}

if [[ -f "$ENV_FILE" ]]; then
  # shellcheck disable=SC1090
  source "$ENV_FILE"
fi

echo "Generating $ENV_FILE"
echo

prompt_default WORKER_MODE "Worker mode" "oneshot"
if [[ "$WORKER_MODE" == "oneshot-competitor" ]]; then
  prompt_default OUR_SITEMAP_URL "CreateOS sitemap URL" "https://createos.sh/sitemap.xml"
  prompt_default COMPETITOR_WINDOW_DAYS "Window days" "30"
  prompt_default COMPETITOR_HTTP_TIMEOUT_SEC "HTTP timeout seconds" "30"
  prompt_default COMPETITOR_REPORT_PATH "Competitor report path" "competitor-report.json"
  prompt_secret OPENROUTER_API_KEY "OpenRouter API key (optional; press Enter to leave empty)"
  prompt_default OPENROUTER_MODEL "OpenRouter model" "moonshotai/kimi-k2"

  cat > "$ENV_FILE" <<ENV
WORKER_MODE=$WORKER_MODE
OUR_SITEMAP_URL=$OUR_SITEMAP_URL
COMPETITOR_WINDOW_DAYS=$COMPETITOR_WINDOW_DAYS
COMPETITOR_HTTP_TIMEOUT_SEC=$COMPETITOR_HTTP_TIMEOUT_SEC
COMPETITOR_REPORT_PATH=$COMPETITOR_REPORT_PATH
OPENROUTER_API_KEY=${OPENROUTER_API_KEY:-}
OPENROUTER_MODEL=$OPENROUTER_MODEL
ENV
else
  prompt_default SCAN_PROPERTY "GSC property (e.g. sc-domain:example.com)" "sc-domain:example.com"
  prompt_default SCAN_REPO "GitHub repo (owner/repo)" "owner/repo"
  prompt_default SCAN_SITEMAP_URL "Sitemap URL" "https://example.com/sitemap.xml"
  prompt_secret GSC_ACCESS_TOKEN "Google Search Console access token (starts with ya29.)"

  prompt_default WORKER_DRY_RUN "Dry run (true/false)" "true"
  if [[ "$WORKER_DRY_RUN" == "false" ]]; then
    prompt_secret GITHUB_TOKEN "GitHub token (required for real PR open)"
  fi

  prompt_default SCAN_QPM "Scan QPM" "600"
  prompt_default GSC_LOOKBACK_DAYS "GSC lookback days" "7"
  prompt_default GSC_ROW_LIMIT "GSC row limit" "1000"
  prompt_default GSC_HTTP_TIMEOUT_SEC "HTTP timeout seconds" "30"
  prompt_default GITHUB_BASE_BRANCH "GitHub base branch" "main"
  prompt_default GITHUB_SITEMAP_PATH "Repo sitemap path" "public/sitemap.xml"

  cat > "$ENV_FILE" <<ENV
WORKER_MODE=$WORKER_MODE

SCAN_PROPERTY=$SCAN_PROPERTY
SCAN_REPO=$SCAN_REPO
SCAN_SITEMAP_URL=$SCAN_SITEMAP_URL

GSC_ACCESS_TOKEN=$GSC_ACCESS_TOKEN

WORKER_DRY_RUN=$WORKER_DRY_RUN
GITHUB_TOKEN=${GITHUB_TOKEN:-}

SCAN_QPM=$SCAN_QPM
GSC_LOOKBACK_DAYS=$GSC_LOOKBACK_DAYS
GSC_ROW_LIMIT=$GSC_ROW_LIMIT
GSC_HTTP_TIMEOUT_SEC=$GSC_HTTP_TIMEOUT_SEC
GITHUB_BASE_BRANCH=$GITHUB_BASE_BRANCH
GITHUB_SITEMAP_PATH=$GITHUB_SITEMAP_PATH
ENV
fi

echo
echo "Saved $ENV_FILE"
if [[ "$WORKER_MODE" == "oneshot-competitor" ]]; then
  echo "Run: make smoke-competitor"
else
  echo "Run: make smoke-oneshot"
fi
