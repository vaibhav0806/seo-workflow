package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func setRequiredEnv(t *testing.T) {
	t.Setenv("WORKER_MODE", "standby")
	t.Setenv("DATABASE_URL", "postgres://db")
	t.Setenv("REDIS_URL", "redis://cache")
	t.Setenv("GSC_CLIENT_ID", "client-id")
	t.Setenv("GSC_CLIENT_SECRET", "client-secret")
	t.Setenv("GITHUB_APP_ID", "123")
	t.Setenv("GITHUB_INSTALLATION_ID", "456")
}

func TestLoadMissingRequiredEnv(t *testing.T) {
	t.Setenv("WORKER_MODE", "standby")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("REDIS_URL", "")
	t.Setenv("GSC_CLIENT_ID", "")
	t.Setenv("GSC_CLIENT_SECRET", "")
	t.Setenv("GITHUB_APP_ID", "")
	t.Setenv("GITHUB_INSTALLATION_ID", "")
	t.Setenv("SCAN_QPM", "")

	cfg, err := Load()

	require.Nil(t, cfg)
	require.Error(t, err)
	errStr := err.Error()
	require.Contains(t, errStr, "missing required env")
	requiredKeys := []string{
		"DATABASE_URL",
		"REDIS_URL",
		"GSC_CLIENT_ID",
		"GSC_CLIENT_SECRET",
		"GITHUB_APP_ID",
		"GITHUB_INSTALLATION_ID",
	}
	for _, key := range requiredKeys {
		require.Contains(t, errStr, key)
	}
}

func setOneshotEnv(t *testing.T) {
	t.Setenv("WORKER_MODE", "oneshot")
	t.Setenv("SCAN_PROPERTY", "sc-domain:example.com")
	t.Setenv("SCAN_REPO", "nodeops/seo-workflow")
	t.Setenv("SCAN_SITEMAP_URL", "https://example.com/sitemap.xml")
	t.Setenv("GSC_ACCESS_TOKEN", "gsc-token")
	t.Setenv("WORKER_DRY_RUN", "true")
}

func TestLoadOneshotModeSuccessDefaults(t *testing.T) {
	setOneshotEnv(t)

	cfg, err := Load()

	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, "oneshot", cfg.WorkerMode)
	require.Equal(t, "sc-domain:example.com", cfg.ScanProperty)
	require.Equal(t, "nodeops/seo-workflow", cfg.ScanRepo)
	require.Equal(t, "https://example.com/sitemap.xml", cfg.ScanSitemapURL)
	require.Equal(t, "gsc-token", cfg.GSCAccessToken)
	require.True(t, cfg.DryRun)
	require.Equal(t, 7, cfg.LookbackDays)
	require.Equal(t, 1000, cfg.RowLimit)
	require.Equal(t, 30, cfg.HTTPTimeoutSecs)
	require.Equal(t, "public/sitemap.xml", cfg.GitHubSitemapPath)
}

func TestLoadOneshotRequiresGitHubTokenWhenNotDryRun(t *testing.T) {
	setOneshotEnv(t)
	t.Setenv("WORKER_DRY_RUN", "false")
	t.Setenv("GITHUB_TOKEN", "")

	cfg, err := Load()

	require.Nil(t, cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "GITHUB_TOKEN")
}

func TestLoadOneshotRejectsInvalidWorkerMode(t *testing.T) {
	t.Setenv("WORKER_MODE", "invalid")

	cfg, err := Load()

	require.Nil(t, cfg)
	require.EqualError(t, err, "invalid WORKER_MODE: \"invalid\"")
}

func TestLoadSuccessWithScanQPM(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("SCAN_QPM", "777")

	cfg, err := Load()

	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, "postgres://db", cfg.DatabaseURL)
	require.Equal(t, "redis://cache", cfg.RedisURL)
	require.Equal(t, "client-id", cfg.GSCClientID)
	require.Equal(t, "client-secret", cfg.GSCClientSecret)
	require.Equal(t, "123", cfg.GitHubAppID)
	require.Equal(t, "456", cfg.GitHubInstallationID)
	require.Equal(t, 777, cfg.ScanQPM)
}

