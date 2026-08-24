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
	if strings.TrimSpace(cfg.GitHubToken) == "" {
		return fmt.Errorf("GITHUB_TOKEN is required to create content pull request")
	}

	generatedAt := time.Now().UTC()
	if parsed, err := time.Parse(time.RFC3339, summary.GeneratedAtUTC); err == nil {
		generatedAt = parsed.UTC()
	}
	publisher := contentrepo.NewGitHubPublisher(cfg.GitHubToken, cfg.ContentRepo, cfg.ContentBaseBranch, cfg.ContentReviewer)

	var duplicateErr error
	for _, recommendation := range draftRecommendations(summary.ContentPlan) {
		post, err := contentrepo.BuildBlogPost(recommendation, generatedAt, cfg.ContentAuthor, cfg.ContentCoverURL)
		if err != nil {
			return err
		}
		if err := publisher.ValidateCanPublish(ctx, post); err != nil {
			if isDuplicateContentError(err) {
				duplicateErr = err
				log.Printf("competitor content pull request candidate skipped: %v", err)
				continue
			}
			return err
		}
		return publishCompetitorContentPost(ctx, cfg, publisher, post)
	}

	if duplicateErr != nil {
		return duplicateErr
	}
	log.Printf("competitor content pull request skipped: no generated blog draft")
	return nil
}

func publishCompetitorContentPost(ctx context.Context, cfg *config.Config, publisher *contentrepo.GitHubPublisher, post contentrepo.BlogPost) error {
	coverUploader := newCoverUploaderFromConfig(cfg)
	coverAssets := []contentrepo.CoverAsset{}
	if !shouldGenerateCover(cfg, coverUploader) {
		log.Printf("competitor cover image generation skipped: configure Cloudinary credentials or CONTENT_COVER_ASSET_BASE_URL")
	} else {
		cover, coverErr := contentrepo.GenerateDesignSystemCover(ctx, post, cfg.ContentCoverAssetBaseURL)
		if coverErr != nil {
			log.Printf("competitor cover image generation skipped: %v", coverErr)
		} else {
			coverAssets = applyGeneratedCover(ctx, &post, cover, coverUploader)
		}
	}

	riskReport := contentrepo.AssessSEORisk(post)
	titlePrefix := ""
	if riskReport.Risky {
		titlePrefix = "RISKY:"
	}
	result, err := publisher.PublishWithOptions(ctx, post, contentrepo.PublishOptions{
		SourceReportPath:    cfg.CompetitorReportPath,
		TitlePrefix:         titlePrefix,
		PRBodyExtraMarkdown: riskReport.Markdown(),
	}, coverAssets...)
	if err != nil {
		return err
	}
	log.Printf("competitor content pull request created: url=%q branch=%q file=%q", result.PullRequestURL, result.Branch, result.FilePath)
	for _, warning := range result.Warnings {
		log.Printf("competitor content pull request warning: %s", warning)
	}
	if riskReport.Risky {
		mitigatedPost := contentrepo.MitigateSEORisk(post, riskReport)
		mitigatedResult, mitigationErr := publisher.PublishWithOptions(ctx, mitigatedPost, contentrepo.PublishOptions{
			SourceReportPath:      cfg.CompetitorReportPath,
			TitlePrefix:           "MITIGATED:",
			RelatedPullRequestURL: result.PullRequestURL,
			PRBodyExtraMarkdown:   mitigatedPRBody(riskReport),
		})
		if mitigationErr != nil {
			return fmt.Errorf("create mitigated content pull request: %w", mitigationErr)
		}
		log.Printf("mitigated competitor content pull request created: url=%q branch=%q file=%q", mitigatedResult.PullRequestURL, mitigatedResult.Branch, mitigatedResult.FilePath)
		for _, warning := range mitigatedResult.Warnings {
			log.Printf("mitigated competitor content pull request warning: %s", warning)
		}
	}
	return nil
}

func mitigatedPRBody(report contentrepo.SEORiskReport) string {
	return strings.Join([]string{
		"## Mitigated SEO Draft",
		"",
		"This companion PR was created because the original generated draft was marked risky. Merge either the original after human edits or this mitigated version, not both.",
		"",
		report.Markdown(),
	}, "\n")
}

func isDuplicateContentError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "content file already exists on ") ||
		strings.Contains(err.Error(), "content file already exists in open PR #")
}

func newCoverUploaderFromConfig(cfg *config.Config) contentrepo.CoverUploader {
	if cfg == nil {
		return nil
	}
	cloudName := strings.TrimSpace(cfg.CloudinaryCloudName)
	apiKey := strings.TrimSpace(cfg.CloudinaryAPIKey)
	apiSecret := strings.TrimSpace(cfg.CloudinaryAPISecret)
	if cloudName == "" || apiKey == "" || apiSecret == "" {
		return nil
	}
	return contentrepo.NewCloudinaryCoverUploader(cloudName, apiKey, apiSecret, cfg.CloudinaryUploadFolder)
}

func shouldGenerateCover(cfg *config.Config, uploader contentrepo.CoverUploader) bool {
	if cfg == nil {
		return false
	}
	return uploader != nil || strings.TrimSpace(cfg.ContentCoverAssetBaseURL) != ""
}

func applyGeneratedCover(ctx context.Context, post *contentrepo.BlogPost, cover contentrepo.GeneratedCover, uploader contentrepo.CoverUploader) []contentrepo.CoverAsset {
	if post == nil {
		return nil
	}

	asset := cover.Asset
	if uploader != nil && len(asset.Content) > 0 {
		result, err := uploader.UploadCover(ctx, contentrepo.CoverUploadAsset{
			Path:        asset.Path,
			Content:     asset.Content,
			ContentType: contentTypeFromAssetPath(asset.Path),
			Slug:        post.Slug,
		})
		if err != nil {
			log.Printf("competitor cover image upload skipped: path=%q error=%v", asset.Path, err)
			return nil
		}
		if strings.TrimSpace(result.URL) != "" {
			post.Cover = result.URL
			log.Printf("competitor cover image uploaded: path=%q url=%q", asset.Path, result.URL)
			return nil
		}
	}

	if strings.TrimSpace(cover.URL) != "" {
		post.Cover = cover.URL
		log.Printf("competitor cover image generated: path=%q url=%q", asset.Path, cover.URL)
	}
	if strings.TrimSpace(asset.Path) == "" || len(asset.Content) == 0 {
		return nil
	}
	return []contentrepo.CoverAsset{asset}
}

func contentTypeFromAssetPath(assetPath string) string {
	assetPath = strings.ToLower(strings.TrimSpace(assetPath))
	switch {
	case strings.HasSuffix(assetPath, ".jpg"), strings.HasSuffix(assetPath, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(assetPath, ".webp"):
		return "image/webp"
	case strings.HasSuffix(assetPath, ".png"):
		return "image/png"
	default:
		return ""
	}
}

func firstDraftRecommendation(recommendations []competitor.ContentRecommendation) (competitor.ContentRecommendation, bool) {
	drafts := draftRecommendations(recommendations)
	if len(drafts) == 0 {
		return competitor.ContentRecommendation{}, false
	}
	return drafts[0], true
}

func draftRecommendations(recommendations []competitor.ContentRecommendation) []competitor.ContentRecommendation {
	out := make([]competitor.ContentRecommendation, 0, len(recommendations))
	for _, recommendation := range recommendations {
		if recommendation.Draft == nil {
			continue
		}
		if strings.TrimSpace(recommendation.Draft.BodyMarkdown) == "" {
			continue
		}
		out = append(out, recommendation)
	}
	return out
}
