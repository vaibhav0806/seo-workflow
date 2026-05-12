package config

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

const defaultScanQPM = 600
const (
	defaultWorkerMode             = "standby"
	defaultDryRun                 = true
	defaultLookbackDays           = 7
	defaultRowLimit               = 1000
	defaultHTTPTimeoutSecs        = 30
	defaultSitemapPath            = "public/sitemap.xml"
	defaultContentRepo            = "NodeOps-app/createos-content"
	defaultContentBaseBranch      = "main"
	defaultContentAuthor          = "CreateOS"
	defaultContentReviewer        = "vaibhav0806"
	defaultContentCoverURL        = "https://cdn.hashnode.com/res/hashnode/image/upload/v1770132301745/89493e47-b967-46a6-9ff9-60c55aaaa3de.png"
	defaultCompetitorModel        = "moonshotai/kimi-k2"
	defaultWindowDays             = 30
	defaultCompetitorDraftLimit   = 1
	defaultOpenRouterTopicTimeout = 180
	defaultOpenRouterDraftTimeout = 240
)

type Config struct {
	WorkerMode string

	DatabaseURL          string
	RedisURL             string
	GSCClientID          string
	GSCClientSecret      string
	GitHubAppID          string
	GitHubInstallationID string
	ScanQPM              int

	ScanProperty      string
	ScanRepo          string
	ScanSitemapURL    string
	GSCAccessToken    string
	GitHubToken       string
	GitHubBaseBranch  string
	GitHubSitemapPath string
	OneshotReportPath string
	DryRun            bool
	LookbackDays      int
	RowLimit          int
	HTTPTimeoutSecs   int

	CompetitorReportPath               string
	CompetitorWindowDays               int
	OurSitemapURL                      string
	OpenRouterAPIKey                   string
	OpenRouterModel                    string
	OpenRouterFallbackModel            string
	OpenRouterDraftModel               string
	OpenRouterDraftFallbackModel       string
	OpenRouterTopicTimeoutSecs         int
	OpenRouterDraftTimeoutSecs         int
	CompetitorContentDraftLimit        int
	NotionAPIKey                       string
	NotionCompetitorReportParentPageID string
	ContentRepo                        string
	ContentBaseBranch                  string
	ContentAuthor                      string
	ContentReviewer                    string
	ContentCoverURL                    string
}

