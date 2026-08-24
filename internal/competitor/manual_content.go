package competitor

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nodeops/seo-workflow/internal/config"
)

const manualContentOpportunityPrefix = "Manual content request:"

func ManualContentSummary(cfg *config.Config) (Summary, error) {
	recommendation, ok := manualContentRecommendationFromConfig(cfg)
	if !ok {
		return Summary{}, fmt.Errorf("CONTENT_MANUAL_TITLE is required")
	}
	return Summary{
		GeneratedAtUTC:  time.Now().UTC().Format(time.RFC3339),
		ContentPlan:     []ContentRecommendation{recommendation},
		OpenRouterModel: strings.TrimSpace(cfg.OpenRouterModel),
	}, nil
}

func RunManualContent(ctx context.Context, cfg *config.Config) (Summary, error) {
	summary, err := ManualContentSummary(cfg)
	if err != nil {
		return Summary{}, err
	}
	summary.ContentPlan, summary.InventoryReport, summary.Warnings = applyConfiguredInventory(ctx, cfg, summary.ContentPlan)
	if len(summary.ContentPlan) == 0 || summary.ContentPlan[0].Decision != ContentDecisionCreate {
		summary.RefreshQueue = buildRefreshRecommendations(summary.ContentPlan)
		return summary, nil
	}

	draftModel := strings.TrimSpace(cfg.OpenRouterDraftModel)
	if draftModel == "" {
		draftModel = cfg.OpenRouterModel
	}
	createOSContext, contextErr := readGuidanceFile(createOSContextPath)
	if contextErr != nil {
		summary.Warnings = append(summary.Warnings, fmt.Sprintf("createos context skipped: %v", contextErr))
	}
	guidelines, guidelinesErr := readGuidanceFile(createOSWritingGuidesPath)
	if guidelinesErr != nil {
		summary.Warnings = append(summary.Warnings, fmt.Sprintf("createos writing guidelines skipped: %v", guidelinesErr))
	}
	drafts, draftErr := generateContentDraftsWithOpenRouter(ctx, cfg.OpenRouterAPIKey, draftModel, cfg.OpenRouterDraftFallbackModel, summary.ContentPlan, 1, createOSContext, guidelines, cfg.OpenRouterDraftTimeoutSecs, nil)
	if draftErr != nil {
		return Summary{}, fmt.Errorf("generate manual content draft: %w", draftErr)
	}
	summary.ContentPlan = attachDraftsToContentRecommendations(summary.ContentPlan, drafts, 1)
	return summary, nil
}

func IsManualContentRecommendation(recommendation ContentRecommendation) bool {
	return strings.HasPrefix(strings.TrimSpace(recommendation.Opportunity), manualContentOpportunityPrefix)
}

func manualContentRecommendationFromConfig(cfg *config.Config) (ContentRecommendation, bool) {
	if cfg == nil {
		return ContentRecommendation{}, false
	}
	title := strings.TrimSpace(cfg.ManualContentTitle)
	if title == "" {
		return ContentRecommendation{}, false
	}

	theme := manualContentTheme(cfg.ManualContentTheme, title)
	competitor := strings.TrimSpace(cfg.ManualContentCompetitor)
	if competitor == "" {
		competitor = "manual"
	}
	angle := strings.TrimSpace(cfg.ManualContentAngle)
	if angle == "" {
		angle = "Turn the manually requested topic into a CreateOS-specific article with concrete buyer questions, competitor context, internal links, and product-led proof."
	}

	return ContentRecommendation{
		Priority:       1,
		Opportunity:    fmt.Sprintf("%s %s", manualContentOpportunityPrefix, title),
		Competitor:     competitor,
		Theme:          theme,
		PageType:       pageTypeForTheme(theme),
		SuggestedSlug:  manualContentSlug(cfg.ManualContentSlug, title),
		SuggestedTitle: title,
		TargetIntent:   intentForTheme(theme),
		ContentAngle:   angle,
		Pillar:         pillarForTheme(theme),
		PrimaryKeyword: strings.Join(filteredTokens(title), " "),
		SecondaryKeywords: secondaryKeywordsForRecommendation(
			strings.Join(filteredTokens(title), " "), theme, competitor,
		),
		ClusterPages:   clusterPagesForOpportunity(title, theme, competitor),
		SourceEvidence: limitStrings(cfg.ManualContentEvidenceURLs, 8),
	}, true
}

