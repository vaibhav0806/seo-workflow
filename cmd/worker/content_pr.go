package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/nodeops/seo-workflow/internal/competitor"
	"github.com/nodeops/seo-workflow/internal/config"
	"github.com/nodeops/seo-workflow/internal/contentrepo"
)

func writeCompetitorContentPullRequest(ctx context.Context, cfg *config.Config, summary competitor.Summary) error {
	recommendation, ok := firstDraftRecommendation(summary.ContentPlan)
	if !ok {
		log.Printf("competitor content pull request skipped: no generated blog draft")
		return nil
	}
	if strings.TrimSpace(cfg.GitHubToken) == "" {
		return fmt.Errorf("GITHUB_TOKEN is required to create content pull request")
	}

	generatedAt := time.Now().UTC()
	if parsed, err := time.Parse(time.RFC3339, summary.GeneratedAtUTC); err == nil {
		generatedAt = parsed.UTC()
	}
	post, err := contentrepo.BuildBlogPost(recommendation, generatedAt, cfg.ContentAuthor, cfg.ContentCoverURL)
	if err != nil {
		return err
	}

	publisher := contentrepo.NewGitHubPublisher(cfg.GitHubToken, cfg.ContentRepo, cfg.ContentBaseBranch, cfg.ContentReviewer)
	result, err := publisher.Publish(ctx, post, cfg.CompetitorReportPath)
	if err != nil {
		return err
	}
	log.Printf("competitor content pull request created: url=%q branch=%q file=%q", result.PullRequestURL, result.Branch, result.FilePath)
	return nil
}

func firstDraftRecommendation(recommendations []competitor.ContentRecommendation) (competitor.ContentRecommendation, bool) {
	for _, recommendation := range recommendations {
		if recommendation.Draft == nil {
			continue
		}
		if strings.TrimSpace(recommendation.Draft.BodyMarkdown) == "" {
			continue
		}
		return recommendation, true
	}
	return competitor.ContentRecommendation{}, false
}
