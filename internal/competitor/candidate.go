package competitor

import (
	"net/url"
	"sort"
	"strings"
)

const promptCandidateThreshold = 55

type PageCandidate struct {
	Competitor      string   `json:"competitor,omitempty"`
	URL             string   `json:"url"`
	Title           string   `json:"title,omitempty"`
	PageType        string   `json:"pageType"`
	RelevanceScore  int      `json:"relevanceScore"`
	PositiveSignals []string `json:"positiveSignals,omitempty"`
	NegativeSignals []string `json:"negativeSignals,omitempty"`
	WhySelected     []string `json:"whySelected,omitempty"`
}

func buildPageCandidates(snapshot SiteSnapshot) []PageCandidate {
	candidates := make([]PageCandidate, 0, len(snapshot.RecentURLs))
	for _, entry := range snapshot.RecentURLs {
		candidates = append(candidates, pageCandidate(snapshot.Name, entry))
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].RelevanceScore == candidates[j].RelevanceScore {
			return candidates[i].URL < candidates[j].URL
		}
		return candidates[i].RelevanceScore > candidates[j].RelevanceScore
	})
	return candidates
}

func pageCandidate(competitor string, entry SitemapEntry) PageCandidate {
	rawURL := strings.TrimSpace(entry.URL)
	title := strings.TrimSpace(entry.Title)
	pageType := classifyPageType(competitor, rawURL, title)
	score := 35
	positive := make([]string, 0)
	negative := make([]string, 0)

	switch pageType {
	case "comparison":
		score += 38
		positive = append(positive, "comparison")
	case "usecase":
		score += 32
		positive = append(positive, "usecase")
	case "guide":
		score += 28
		positive = append(positive, "guide")
	case "solution":
		score += 26
		positive = append(positive, "solution")
	case "integration":
		score += 24
		positive = append(positive, "integration")
	case "enterprise":
		score += 22
		positive = append(positive, "enterprise")
	case "security":
		score += 22
		positive = append(positive, "security")
	case "article":
		score += 20
		positive = append(positive, "article")
	case "product":
		score += 18
		positive = append(positive, "product")
	case "homepage":
		score += 4
		negative = append(negative, "homepage")
	case "index", "legal", "career", "status":
		score -= 45
		negative = append(negative, "low-value-path")
	default:
		score += 5
	}

	pathText := candidatePathText(rawURL)
	text := strings.ToLower(pathText + " " + title)
	for _, signal := range []struct {
		token string
		score int
		name  string
	}{
		{"ai", 8, "ai"},
		{"llm", 8, "ai"},
		{"agent", 8, "agents"},
		{"mvp", 7, "mvp"},
		{"prototype", 7, "prototype"},
		{"alternative", 7, "alternative"},
		{" vs ", 7, "comparison"},
		{"enterprise", 6, "enterprise"},
		{"workflow", 6, "workflow"},
		{"security", 6, "security"},
		{"partner", 5, "partner"},
		{"connect", 5, "integration"},
	} {
		if strings.Contains(text, signal.token) {
			score += signal.score
			positive = appendUnique(positive, signal.name)
		}
	}

	if title == "" {
		score -= 30
		negative = append(negative, "missing-title")
	}
	if isLowValuePromptTitle(title) {
		score -= 45
		negative = append(negative, "low-value-title")
	}
	if isGenericTitle(title) {
		score -= 18
		negative = append(negative, "generic-title")
	}

	score = clampImpact(score)
	return PageCandidate{
		Competitor:      competitor,
		URL:             rawURL,
		Title:           title,
		PageType:        pageType,
		RelevanceScore:  score,
		PositiveSignals: positive,
		NegativeSignals: negative,
		WhySelected:     positive,
	}
}

func candidatePathText(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return strings.ToLower(rawURL)
	}
	return strings.ToLower(strings.ReplaceAll(parsed.Path, "-", " "))
}

func classifyPageType(competitor string, rawURL string, title string) string {
	if isLowValuePromptPath(competitor, rawURL) {
		if strings.Contains(strings.ToLower(rawURL), "career") || strings.Contains(strings.ToLower(title), "career") {
			return "career"
		}
		if strings.Contains(strings.ToLower(rawURL), "status") || strings.Contains(strings.ToLower(title), "dashboard") {
			return "status"
		}
		return "legal"
	}
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	path := ""
	if err == nil {
		path = strings.ToLower(strings.Trim(parsed.Path, "/"))
	}
	titleLower := strings.ToLower(title)
	if path == "" {
		return "homepage"
	}
	segments := strings.Split(path, "/")
	first := segments[0]
	if strings.Contains(path, "-vs-") || strings.Contains(path, "/comparisons/") || strings.Contains(titleLower, " vs ") || strings.Contains(titleLower, "alternatives") || strings.Contains(titleLower, "compared") {
		return "comparison"
	}
	if first == "usecases" || strings.Contains(titleLower, "use case") {
		return "usecase"
	}
	if first == "guides" || first == "guide" || first == "kb" {
		return "guide"
	}
	if first == "solutions" || strings.Contains(titleLower, "solution") {
		return "solution"
	}
	if first == "partners" || first == "connect" || strings.Contains(titleLower, "connector") || strings.Contains(titleLower, "integration") {
		return "integration"
	}
	if strings.Contains(path, "enterprise") || strings.Contains(titleLower, "enterprise") {
		return "enterprise"
	}
	if strings.Contains(path, "security") || strings.Contains(titleLower, "security") || strings.Contains(titleLower, "vulnerabilit") {
		return "security"
	}
	if first == "blog" || first == "blogs" || first == "i" {
		return "article"
	}
	if strings.Contains(path, "ai-gateway") || strings.Contains(titleLower, "app builder") || strings.Contains(titleLower, "landing pages") {
		return "product"
	}
	return "landing"
}

func isGenericTitle(title string) bool {
	switch strings.ToLower(strings.TrimSpace(title)) {
	case "build", "discover", "dashboard", "error codes", "blog", "guides":
		return true
	default:
		return false
	}
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
