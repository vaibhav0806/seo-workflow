package competitor

import (
	"net/url"
	"sort"
	"strings"
)

var internalLinkHighPriorityPaths = map[string]int{
	"/":               72,
	"/blogs":          66,
	"/services":       70,
	"/case-studies":   68,
	"/industries":     64,
	"/about":          52,
	"/amazon-sellers": 60,
	"/schools":        60,
}

func buildCreateOSInternalLinkInventory(entries []rawSitemapEntry, recommendations []ContentRecommendation) []InternalLinkCandidate {
	queryText := contentRecommendationQueryText(recommendations)
	candidates := make([]InternalLinkCandidate, 0, len(entries))
	seen := map[string]struct{}{}
	for _, entry := range entries {
		parsed, err := url.Parse(strings.TrimSpace(entry.URL))
		if err != nil || parsed.Host != "createos.sh" {
			continue
		}
		path := normalizeInternalPath(parsed.Path)
		if path == "" {
			continue
		}
		if _, exists := seen[path]; exists {
			continue
		}
		seen[path] = struct{}{}
		candidate := createOSInternalLinkCandidate(path, queryText)
		if candidate.Score <= 0 {
			continue
		}
		candidates = append(candidates, candidate)
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Score == candidates[j].Score {
			return candidates[i].Path < candidates[j].Path
		}
		return candidates[i].Score > candidates[j].Score
	})
	return candidates
}

func createOSInternalLinkCandidate(path string, queryText string) InternalLinkCandidate {
	pageType := createOSInternalPageType(path)
	category := createOSInternalCategory(path)
	score := 0
	switch pageType {
	case "homepage":
		score = 72
	case "top-level":
		score = internalLinkHighPriorityPaths[path]
	case "blog":
		score = 58
	case "case-study":
		score = 64
	case "school":
		score = 58
	default:
		score = 20
	}
	if score == 0 {
		score = 45
	}
	switch category {
	case "core-positioning-workspace", "developer-productivity-tool-sprawl", "deployment-cli-devops", "agents-ai-vibecoding":
		score += 18
	case "monetization-marketplace-api", "integrations-partnerships", "case-study-industry", "security-risk":
		score += 12
	case "company-news-milestones":
		score -= 6
	case "penalized-nodeops-web3":
		score -= 40
	}
	score += internalLinkKeywordOverlapScore(path, queryText)
	score = clampImpact(score)
	return InternalLinkCandidate{
		URL:      "https://createos.sh" + path,
		Path:     path,
		Title:    titleFromPath(path),
		PageType: pageType,
		Category: category,
		Score:    score,
		Reason:   internalLinkReason(pageType, category),
	}
}

func selectInternalLinkCandidatesForRecommendation(recommendation ContentRecommendation, inventory []InternalLinkCandidate, limit int) []InternalLinkCandidate {
	if limit <= 0 || len(inventory) == 0 {
		return nil
	}
	queryText := strings.ToLower(strings.Join([]string{
		recommendation.SuggestedTitle,
		recommendation.SuggestedSlug,
		recommendation.Theme,
		recommendation.PageType,
		recommendation.TargetIntent,
		recommendation.ContentAngle,
		recommendation.Pillar,
	}, " "))
	scored := make([]InternalLinkCandidate, 0, len(inventory))
	for _, candidate := range inventory {
		item := candidate
		item.Score = clampImpact(candidate.Score + internalLinkKeywordOverlapScore(candidate.Path+" "+candidate.Title+" "+candidate.Category, queryText))
		if item.Score < 45 {
			continue
		}
		scored = append(scored, item)
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].Score == scored[j].Score {
			return scored[i].Path < scored[j].Path
		}
		return scored[i].Score > scored[j].Score
	})
	if len(scored) > limit {
		scored = scored[:limit]
	}
	return scored
}