func Load() (*Config, error) {
	workerMode := strings.ToLower(strings.TrimSpace(os.Getenv("WORKER_MODE")))
	if workerMode == "" {
		workerMode = defaultWorkerMode
	}

	cfg := &Config{
		WorkerMode:                  workerMode,
		DatabaseURL:                 strings.TrimSpace(os.Getenv("DATABASE_URL")),
		RedisURL:                    strings.TrimSpace(os.Getenv("REDIS_URL")),
		GSCClientID:                 strings.TrimSpace(os.Getenv("GSC_CLIENT_ID")),
		GSCClientSecret:             strings.TrimSpace(os.Getenv("GSC_CLIENT_SECRET")),
		GitHubAppID:                 strings.TrimSpace(os.Getenv("GITHUB_APP_ID")),
		GitHubInstallationID:        strings.TrimSpace(os.Getenv("GITHUB_INSTALLATION_ID")),
		ScanQPM:                     defaultScanQPM,
		GitHubSitemapPath:           defaultSitemapPath,
		DryRun:                      defaultDryRun,
		LookbackDays:                defaultLookbackDays,
		RowLimit:                    defaultRowLimit,
		HTTPTimeoutSecs:             defaultHTTPTimeoutSecs,
		OpenRouterModel:             defaultCompetitorModel,
		ContentRepo:                 defaultContentRepo,
		ContentBaseBranch:           defaultContentBaseBranch,
		ContentAuthor:               defaultContentAuthor,
		ContentReviewer:             defaultContentReviewer,
		ContentCoverURL:             defaultContentCoverURL,
		CompetitorWindowDays:        defaultWindowDays,
		OpenRouterTopicTimeoutSecs:  defaultOpenRouterTopicTimeout,
		OpenRouterDraftTimeoutSecs:  defaultOpenRouterDraftTimeout,
		CompetitorContentDraftLimit: defaultCompetitorDraftLimit,
	}

	if scanQPMRaw := strings.TrimSpace(os.Getenv("SCAN_QPM")); scanQPMRaw != "" {
		scanQPM, err := strconv.Atoi(scanQPMRaw)
		if err != nil {
			return nil, fmt.Errorf("invalid SCAN_QPM: %w", err)
		}
		if scanQPM <= 0 {
			return nil, fmt.Errorf("invalid SCAN_QPM: must be > 0")
		}
		cfg.ScanQPM = scanQPM
	}

	switch cfg.WorkerMode {
	case "standby":
		missing := make([]string, 0, 6)
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
			sort.Strings(missing)
			return nil, fmt.Errorf("missing required env: %s", strings.Join(missing, ", "))
		}
	case "oneshot":
		cfg.ScanProperty = strings.TrimSpace(os.Getenv("SCAN_PROPERTY"))
		cfg.ScanRepo = strings.TrimSpace(os.Getenv("SCAN_REPO"))
		cfg.ScanSitemapURL = strings.TrimSpace(os.Getenv("SCAN_SITEMAP_URL"))
		cfg.GSCAccessToken = strings.TrimSpace(os.Getenv("GSC_ACCESS_TOKEN"))
		cfg.GitHubToken = strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
		cfg.GitHubBaseBranch = strings.TrimSpace(os.Getenv("GITHUB_BASE_BRANCH"))
		if overridePath := strings.TrimSpace(os.Getenv("GITHUB_SITEMAP_PATH")); overridePath != "" {
			cfg.GitHubSitemapPath = overridePath
		}
		cfg.OneshotReportPath = strings.TrimSpace(os.Getenv("ONESHOT_REPORT_PATH"))

		if rawDryRun := strings.TrimSpace(os.Getenv("WORKER_DRY_RUN")); rawDryRun != "" {
			parsedDryRun, err := strconv.ParseBool(rawDryRun)
			if err != nil {
				return nil, fmt.Errorf("invalid WORKER_DRY_RUN: %w", err)
			}
			cfg.DryRun = parsedDryRun
		}

		lookbackDays, err := parsePositiveIntEnv("GSC_LOOKBACK_DAYS", defaultLookbackDays)
		if err != nil {
			return nil, err
		}
		cfg.LookbackDays = lookbackDays

		rowLimit, err := parsePositiveIntEnv("GSC_ROW_LIMIT", defaultRowLimit)
		if err != nil {
			return nil, err
		}
		cfg.RowLimit = rowLimit

		timeoutSecs, err := parsePositiveIntEnv("GSC_HTTP_TIMEOUT_SEC", defaultHTTPTimeoutSecs)
		if err != nil {
			return nil, err
		}
		cfg.HTTPTimeoutSecs = timeoutSecs

		missing := make([]string, 0, 5)
		if cfg.ScanProperty == "" {
			missing = append(missing, "SCAN_PROPERTY")
		}
		if cfg.ScanRepo == "" {
			missing = append(missing, "SCAN_REPO")
		}
		if cfg.ScanSitemapURL == "" {
			missing = append(missing, "SCAN_SITEMAP_URL")
		}
		if cfg.GSCAccessToken == "" {
			missing = append(missing, "GSC_ACCESS_TOKEN")
		}
		if !cfg.DryRun && cfg.GitHubToken == "" {
			missing = append(missing, "GITHUB_TOKEN")
		}
		if len(missing) > 0 {
			sort.Strings(missing)
			return nil, fmt.Errorf("missing required env: %s", strings.Join(missing, ", "))
		}
	case "oneshot-competitor":
		cfg.OurSitemapURL = strings.TrimSpace(os.Getenv("OUR_SITEMAP_URL"))
		cfg.CompetitorReportPath = strings.TrimSpace(os.Getenv("COMPETITOR_REPORT_PATH"))
		cfg.OpenRouterAPIKey = strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY"))
		cfg.GitHubToken = strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
		cfg.NotionAPIKey = strings.TrimSpace(os.Getenv("NOTION_API_KEY"))
		cfg.NotionCompetitorReportParentPageID = strings.TrimSpace(os.Getenv("NOTION_COMPETITOR_REPORT_PARENT_PAGE_ID"))
		if contentRepo := strings.TrimSpace(os.Getenv("CONTENT_REPO")); contentRepo != "" {
			cfg.ContentRepo = contentRepo
		}
		if contentBaseBranch := strings.TrimSpace(os.Getenv("CONTENT_BASE_BRANCH")); contentBaseBranch != "" {
			cfg.ContentBaseBranch = contentBaseBranch
		}
		if contentAuthor := strings.TrimSpace(os.Getenv("CONTENT_AUTHOR")); contentAuthor != "" {
			cfg.ContentAuthor = contentAuthor
		}
		if contentReviewer := strings.TrimSpace(os.Getenv("CONTENT_REVIEWER_GITHUB_HANDLE")); contentReviewer != "" {
			cfg.ContentReviewer = contentReviewer
		}
		if contentCoverURL := strings.TrimSpace(os.Getenv("CONTENT_DEFAULT_COVER_URL")); contentCoverURL != "" {
			cfg.ContentCoverURL = contentCoverURL
		}
		if model := strings.TrimSpace(os.Getenv("OPENROUTER_MODEL")); model != "" {
			cfg.OpenRouterModel = model
		}
		cfg.OpenRouterFallbackModel = strings.TrimSpace(os.Getenv("OPENROUTER_FALLBACK_MODEL"))
		cfg.OpenRouterDraftModel = strings.TrimSpace(os.Getenv("OPENROUTER_DRAFT_MODEL"))
		cfg.OpenRouterDraftFallbackModel = strings.TrimSpace(os.Getenv("OPENROUTER_DRAFT_FALLBACK_MODEL"))
		topicTimeoutSecs, err := parsePositiveIntEnv("OPENROUTER_TOPIC_TIMEOUT_SEC", defaultOpenRouterTopicTimeout)
		if err != nil {
			return nil, err
		}
		cfg.OpenRouterTopicTimeoutSecs = topicTimeoutSecs
		draftTimeoutSecs, err := parsePositiveIntEnv("OPENROUTER_DRAFT_TIMEOUT_SEC", defaultOpenRouterDraftTimeout)
		if err != nil {
			return nil, err
		}
		cfg.OpenRouterDraftTimeoutSecs = draftTimeoutSecs
		draftLimit, err := parsePositiveIntEnv("COMPETITOR_CONTENT_DRAFT_LIMIT", defaultCompetitorDraftLimit)
		if err != nil {
			return nil, err
		}
		cfg.CompetitorContentDraftLimit = draftLimit
		windowDays, err := parsePositiveIntEnv("COMPETITOR_WINDOW_DAYS", defaultWindowDays)
		if err != nil {
			return nil, err
		}
		cfg.CompetitorWindowDays = windowDays
		timeoutSecs, err := parsePositiveIntEnv("COMPETITOR_HTTP_TIMEOUT_SEC", defaultHTTPTimeoutSecs)
		if err != nil {
			return nil, err
		}
		cfg.HTTPTimeoutSecs = timeoutSecs

		missing := make([]string, 0, 1)
		if cfg.OurSitemapURL == "" {
			missing = append(missing, "OUR_SITEMAP_URL")
		}
		if len(missing) > 0 {
			return nil, fmt.Errorf("missing required env: %s", strings.Join(missing, ", "))
		}
	default:
		return nil, fmt.Errorf("invalid WORKER_MODE: %q", cfg.WorkerMode)
	}

	return cfg, nil
}

func parsePositiveIntEnv(key string, defaultValue int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return defaultValue, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", key, err)
	}
	if value <= 0 {
		return 0, fmt.Errorf("invalid %s: must be > 0", key)
	}
	return value, nil
}
