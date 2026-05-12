package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/nodeops/seo-workflow/internal/competitor"
	"github.com/nodeops/seo-workflow/internal/config"
	"github.com/nodeops/seo-workflow/internal/oneshot"
)

func main() {
	if err := loadDotEnv(".env"); err != nil {
		log.Fatalf("failed to load .env: %v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	if cfg.WorkerMode == "oneshot" {
		log.Printf(
			"oneshot workflow starting: property=%q repo=%q dry_run=%t qpm=%d",
			cfg.ScanProperty,
			cfg.ScanRepo,
			cfg.DryRun,
			cfg.ScanQPM,
		)

		summary, runErr := oneshot.Run(context.Background(), cfg)
		if runErr != nil {
			log.Fatalf("oneshot workflow failed: %v", runErr)
		}

		log.Printf(
			"oneshot workflow complete: total_urls=%d bucket_counts=%v pr_url=%q",
			summary.TotalURLs,
			summary.BucketCounts,
			summary.PullRequestURL,
		)
		logDetailedSummary(summary)
		if reportErr := writeOneshotReport(cfg, summary); reportErr != nil {
			log.Fatalf("failed to write oneshot report: %v", reportErr)
		}
		return
	}
	if cfg.WorkerMode == "oneshot-competitor" {
		log.Printf(
			"competitor oneshot starting: our_sitemap=%q window_days=%d",
			cfg.OurSitemapURL,
			cfg.CompetitorWindowDays,
		)

		summary, runErr := competitor.Run(context.Background(), cfg)
		if runErr != nil {
			log.Fatalf("competitor oneshot failed: %v", runErr)
		}
		logCompetitorSummary(summary)
		if reportErr := writeCompetitorReport(cfg, summary); reportErr != nil {
			log.Fatalf("failed to write competitor report: %v", reportErr)
		}
		if reportErr := writeCompetitorContentPullRequest(context.Background(), cfg, summary); reportErr != nil {
			log.Fatalf("failed to create competitor content pull request: %v", reportErr)
		}
		log.Printf("competitor oneshot complete")
		return
	}

	log.Printf("worker bootstrap complete (standby mode), configured qpm=%d", cfg.ScanQPM)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	sig := <-sigCh
	log.Printf("worker shutting down due to signal=%s", sig)
}