func contentRecommendationQueryText(recommendations []ContentRecommendation) string {
	parts := make([]string, 0, len(recommendations)*6)
	for _, recommendation := range recommendations {
		parts = append(parts,
			recommendation.SuggestedTitle,
			recommendation.SuggestedSlug,
			recommendation.Theme,
			recommendation.PageType,
			recommendation.TargetIntent,
			recommendation.ContentAngle,
			recommendation.Pillar,
		)
	}
	return strings.ToLower(strings.Join(parts, " "))
}

func normalizeInternalPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || path == "/" {
		return "/"
	}
	path = "/" + strings.Trim(path, "/")
	return path
}

func createOSInternalPageType(path string) string {
	switch {
	case path == "/":
		return "homepage"
	case strings.HasPrefix(path, "/blogs/"):
		return "blog"
	case strings.HasPrefix(path, "/case-studies/"):
		return "case-study"
	case strings.HasPrefix(path, "/schools/"):
		return "school"
	case strings.Count(strings.Trim(path, "/"), "/") == 0:
		return "top-level"
	default:
		return "other"
	}
}

func createOSInternalCategory(path string) string {
	slug := strings.ToLower(strings.TrimPrefix(path, "/blogs/"))
	switch {
	case !strings.HasPrefix(path, "/blogs/"):
		if strings.HasPrefix(path, "/case-studies/") {
			return "case-study"
		}
		return "site-page"
	case containsAny(slug, "workspace", "unified", "execution-environment", "ecosystem", "build-it-now", "dashboard", "developer-focus"):
		return "core-positioning-workspace"
	case containsAny(slug, "context-switching", "fragmented", "tool-sprawl", "hidden-glue", "shadow-ops", "developer-productivity", "workflow"):
		return "developer-productivity-tool-sprawl"
	case containsAny(slug, "deploy", "deployment", "cli", "cron", "env-vars", "terminal", "api-in-2-minutes"):
		return "deployment-cli-devops"
	case containsAny(slug, "agent", "agentic", "ai-agents", "vibe-coder", "vibe"):
		return "agents-ai-vibecoding"
	case containsAny(slug, "monetized", "marketplace", "earn", "api-skills", "pricing", "capabilities"):
		return "monetization-marketplace-api"
	case containsAny(slug, "supabase", "google-workspace", "fluence", "localhost", "bnb-chain", "partners", "partnership"):
		return "integrations-partnerships"
	case containsAny(slug, "case-study", "industrial", "textile"):
		return "case-study-industry"
	case containsAny(slug, "breach", "security", "supply-chain"):
		return "security-risk"
	case containsAny(slug, "live", "milestones", "innovation-lab", "scaler"):
		return "company-news-milestones"
	case containsAny(slug, "staking", "token", "node-sale", "node-sale", "validator", "depin", "airdrop", "node-operator", "nodeops-network", "mint-and-burn"):
		return "penalized-nodeops-web3"
	default:
		return "blog-other"
	}
}

func internalLinkKeywordOverlapScore(text string, queryText string) int {
	tokens := filteredTokens(strings.ToLower(text))
	queryTokens := tokenSet(filteredTokens(strings.ToLower(queryText)))
	score := 0
	for _, token := range tokens {
		if _, ok := queryTokens[token]; ok {
			score += 8
		}
	}
	if score > 32 {
		return 32
	}
	return score
}

func tokenSet(tokens []string) map[string]struct{} {
	out := make(map[string]struct{}, len(tokens))
	for _, token := range tokens {
		out[token] = struct{}{}
	}
	return out
}

func titleFromPath(path string) string {
	if path == "/" {
		return "CreateOS"
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	slug := parts[len(parts)-1]
	words := strings.Split(strings.ReplaceAll(slug, "-", " "), " ")
	for idx, word := range words {
		words[idx] = titleWord(word)
	}
	return strings.Join(words, " ")
}

func internalLinkReason(pageType string, category string) string {
	if category != "" && category != "site-page" {
		return "Matched CreateOS sitemap category: " + category
	}
	return "Relevant CreateOS sitemap page type: " + pageType
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}
