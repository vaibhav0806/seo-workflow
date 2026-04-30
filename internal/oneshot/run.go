package oneshot

import (
	"context"
	"fmt"

	"github.com/nodeops/seo-workflow/internal/config"
	"github.com/nodeops/seo-workflow/internal/ratelimit"
	"github.com/nodeops/seo-workflow/internal/scan"
)

func Run(ctx context.Context, cfg *config.Config) (scan.Summary, error) {
	if cfg == nil {
		return scan.Summary{}, fmt.Errorf("oneshot config is nil")
	}

	gscAdapter := NewGSCAdapter(
		cfg.GSCAccessToken,
		cfg.ScanSitemapURL,
		cfg.LookbackDays,
		cfg.RowLimit,
		cfg.HTTPTimeoutSecs,
	)

	waiter := ratelimit.NewPropertyLimiter(cfg.ScanQPM, 10)
	opener := NewGitHubOpener(
		cfg.GitHubToken,
		cfg.ScanRepo,
		cfg.GitHubBaseBranch,
		cfg.GitHubSitemapPath,
		cfg.DryRun,
	)

	service := scan.NewService(gscAdapter, gscAdapter, waiter, gscAdapter, opener)
	summary, err := service.RunPropertyScan(ctx, cfg.ScanProperty)
	if err != nil {
		return scan.Summary{}, err
	}
	return summary, nil
}
