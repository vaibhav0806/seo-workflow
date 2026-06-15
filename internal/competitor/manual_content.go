package competitor

import (
	"fmt"
	"strings"

	"github.com/nodeops/seo-workflow/internal/config"
)

const manualContentOpportunityPrefix = "Manual content request:"

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
	if strings.Contains(slug, "/") {
		return "/" + slug
	}
	return "/blogs/" + safeSlug(slug)
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
