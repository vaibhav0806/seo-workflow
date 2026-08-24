package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

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

		summary, runErr := competitor.RunDiscovery(context.Background(), cfg)
		if runErr != nil {
			log.Fatalf("competitor oneshot failed: %v", runErr)
		}
		logCompetitorSummary(summary)
		if reportErr := writeCompetitorReport(cfg, summary); reportErr != nil {
			log.Fatalf("failed to write competitor report: %v", reportErr)
		}
		generatedAt := time.Now().UTC()
		if parsed, parseErr := time.Parse(time.RFC3339, summary.GeneratedAtUTC); parseErr == nil {
			generatedAt = parsed
		}
		queue := competitor.BuildApprovalQueue(summary.ContentPlan, generatedAt)
		if queueErr := competitor.WriteApprovalQueue(cfg.ContentApprovalPath, queue); queueErr != nil {
			log.Fatalf("failed to write content approval queue: %v", queueErr)
		}
		log.Printf("competitor oneshot complete: approval_queue=%q candidates=%d", cfg.ContentApprovalPath, len(queue.Items))
		return
	}
	if cfg.WorkerMode == "approved-content" {
		log.Printf("approved content starting: approval_queue=%q", cfg.ContentApprovalPath)
		summary, runErr := competitor.RunApprovedContent(context.Background(), cfg)
		if runErr != nil {
			log.Fatalf("approved content workflow failed: %v", runErr)
		}
		if reportErr := writeCompetitorReport(cfg, summary); reportErr != nil {
			log.Fatalf("failed to write approved content report: %v", reportErr)
		}
		if publishErr := writeCompetitorContentPullRequest(context.Background(), cfg, summary); publishErr != nil {
			log.Fatalf("failed to create approved content pull request: %v", publishErr)
		}
		log.Printf("approved content complete")
		return
	}
	if cfg.WorkerMode == "manual-content" {
		log.Printf("manual content starting: title=%q", cfg.ManualContentTitle)
		summary, runErr := competitor.RunManualContent(context.Background(), cfg)
		if runErr != nil {
			log.Fatalf("manual content workflow failed: %v", runErr)
		}
		if reportErr := writeCompetitorReport(cfg, summary); reportErr != nil {
			log.Fatalf("failed to write manual content report: %v", reportErr)
		}
		if reportErr := writeCompetitorContentPullRequest(context.Background(), cfg, summary); reportErr != nil {
			log.Fatalf("failed to create manual content pull request: %v", reportErr)
		}
		log.Printf("manual content complete")
		return
	}

	log.Printf("worker bootstrap complete (standby mode), configured qpm=%d", cfg.ScanQPM)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	sig := <-sigCh
	log.Printf("worker shutting down due to signal=%s", sig)
}