func TestLoadDefaultScanQPMWhenUnset(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("SCAN_QPM", "")

	cfg, err := Load()

	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, 600, cfg.ScanQPM)
}

func TestLoadInvalidScanQPM(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("SCAN_QPM", "not-an-int")

	cfg, err := Load()

	require.Nil(t, cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid SCAN_QPM")
}

func TestLoadRejectsZeroScanQPM(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("SCAN_QPM", "0")

	cfg, err := Load()

	require.Nil(t, cfg)
	require.Error(t, err)
	require.Equal(t, "invalid SCAN_QPM: must be > 0", err.Error())
}

func TestLoadRejectsNegativeScanQPM(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("SCAN_QPM", "-5")

	cfg, err := Load()

	require.Nil(t, cfg)
	require.Error(t, err)
	require.Equal(t, "invalid SCAN_QPM: must be > 0", err.Error())
}

func setCompetitorEnv(t *testing.T) {
	t.Setenv("WORKER_MODE", "oneshot-competitor")
	t.Setenv("OUR_SITEMAP_URL", "https://createos.sh/sitemap.xml")
}

func TestLoadCompetitorModeSuccessDefaults(t *testing.T) {
	setCompetitorEnv(t)

	cfg, err := Load()

	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, "oneshot-competitor", cfg.WorkerMode)
	require.Equal(t, "https://createos.sh/sitemap.xml", cfg.OurSitemapURL)
	require.Equal(t, 30, cfg.CompetitorWindowDays)
	require.Equal(t, 30, cfg.HTTPTimeoutSecs)
	require.Equal(t, "moonshotai/kimi-k2", cfg.OpenRouterModel)
	require.Equal(t, 240, cfg.OpenRouterDraftTimeoutSecs)
	require.Equal(t, 1, cfg.CompetitorContentDraftLimit)
}

func TestLoadCompetitorModeAllowsOverrides(t *testing.T) {
	setCompetitorEnv(t)
	t.Setenv("OPENROUTER_API_KEY", "sk-or-test")
	t.Setenv("OPENROUTER_MODEL", "moonshotai/kimi-k2")
	t.Setenv("OPENROUTER_DRAFT_MODEL", "qwen/qwen3.6-flash")
	t.Setenv("COMPETITOR_REPORT_PATH", "competitor-report.json")
	t.Setenv("COMPETITOR_WINDOW_DAYS", "14")
	t.Setenv("COMPETITOR_HTTP_TIMEOUT_SEC", "45")
	t.Setenv("OPENROUTER_DRAFT_TIMEOUT_SEC", "360")
	t.Setenv("COMPETITOR_CONTENT_DRAFT_LIMIT", "1")
	t.Setenv("NOTION_API_KEY", "ntn_test")
	t.Setenv("NOTION_COMPETITOR_REPORT_PARENT_PAGE_ID", "1234567890abcdef1234567890abcdef")

	cfg, err := Load()

	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, "sk-or-test", cfg.OpenRouterAPIKey)
	require.Equal(t, "moonshotai/kimi-k2", cfg.OpenRouterModel)
	require.Equal(t, "qwen/qwen3.6-flash", cfg.OpenRouterDraftModel)
	require.Equal(t, "competitor-report.json", cfg.CompetitorReportPath)
	require.Equal(t, 14, cfg.CompetitorWindowDays)
	require.Equal(t, 45, cfg.HTTPTimeoutSecs)
	require.Equal(t, 360, cfg.OpenRouterDraftTimeoutSecs)
	require.Equal(t, 1, cfg.CompetitorContentDraftLimit)
	require.Equal(t, "ntn_test", cfg.NotionAPIKey)
	require.Equal(t, "1234567890abcdef1234567890abcdef", cfg.NotionCompetitorReportParentPageID)
}

func TestLoadCompetitorModeMissingRequiredEnv(t *testing.T) {
	t.Setenv("WORKER_MODE", "oneshot-competitor")
	t.Setenv("OUR_SITEMAP_URL", "")

	cfg, err := Load()

	require.Nil(t, cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "OUR_SITEMAP_URL")
}