func manualContentTheme(rawTheme string, title string) string {
	theme := strings.ToLower(strings.TrimSpace(rawTheme))
	if theme != "" {
		return theme
	}
	return llmTopicTheme(title)
}

func manualContentSlug(rawSlug string, title string) string {
	slug := strings.TrimSpace(rawSlug)
	if slug == "" {
		return "/blogs/" + safeSlug(title)
	}
	slug = strings.Trim(slug, "/")
	if slug == "" {
		return "/blogs/" + safeSlug(title)
	}
	if idx := strings.LastIndex(slug, "/"); idx >= 0 {
		slug = slug[idx+1:]
	}
	slug = tokenSplitPattern.ReplaceAllString(strings.ToLower(slug), "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = safeSlug(title)
	}
	return "/blogs/" + slug
}

func prependContentRecommendation(manual ContentRecommendation, recommendations []ContentRecommendation) []ContentRecommendation {
	out := make([]ContentRecommendation, 0, len(recommendations)+1)
	out = append(out, manual)
	out = append(out, recommendations...)
	for idx := range out {
		out[idx].Priority = idx + 1
	}
	return out
}

func enrichManualContentEvidence(manual ContentRecommendation, opportunities []Opportunity, topics []TopicSummary, snapshots []SiteSnapshot) ContentRecommendation {
	evidence := make([]string, 0, 8)
	seen := map[string]struct{}{}
	add := func(values ...string) {
		for _, value := range values {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			if _, exists := seen[value]; exists {
				continue
			}
			seen[value] = struct{}{}
			evidence = append(evidence, value)
			if len(evidence) >= 8 {
				return
			}
		}
	}

	add(manual.SourceEvidence...)
	if len(evidence) >= 8 {
		manual.SourceEvidence = evidence
		return manual
	}

	for _, opportunity := range opportunities {
		if !manualEvidenceRelevant(manual, opportunity.Competitor, opportunity.Theme, opportunity.Title) {
			continue
		}
		add(opportunity.Evidence...)
		if len(evidence) >= 8 {
			manual.SourceEvidence = evidence
			return manual
		}
	}
	for _, topic := range topics {
		if !manualEvidenceRelevant(manual, topic.Competitor, llmTopicTheme(topic.Name), topic.Name) {
			continue
		}
		add(topic.EvidenceURLs...)
		if len(evidence) >= 8 {
			manual.SourceEvidence = evidence
			return manual
		}
	}
	for _, snapshot := range snapshots {
		for _, entry := range snapshot.RecentURLs {
			if !manualEvidenceRelevant(manual, snapshot.Name, "", entry.Title+" "+entry.URL) {
				continue
			}
			add(entry.URL)
			if len(evidence) >= 8 {
				manual.SourceEvidence = evidence
				return manual
			}
		}
	}

	manual.SourceEvidence = evidence
	return manual
}

func manualEvidenceRelevant(manual ContentRecommendation, competitor string, theme string, text string) bool {
	if manual.Competitor != "" && manual.Competitor != "manual" && strings.EqualFold(manual.Competitor, competitor) {
		return true
	}
	if manual.Theme != "" && theme != "" && strings.EqualFold(manual.Theme, theme) {
		return true
	}
	manualTokens := filteredTokens(manual.SuggestedTitle + " " + manual.Theme)
	textTokens := filteredTokens(text)
	if len(manualTokens) == 0 || len(textTokens) == 0 {
		return false
	}
	textSet := make(map[string]struct{}, len(textTokens))
	for _, token := range textTokens {
		textSet[token] = struct{}{}
	}
	for _, token := range manualTokens {
		if len(token) < 3 {
			continue
		}
		if _, exists := textSet[token]; exists {
			return true
		}
	}
	return false
}
