package competitor

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

var tokenSplitPattern = regexp.MustCompile(`[^a-z0-9]+`)
var datedPathPattern = regexp.MustCompile(`/(20[0-9]{2})/([01][0-9])/([0-3][0-9])(?:/|$)`)
var datedSlugPattern = regexp.MustCompile(`(20[0-9]{2})-([01][0-9])-([0-3][0-9])`)

var themeKeywordMap = map[string][]string{
	"ai":           {"ai", "llm", "genai", "gpt", "model", "inference"},
	"agents":       {"agent", "agents", "autonomous", "orchestration", "workflow"},
	"vibecoding":   {"vibe", "vibecoding", "nocode", "builder"},
	"mcp":          {"mcp", "modelcontextprotocol", "protocol"},
	"security":     {"security", "breach", "incident", "vulnerability", "cve", "outage", "downtime"},
	"deployment":   {"deploy", "deployment", "hosting", "production", "ci", "cd", "serverless"},
	"pricing":      {"pricing", "plan", "enterprise", "free", "credit"},
	"templates":    {"template", "boilerplate", "starter", "examples"},
	"integrations": {"integration", "plugin", "sdk", "api", "github", "cursor", "claude", "copilot"},
}

var incidentTokens = map[string]struct{}{
	"breach": {}, "incident": {}, "vulnerability": {}, "outage": {}, "downtime": {}, "cve": {},
}

var genericPathTokens = map[string]struct{}{
	"blog": {}, "blogs": {}, "docs": {}, "doc": {}, "post": {}, "posts": {}, "news": {},
	"updates": {}, "update": {}, "changelog": {}, "release": {}, "releases": {}, "learn": {},
	"product": {}, "products": {}, "platform": {}, "page": {}, "pages": {}, "article": {},
	"articles": {}, "guide": {}, "guides": {}, "how": {}, "what": {}, "why": {}, "best": {},
	"new": {}, "top": {}, "home": {}, "homepage": {}, "terms": {}, "privacy": {}, "legal": {},
	"overview": {}, "intro": {}, "introduction": {}, "index": {}, "category": {}, "categories": {},
	"pt": {}, "br": {}, "de": {}, "es": {}, "fr": {}, "ja": {}, "ko": {}, "zh": {},
	"en": {}, "it": {}, "nl": {}, "ru": {}, "tr": {}, "jp": {}, "cn": {}, "tw": {},
	"faq": {}, "about": {}, "company": {}, "team": {}, "careers": {}, "jobs": {},
	"gallery": {}, "life": {}, "entertainment": {}, "education": {}, "users": {},
	"community": {}, "forum": {}, "talk": {}, "support": {}, "help": {},
	"login": {}, "signup": {}, "auth": {}, "dashboard": {}, "account": {}, "settings": {},
	"customers": {}, "customer": {}, "case": {}, "studies": {}, "stories": {},
	"author": {}, "authors": {}, "tag": {}, "tags": {},
}

var specificPathTokens = map[string]struct{}{
	"ai": {}, "agents": {}, "agent": {}, "mcp": {}, "api": {}, "sdk": {}, "claude": {}, "cursor": {},
	"replit": {}, "vercel": {}, "lovable": {}, "pricing": {}, "enterprise": {}, "security": {},
	"template": {}, "templates": {}, "integration": {}, "integrations": {}, "workflow": {},
	"deploy": {}, "deployment": {}, "hosting": {}, "vibe": {}, "vibecoding": {}, "builder": {},
	"assistant": {}, "studio": {}, "playground": {}, "auth": {}, "login": {}, "signup": {},
}

func classifyThemes(rawURL string) []string {
	tokens := urlTokens(rawURL)
	seen := map[string]struct{}{}
	for theme, keywords := range themeKeywordMap {
		for _, keyword := range keywords {
			for _, token := range tokens {
				if token == keyword {
					seen[theme] = struct{}{}
				}
			}
		}
	}
	if len(seen) == 0 {
		return []string{"general"}
	}
	out := make([]string, 0, len(seen))
	for theme := range seen {
		out = append(out, theme)
	}
	sort.Strings(out)
	return out
}

func deriveOpportunities(ours SiteSnapshot, competitors []SiteSnapshot) []Opportunity {
	opportunities := make([]Opportunity, 0)
	ourSignals := buildTopicSignals(ours)
	ourSignalCounts := signalCountMap(ourSignals)

	for _, competitor := range competitors {
		if competitor.Error != "" || competitor.RecentURLCount == 0 {
			continue
		}

		signals := buildTopicSignals(competitor)
		signals = dedupeSignalsByURLs(signals)
		signals = dedupeBySubstring(signals)
		competitorOpportunities := make([]Opportunity, 0)

		for phrase, signal := range signals {
			if signal.Count < 2 {
				continue
			}
			ourCount := ourSignalCounts[phrase]
			gap := signal.Count - ourCount
			if gap < 1 {
				continue
			}
			if signal.Specificity < 2 && signal.Count < 3 {
				continue
			}

			competitorOpportunities = append(competitorOpportunities, Opportunity{
				Title:           fmt.Sprintf("%s is shipping %q", titleWord(competitor.Name), phrase),
				WhyItMatters:    fmt.Sprintf("%s published %d recent URLs around %q vs %d on CreateOS.", competitor.Name, signal.Count, phrase, ourCount),
				WhatToDo:        buildWhatToDo(phrase, competitor.Name),
				HowToExecute:    buildExecutionPlan(phrase, competitor.Name, signal.SampleURLs),
				ImpactScore:     scoreSignal(signal.Count, gap, signal.Specificity),
				Competitor:      competitor.Name,
				Theme:           signal.Theme,
				OpportunityType: "topic-gap",
				Evidence:        signal.SampleURLs,
			})
		}

		incidentURLs := findIncidentURLs(competitor)
		if len(incidentURLs) >= 2 {
			competitorOpportunities = append(competitorOpportunities, Opportunity{
				Title:           fmt.Sprintf("CreateOS can exploit a trust opening vs %s", titleWord(competitor.Name)),
				WhyItMatters:    "Security, outage, or incident language in competitor pages is a stronger signal than broad content volume.",
				WhatToDo:        "Ship a trust and resilience page now, then publish one comparison post targeting the exact event.",
				HowToExecute:    buildIncidentPlan(competitor.Name, incidentURLs),
				ImpactScore:     clampImpact(84 + len(incidentURLs)*3),
				Competitor:      competitor.Name,
				Theme:           "security",
				OpportunityType: "incident-response",
				Evidence:        incidentURLs,
			})
		}

		sort.Slice(competitorOpportunities, func(i, j int) bool {
			if competitorOpportunities[i].ImpactScore == competitorOpportunities[j].ImpactScore {
				return competitorOpportunities[i].Title < competitorOpportunities[j].Title
			}
			return competitorOpportunities[i].ImpactScore > competitorOpportunities[j].ImpactScore
		})
		if len(competitorOpportunities) > 3 {
			competitorOpportunities = competitorOpportunities[:3]
		}
		opportunities = append(opportunities, competitorOpportunities...)
	}

	if len(opportunities) == 0 {
		opportunities = append(opportunities, Opportunity{
			Title:        "No strong gap detected in this window",
			WhyItMatters: "Competitor and CreateOS publishing patterns are relatively close for available signals.",
			WhatToDo:     "Keep weekly tracking and focus on conversion improvements for existing high-intent pages.",
			HowToExecute: []string{
				"Run this competitor workflow weekly.",
				"Track top 10 revenue-intent keywords and improve CTR/meta snippets.",
				"Prioritize pages where impressions grow but clicks lag.",
			},
			ImpactScore:     40,
			Competitor:      "market",
			Theme:           "baseline",
			OpportunityType: "baseline",
			Evidence:        []string{"No theme gap >= 2 detected"},
		})
	}

	sort.Slice(opportunities, func(i, j int) bool {
		if opportunities[i].ImpactScore == opportunities[j].ImpactScore {
			return opportunities[i].Title < opportunities[j].Title
		}
		return opportunities[i].ImpactScore > opportunities[j].ImpactScore
	})

	if len(opportunities) > 8 {
		opportunities = opportunities[:8]
	}

	return opportunities
}

func deriveTopicOpportunities(ours SiteSnapshot, topics []TopicSummary) []Opportunity {
	opportunities, _ := deriveTopicOpportunitiesWithDebug(ours, topics)
	return opportunities
}

func deriveTopicOpportunitiesWithDebug(ours SiteSnapshot, topics []TopicSummary) ([]Opportunity, TopicAnalysisDebug) {
	ourTokens := tokenFrequency(joinEntryTitlesAndURLs(ours.RecentURLs))
	opportunities := make([]Opportunity, 0)
	debug := TopicAnalysisDebug{
		ScoredTopics:  make([]TopicScoringDebug, 0, len(topics)),
		SkippedTopics: make([]SkippedTopicDebug, 0),
	}

	for _, topic := range topics {
		theme := llmTopicTheme(topic.Name)
		topicTokens := filteredTokens(topic.Name)
		matchedTokens, totalTokens := topicCoverage(topicTokens, ourTokens)
		uncoveredTokens := max(totalTokens-matchedTokens, 0)
		if topic.PageCount < 2 {
			debug.SkippedTopics = append(debug.SkippedTopics, skippedTopicDebug(topic, theme, "single-page-topic", matchedTokens, totalTokens, uncoveredTokens))
			continue
		}
		if themeCoveredByCreateOS(topic.Name, ourTokens) {
			debug.SkippedTopics = append(debug.SkippedTopics, skippedTopicDebug(topic, theme, "covered-by-createos", matchedTokens, totalTokens, uncoveredTokens))
			continue
		}
		effectivePages := supportedPageCount(topic.PageCount, len(topic.EvidenceURLs))
		evidenceQuality := evidenceQualityForTopic(topic)
		breakdown := buildScoreBreakdown(theme, effectivePages, uncoveredTokens, totalTokens, evidenceQuality)
		score := finalTopicScore(theme, len(topic.EvidenceURLs), breakdown)

		debug.ScoredTopics = append(debug.ScoredTopics, TopicScoringDebug{
			Competitor:           topic.Competitor,
			Topic:                topic.Name,
			Theme:                theme,
			PageCount:            topic.PageCount,
			EffectivePages:       effectivePages,
			EvidenceCount:        len(topic.EvidenceURLs),
			EvidenceQualityScore: evidenceQuality.Score,
			MatchedTokens:        matchedTokens,
			TotalTokens:          totalTokens,
			UncoveredTokens:      uncoveredTokens,
			ScoreBreakdown:       breakdown,
			Score:                score,
		})

		opportunities = append(opportunities, Opportunity{
			Title:           fmt.Sprintf("CreateOS should cover %q", topic.Name),
			WhyItMatters:    topic.WhyItMatters,
			WhatToDo:        topicWhatToDo(topic, theme),
			HowToExecute:    topicExecutionPlan(topic),
			ImpactScore:     score,
			Competitor:      topic.Competitor,
			Theme:           theme,
			OpportunityType: "llm-topic-gap",
			Evidence:        topic.EvidenceURLs,
		})
	}

	return opportunities, debug
}

func skippedTopicDebug(topic TopicSummary, theme string, reason string, matchedTokens int, totalTokens int, uncoveredTokens int) SkippedTopicDebug {
	return SkippedTopicDebug{
		Competitor:      topic.Competitor,
		Topic:           topic.Name,
		Theme:           theme,
		Reason:          reason,
		PageCount:       topic.PageCount,
		EvidenceCount:   len(topic.EvidenceURLs),
		MatchedTokens:   matchedTokens,
		TotalTokens:     totalTokens,
		UncoveredTokens: uncoveredTokens,
		EvidenceURLs:    limitStrings(topic.EvidenceURLs, 3),
	}
}

type EvidenceQuality struct {
	Score                 int `json:"score"`
	SpecificEvidenceCount int `json:"specificEvidenceCount"`
	GenericTitlePenalty   int `json:"genericTitlePenalty"`
	HomepagePenalty       int `json:"homepagePenalty"`
	MixedEvidencePenalty  int `json:"mixedEvidencePenalty"`
	WeakEvidencePenalty   int `json:"weakEvidencePenalty"`
}

func evidenceQualityForTopic(topic TopicSummary) EvidenceQuality {
	quality := EvidenceQuality{Score: 45}
	theme := llmTopicTheme(topic.Name)
	pageTypes := map[string]struct{}{}
	for idx, evidenceURL := range topic.EvidenceURLs {
		title := ""
		if idx < len(topic.RepresentativeTitles) {
			title = topic.RepresentativeTitles[idx]
		}
		pageType := classifyPageType(topic.Competitor, evidenceURL, title)
		pageTypes[pageType] = struct{}{}
		if pageType == "homepage" {
			quality.HomepagePenalty += 12
			continue
		}
		if isGenericTitle(title) {
			quality.GenericTitlePenalty += 10
			continue
		}
		quality.SpecificEvidenceCount++
	}
	quality.Score += min(quality.SpecificEvidenceCount, 4) * 12
	if len(pageTypes) > 1 {
		quality.MixedEvidencePenalty = (len(pageTypes) - 1) * 8
	}
	if quality.SpecificEvidenceCount < 2 {
		quality.WeakEvidencePenalty = 16
	}
	if theme == "security" && len(pageTypes) > 1 {
		quality.MixedEvidencePenalty += 8
	}
	quality.Score = clampImpact(quality.Score - quality.GenericTitlePenalty - quality.HomepagePenalty - quality.MixedEvidencePenalty - quality.WeakEvidencePenalty)
	return quality
}

func buildScoreBreakdown(theme string, effectivePages int, uncoveredTokens int, totalTokens int, evidenceQuality EvidenceQuality) ScoreBreakdown {
	gap := scoreLLMTopicGap(effectivePages, uncoveredTokens, totalTokens)
	return ScoreBreakdown{
		GapCoverage:     gap,
		EvidenceQuality: evidenceQuality.Score,
		ThemePriority:   themePriorityScore(theme),
		Relevance:       relevanceScore(theme, evidenceQuality),
	}
}

func finalTopicScore(theme string, evidenceCount int, breakdown ScoreBreakdown) int {
	score := (breakdown.GapCoverage*45 + breakdown.EvidenceQuality*35 + breakdown.ThemePriority*15 + breakdown.Relevance*5) / 100
	if evidenceCount < 2 && score > 84 {
		score = 84
	}
	if evidenceCount <= 2 && score > 92 {
		score = 92
	}
	switch theme {
	case "general":
		if score > 82 {
			score = 82
		}
	case "comparison", "usecases":
		if score > 88 {
			score = 88
		}
	case "integrations":
		if score > 90 {
			score = 90
		}
	case "vibecoding":
		if score > 94 {
			score = 94
		}
	}
	return clampImpact(score)
}

func themePriorityScore(theme string) int {
	switch theme {
	case "agents", "ai", "vibecoding":
		return 92
	case "security", "enterprise", "comparison":
		return 84
	case "integrations", "usecases":
		return 78
	case "templates":
		return 70
	default:
		return 55
	}
}

func relevanceScore(theme string, evidenceQuality EvidenceQuality) int {
	score := evidenceQuality.Score
	if theme == "usecases" || theme == "comparison" {
		score -= 8
	}
	return clampImpact(score)
}

func joinEntryTitlesAndURLs(entries []SitemapEntry) string {
	parts := make([]string, 0, len(entries)*2)
	for _, entry := range entries {
		parts = append(parts, entry.Title, entry.URL)
	}
	return strings.Join(parts, " ")
}

func tokenFrequency(raw string) map[string]int {
	out := map[string]int{}
	for _, token := range filteredTokens(raw) {
		out[token]++
	}
	return out
}

func topicCoverage(topicTokens []string, ourTokens map[string]int) (int, int) {
	seen := map[string]struct{}{}
	matches := 0
	total := 0
	for _, token := range topicTokens {
		if token == "" {
			continue
		}
		if _, generic := genericPathTokens[token]; generic {
			continue
		}
		if _, already := seen[token]; already {
			continue
		}
		seen[token] = struct{}{}
		total++
		if ourTokens[token] > 0 {
			matches++
		}
	}
	return matches, total
}

func themeCoveredByCreateOS(topicName string, ourTokens map[string]int) bool {
	matches, total := topicCoverage(filteredTokens(topicName), ourTokens)
	if total == 0 {
		return false
	}
	if total == 1 {
		return matches == 1
	}
	needed := max(2, (total+1)/2)
	return matches >= needed
}

func scoreLLMTopicGap(pageCount int, uncoveredTokens int, totalTokens int) int {
	uncoveredPercent := 0
	if totalTokens > 0 {
		uncoveredPercent = (uncoveredTokens * 100) / totalTokens
	}
	score := 35 + min(pageCount, 10)*4 + uncoveredPercent/2 + min(uncoveredTokens, 4)*3
	return clampImpact(score)
}

func topicExecutionPlan(topic TopicSummary) []string {
	theme := llmTopicTheme(topic.Name)
	slug := suggestedSlug(topic.Name, theme)
	pageType := pageTypeForTheme(theme)
	title := suggestedTitle(topic.Name, theme, topic.Competitor)
	evidenceTitle := firstNonEmpty(topic.RepresentativeTitles)
	if evidenceTitle == "" && len(topic.EvidenceURLs) > 0 {
		evidenceTitle = topic.EvidenceURLs[0]
	}
	return []string{
		fmt.Sprintf("Create `%s` as a %s titled %q.", slug, pageType, title),
		fmt.Sprintf("Use competitor evidence like %q to shape H2s, but show CreateOS-specific workflows, screenshots, and outcomes.", evidenceTitle),
		"Add internal links from the homepage, relevant product/docs pages, and at least two related blog posts before requesting indexing.",
	}
}

func topicWhatToDo(topic TopicSummary, theme string) string {
	return fmt.Sprintf(
		"Create a %s at `%s` titled %q targeting %s; use %d competitor evidence URLs as the outline, then link it into the relevant CreateOS cluster.",
		pageTypeForTheme(theme),
		suggestedSlug(topic.Name, theme),
		suggestedTitle(topic.Name, theme, topic.Competitor),
		intentForTheme(theme),
		len(topic.EvidenceURLs),
	)
}

func buildContentRecommendations(opportunities []Opportunity) []ContentRecommendation {
	recommendations := make([]ContentRecommendation, 0, len(opportunities))
	for idx, opportunity := range opportunities {
		topic := opportunityTopicName(opportunity.Title)
		if topic == "" {
			topic = opportunity.Title
		}
		pageType := pageTypeForTheme(opportunity.Theme)
		recommendations = append(recommendations, ContentRecommendation{
			Priority:       idx + 1,
			Opportunity:    opportunity.Title,
			Competitor:     opportunity.Competitor,
			Theme:          opportunity.Theme,
			PageType:       pageType,
			SuggestedSlug:  suggestedSlug(topic, opportunity.Theme),
			SuggestedTitle: suggestedTitle(topic, opportunity.Theme, opportunity.Competitor),
			TargetIntent:   intentForTheme(opportunity.Theme),
			ContentAngle:   contentAngleForTheme(topic, opportunity.Theme, opportunity.Competitor),
			Pillar:         pillarForTheme(opportunity.Theme),
			ClusterPages:   clusterPagesForOpportunity(topic, opportunity.Theme, opportunity.Competitor),
			SourceEvidence: limitStrings(opportunity.Evidence, 3),
		})
	}
	return recommendations
}

func opportunityTopicName(title string) string {
	start := strings.Index(title, "\"")
	end := strings.LastIndex(title, "\"")
	if start >= 0 && end > start {
		return title[start+1 : end]
	}
	return strings.TrimSpace(strings.TrimPrefix(title, "CreateOS should cover "))
}

func pageTypeForTheme(theme string) string {
	switch theme {
	case "comparison":
		return "comparison page"
	case "usecases", "vibecoding":
		return "use-case landing page"
	case "enterprise":
		return "solution landing page"
	case "security":
		return "trust page"
	case "integrations":
		return "integration page"
	case "agents", "ai":
		return "pillar guide"
	default:
		return "focused SEO page"
	}
}

func suggestedSlug(topic string, theme string) string {
	slug := safeSlug(topic)
	switch theme {
	case "comparison":
		return "/compare/" + slug
	case "usecases", "vibecoding":
		return "/use-cases/" + slug
	case "enterprise", "security":
		return "/solutions/" + slug
	case "integrations":
		return "/integrations/" + slug
	default:
		return "/blogs/" + slug
	}
}

func suggestedTitle(topic string, theme string, competitor string) string {
	switch theme {
	case "comparison":
		return fmt.Sprintf("%s: CreateOS Comparison Guide", titleFromTopic(topic))
	case "usecases", "vibecoding":
		return fmt.Sprintf("%s with CreateOS", titleFromTopic(topic))
	case "enterprise":
		return fmt.Sprintf("%s for Teams", titleFromTopic(topic))
	case "security":
		return fmt.Sprintf("%s with CreateOS", titleFromTopic(topic))
	case "integrations":
		return fmt.Sprintf("%s Integration Workflows", titleFromTopic(topic))
	default:
		if competitor != "" && competitor != "market" {
			return fmt.Sprintf("%s: What CreateOS Can Learn from %s", titleFromTopic(topic), titleWord(competitor))
		}
		return titleFromTopic(topic)
	}
}

func intentForTheme(theme string) string {
	switch theme {
	case "comparison":
		return "commercial evaluation"
	case "usecases", "vibecoding":
		return "persona/use-case evaluation"
	case "enterprise", "security":
		return "enterprise trust evaluation"
	case "integrations":
		return "implementation intent"
	case "agents", "ai":
		return "educational-to-product intent"
	default:
		return "topic discovery"
	}
}

func contentAngleForTheme(topic string, theme string, competitor string) string {
	switch theme {
	case "comparison":
		return fmt.Sprintf("Capture buyers comparing tools by showing where CreateOS is stronger than %s for %s.", titleWord(competitor), strings.ToLower(topic))
	case "usecases", "vibecoding":
		return fmt.Sprintf("Show the exact workflow from idea to shipped result for %s, with CreateOS as the execution layer.", strings.ToLower(topic))
	case "enterprise":
		return "Translate competitor enterprise positioning into CreateOS-specific team workflows, governance, security, and deployment proof."
	case "security":
		return "Use trust, reliability, and control as the conversion hook, not generic security language."
	case "integrations":
		return "Make the integration path concrete with prerequisites, setup steps, screenshots, and expected outcomes."
	default:
		return "Turn competitor demand into a CreateOS-specific page with examples, proof, and internal links."
	}
}

func pillarForTheme(theme string) string {
	switch theme {
	case "comparison":
		return "AI builder comparisons"
	case "usecases", "vibecoding":
		return "AI app-building use cases"
	case "enterprise":
		return "Enterprise AI development"
	case "security":
		return "Secure AI app development"
	case "integrations":
		return "CreateOS integrations"
	case "agents", "ai":
		return "AI-native development workflows"
	default:
		return "CreateOS product-led SEO"
	}
}

func clusterPagesForOpportunity(topic string, theme string, competitor string) []ContentPageRecommendation {
	baseSlug := safeSlug(topic)
	switch theme {
	case "comparison":
		return []ContentPageRecommendation{
			{PageType: "comparison page", Slug: fmt.Sprintf("/compare/%s-alternative", competitor), Title: fmt.Sprintf("Best %s Alternative for AI App Builders", titleWord(competitor)), TargetIntent: "commercial evaluation"},
			{PageType: "listicle", Slug: "/blogs/best-ai-app-builders", Title: "Best AI App Builders for Shipping Real Products", TargetIntent: "commercial research"},
			{PageType: "comparison page", Slug: "/compare/createos-vs-ai-builders", Title: "CreateOS vs AI App Builders: Which Should You Choose?", TargetIntent: "commercial evaluation"},
		}
	case "usecases", "vibecoding":
		return []ContentPageRecommendation{
			{PageType: "use-case landing page", Slug: "/use-cases/product-managers", Title: "AI App Builder for Product Managers", TargetIntent: "persona evaluation"},
			{PageType: "use-case landing page", Slug: "/use-cases/startup-mvp-builder", Title: "AI MVP Builder for Startups", TargetIntent: "use-case evaluation"},
			{PageType: "guide", Slug: "/blogs/rapid-prototyping-with-ai", Title: "Rapid Prototyping with AI: From Idea to MVP", TargetIntent: "educational-to-product intent"},
		}
	case "enterprise":
		return []ContentPageRecommendation{
			{PageType: "solution landing page", Slug: "/solutions/enterprise-ai-development", Title: "Enterprise AI Development with CreateOS", TargetIntent: "enterprise evaluation"},
			{PageType: "trust page", Slug: "/solutions/security-governance", Title: "Security and Governance for AI-Built Apps", TargetIntent: "enterprise trust evaluation"},
			{PageType: "case-study template", Slug: "/customers/ai-development-teams", Title: "How Teams Ship AI Apps with CreateOS", TargetIntent: "proof evaluation"},
		}
	default:
		return []ContentPageRecommendation{
			{PageType: pageTypeForTheme(theme), Slug: suggestedSlug(topic, theme), Title: suggestedTitle(topic, theme, competitor), TargetIntent: intentForTheme(theme)},
			{PageType: "guide", Slug: "/blogs/" + baseSlug + "-guide", Title: guideTitleFromTopic(topic), TargetIntent: "educational intent"},
			{PageType: "case-study template", Slug: "/customers/" + baseSlug, Title: titleFromTopic(topic) + " Examples", TargetIntent: "proof evaluation"},
		}
	}
}

func guideTitleFromTopic(topic string) string {
	title := titleFromTopic(topic)
	normalized := strings.ToLower(strings.TrimSpace(title))
	if strings.HasSuffix(normalized, " guide") || strings.HasSuffix(normalized, " guides") {
		return title
	}
	return title + " Guide"
}

func titleFromTopic(topic string) string {
	words := strings.Fields(strings.ReplaceAll(topic, "&", "and"))
	for idx, word := range words {
		words[idx] = titleCaseToken(word)
	}
	return strings.Join(words, " ")
}

func titleCaseToken(token string) string {
	parts := strings.Split(token, "-")
	for idx, part := range parts {
		if part == "" {
			continue
		}
		upper := strings.ToUpper(part)
		if upper == "AI" || upper == "MVP" || upper == "SEO" || upper == "API" || upper == "LLM" || upper == "MCP" {
			parts[idx] = upper
			continue
		}
		parts[idx] = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
	}
	return strings.Join(parts, "-")
}

func firstNonEmpty(values []string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func limitStrings(values []string, limit int) []string {
	if len(values) <= limit {
		return values
	}
	return values[:limit]
}

func safeSlug(raw string) string {
	tokens := filteredTokens(raw)
	if len(tokens) == 0 {
		return "topic"
	}
	return strings.Join(tokens, "-")
}

func buildExecutionPlan(phrase string, competitor string, evidence []string) []string {
	example := ""
	if len(evidence) > 0 {
		example = evidence[0]
	}
	return []string{
		fmt.Sprintf("Ship one CreateOS page targeting %q and link it from the homepage/docs.", phrase),
		fmt.Sprintf("Publish one article with screenshots, code samples, and a direct comparison to %s if relevant.", competitor),
		fmt.Sprintf("Request indexing for the new page and refresh internal links after shipping. Example source: %s", example),
	}
}

func buildIncidentPlan(competitor string, evidence []string) []string {
	example := ""
	if len(evidence) > 0 {
		example = evidence[0]
	}
	return []string{
		fmt.Sprintf("Publish a CreateOS trust page that addresses the exact %s event angle.", competitor),
		"Add a comparison post explaining reliability, observability, backups, and support.",
		fmt.Sprintf("Tie the narrative to a specific event URL or announcement. Example: %s", example),
	}
}

func findIncidentURLs(snapshot SiteSnapshot) []string {
	matches := make([]string, 0, 3)
	for _, entry := range snapshot.RecentURLs {
		tokens := urlTokens(entry.URL)
		for _, token := range tokens {
			if _, found := incidentTokens[token]; found {
				matches = append(matches, entry.URL)
				break
			}
		}
		if len(matches) == 3 {
			break
		}
	}
	return matches
}

func buildVelocityPlan(competitor string) []string {
	return []string{
		fmt.Sprintf("Pick the 3 highest-intent topics %s is publishing and publish CreateOS variants this week.", competitor),
		"Turn release notes into one landing page and one article instead of broad content.",
		"Measure indexation, impressions, and click-through before expanding the cluster.",
	}
}

func buildWhatToDo(phrase string, competitor string) string {
	switch {
	case strings.Contains(phrase, "pricing"):
		return "Ship a pricing page, comparison page, and one bottom-funnel article before they own the query."
	case strings.Contains(phrase, "mcp") || strings.Contains(phrase, "integration"):
		return "Publish a CreateOS integration or MCP page that shows the exact workflow users want."
	case strings.Contains(phrase, "agent"):
		return "Build an agent-specific landing page and one concrete workflow tutorial around CreateOS."
	case strings.Contains(phrase, "template") || strings.Contains(phrase, "starter"):
		return "Ship template or starter-kit content with runnable examples and screenshots."
	case strings.Contains(phrase, "security") || strings.Contains(phrase, "incident") || strings.Contains(phrase, "outage"):
		return "Use the trust angle immediately: publish a resilience page and comparison article."
	default:
		return fmt.Sprintf("Create a CreateOS page that beats %s on this exact topic cluster.", competitor)
	}
}

func buildTopicSignals(snapshot SiteSnapshot) map[string]topicSignal {
	signals := map[string]topicSignal{}
	for _, entry := range snapshot.RecentURLs {
		tokens := filteredTokens(entry.URL)
		if len(tokens) < 2 {
			continue
		}
		phrases := phraseCandidates(tokens)
		for _, phrase := range phrases {
			signal := signals[phrase]
			signal.Phrase = phrase
			signal.Theme = phraseTheme(phrase)
			signal.Count++
			if len(signal.SampleURLs) < 3 {
				signal.SampleURLs = append(signal.SampleURLs, entry.URL)
			}
			signal.Specificity = max(signal.Specificity, phraseSpecificity(phrase))
			signals[phrase] = signal
		}
	}
	return signals
}

func dedupeSignalsByURLs(signals map[string]topicSignal) map[string]topicSignal {
	type kv struct {
		k string
		v topicSignal
	}

	list := make([]kv, 0, len(signals))
	for k, v := range signals {
		list = append(list, kv{k: k, v: v})
	}

	sort.Slice(list, func(i, j int) bool {
		if len(list[i].v.SampleURLs) == len(list[j].v.SampleURLs) {
			if len(list[i].k) == len(list[j].k) {
				return list[i].k > list[j].k
			}
			return len(list[i].k) > len(list[j].k)
		}
		return len(list[i].v.SampleURLs) > len(list[j].v.SampleURLs)
	})

	out := map[string]topicSignal{}
	used := map[string]struct{}{}
	for _, item := range list {
		overlap := 0
		for _, u := range item.v.SampleURLs {
			if _, seen := used[u]; seen {
				overlap++
			}
		}
		if overlap >= 2 {
			continue
		}
		out[item.k] = item.v
		for _, u := range item.v.SampleURLs {
			used[u] = struct{}{}
		}
	}
	return out
}

func dedupeBySubstring(signals map[string]topicSignal) map[string]topicSignal {
	keys := make([]string, 0, len(signals))
	for k := range signals {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		left := signals[keys[i]]
		right := signals[keys[j]]
		if left.Count != right.Count {
			return left.Count > right.Count
		}
		if len(keys[i]) != len(keys[j]) {
			return len(keys[i]) < len(keys[j])
		}
		return keys[i] < keys[j]
	})

	out := map[string]topicSignal{}
	for _, k := range keys {
		kept := false
		for existing := range out {
			if sharesTokens(k, existing, 2) {
				kept = true
				break
			}
		}
		if !kept {
			out[k] = signals[k]
		}
	}
	return out
}

func sharesTokens(a, b string, threshold int) bool {
	aTokens := map[string]struct{}{}
	for _, t := range strings.Fields(a) {
		aTokens[t] = struct{}{}
	}
	overlap := 0
	for _, t := range strings.Fields(b) {
		if _, ok := aTokens[t]; ok {
			overlap++
		}
	}
	return overlap >= threshold
}

type topicSignal struct {
	Phrase      string
	Theme       string
	Count       int
	Specificity int
	SampleURLs  []string
}

func signalCountMap(signals map[string]topicSignal) map[string]int {
	counts := make(map[string]int, len(signals))
	for phrase, signal := range signals {
		counts[phrase] = signal.Count
	}
	return counts
}

func filteredTokens(rawURL string) []string {
	tokens := urlTokens(rawURL)
	filtered := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if _, isGeneric := genericPathTokens[token]; isGeneric {
			continue
		}
		filtered = append(filtered, token)
	}
	return filtered
}

func phraseCandidates(tokens []string) []string {
	if len(tokens) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for size := 2; size <= 4; size++ {
		for i := 0; i+size <= len(tokens); i++ {
			window := tokens[i : i+size]
			if !phraseIsUseful(window) {
				continue
			}
			phrase := strings.Join(window, " ")
			if _, exists := seen[phrase]; exists {
				continue
			}
			seen[phrase] = struct{}{}
			out = append(out, phrase)
		}
	}
	return out
}

func phraseIsUseful(tokens []string) bool {
	hasSpecific := false
	for _, token := range tokens {
		if len(token) < 3 {
			return false
		}
		if _, generic := genericPathTokens[token]; generic {
			return false
		}
		if _, specific := specificPathTokens[token]; specific {
			hasSpecific = true
		}
	}
	return hasSpecific
}

func isJunkPath(competitor string, rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	p := strings.ToLower(parsed.Path)

	localePrefixes := []string{"/pt-br/", "/pt/", "/de/", "/es/", "/fr/", "/ja/", "/ko/", "/zh/", "/it/", "/nl/", "/ru/", "/tr/"}
	for _, lp := range localePrefixes {
		if strings.HasPrefix(p, lp) {
			return true
		}
	}

	blocked := map[string][]string{
		"replit":  {"/gallery/", "/@", "/talk/", "/bounties/"},
		"lovable": {"/faq/"},
		"vercel":  {"/customers/", "/blog/author/"},
	}
	for _, bp := range blocked[competitor] {
		if strings.HasPrefix(p, bp) {
			return true
		}
	}
	return false
}

func isRelevantPromptPage(competitor string, rawURL string, title string, allowMissingTitle bool) bool {
	if strings.TrimSpace(rawURL) == "" {
		return false
	}
	if !allowMissingTitle && strings.TrimSpace(title) == "" {
		return false
	}
	if isLowValuePromptPath(competitor, rawURL) {
		return false
	}
	if strings.TrimSpace(title) != "" && isLowValuePromptTitle(title) {
		return false
	}
	return true
}

func isLowValuePromptPath(competitor string, rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	path := strings.ToLower(strings.Trim(parsed.Path, "/"))
	if path == "" {
		return false
	}
	segments := strings.Split(path, "/")
	first := segments[0]
	localeCodes := map[string]struct{}{
		"pt-br": {}, "pt": {}, "de": {}, "es": {}, "fr": {}, "ja": {}, "ko": {}, "zh": {},
		"it": {}, "nl": {}, "ru": {}, "tr": {}, "jp": {}, "cn": {}, "tw": {},
	}
	if _, isLocale := localeCodes[first]; isLocale {
		return true
	}
	lowValueFirstSegments := map[string]struct{}{
		"abuse": {}, "affiliates": {}, "brand": {}, "careers": {}, "events": {}, "gallery": {},
		"jobs": {}, "legal": {}, "privacy": {}, "signin": {}, "sign-in": {}, "signup": {},
		"support": {}, "terms": {}, "faq": {}, "download": {}, "links": {}, "subprocessors": {},
		"birthday": {}, "certifications": {}, "discover": {}, "blog": {}, "blogs": {}, "kb": {},
	}
	if _, lowValue := lowValueFirstSegments[first]; lowValue && len(segments) == 1 {
		return true
	}
	alwaysLowValueFirstSegments := map[string]struct{}{
		"abuse": {}, "affiliates": {}, "brand": {}, "careers": {}, "events": {}, "gallery": {},
		"jobs": {}, "legal": {}, "privacy": {}, "signin": {}, "sign-in": {}, "signup": {},
		"support": {}, "terms": {}, "faq": {}, "download": {}, "links": {}, "subprocessors": {},
		"birthday": {}, "certifications": {}, "discover": {},
	}
	if _, lowValue := alwaysLowValueFirstSegments[first]; lowValue {
		return true
	}
	lowValuePathFragments := []string{
		"cookie", "terms", "privacy", "data-processing", "definitions", "legal-change",
		"code-of-conduct", "do-not-sell", "domain-registration", "dora-addendum",
		"desktop-app-terms", "/errors", "/category/",
	}
	for _, fragment := range lowValuePathFragments {
		if strings.Contains(path, fragment) {
			return true
		}
	}
	if strings.Contains(path, "/status") || strings.HasSuffix(path, "/status") {
		return true
	}
	if strings.HasPrefix(path, "edu/") {
		return true
	}
	if competitor == "replit" && (strings.Contains(path, "/admin") || strings.Contains(path, "/sign-in")) {
		return true
	}
	return false
}

func isLowValuePromptTitle(title string) bool {
	normalized := strings.ToLower(strings.TrimSpace(title))
	if normalized == "" {
		return false
	}
	lowValueFragments := []string{
		"careers", "press enquiries", "report abuse", "access private deployment", "dashboard",
		"cookie policy", "terms", "privacy", "data processing", "definitions", "code of conduct",
		"subprocessors", "certification", "turns 10",
	}
	for _, fragment := range lowValueFragments {
		if strings.Contains(normalized, fragment) {
			return true
		}
	}
	return false
}

func phraseSpecificity(phrase string) int {
	score := 0
	for _, token := range strings.Fields(phrase) {
		if _, specific := specificPathTokens[token]; specific {
			score += 2
			continue
		}
		if len(token) > 5 {
			score++
		}
	}
	if strings.Contains(phrase, "mcp") || strings.Contains(phrase, "cursor") || strings.Contains(phrase, "claude") {
		score += 2
	}
	return score
}

func phraseTheme(phrase string) string {
	switch {
	case strings.Contains(phrase, "pricing"):
		return "pricing"
	case strings.Contains(phrase, "security") || strings.Contains(phrase, "incident") || strings.Contains(phrase, "outage"):
		return "security"
	case strings.Contains(phrase, "agent"):
		return "agents"
	case strings.Contains(phrase, "mcp"):
		return "mcp"
	case strings.Contains(phrase, "template"):
		return "templates"
	case strings.Contains(phrase, "integration") || strings.Contains(phrase, "sdk") || strings.Contains(phrase, "api"):
		return "integrations"
	case strings.Contains(phrase, "deploy") || strings.Contains(phrase, "hosting") || strings.Contains(phrase, "serverless"):
		return "deployment"
	case strings.Contains(phrase, "vibe") || strings.Contains(phrase, "builder"):
		return "vibecoding"
	case strings.Contains(phrase, "ai") || strings.Contains(phrase, "llm") || strings.Contains(phrase, "genai"):
		return "ai"
	default:
		return "general"
	}
}

func llmTopicTheme(topicName string) string {
	raw := strings.ToLower(topicName)
	switch {
	case strings.Contains(raw, "comparison") || strings.Contains(raw, "benchmark") || strings.Contains(raw, "alternative"):
		return "comparison"
	case strings.Contains(raw, "use case") || strings.Contains(raw, "industry-specific") || strings.Contains(raw, "persona"):
		return "usecases"
	case strings.Contains(raw, "enterprise"):
		return "enterprise"
	case strings.Contains(raw, "security") || strings.Contains(raw, "integrity") || strings.Contains(raw, "vulnerability"):
		return "security"
	case strings.Contains(raw, "agentic") || strings.Contains(raw, "agent workflow") || strings.Contains(raw, "coding tools"):
		return "agents"
	case strings.Contains(raw, "prototype") || strings.Contains(raw, "prototyping") || strings.Contains(raw, "mvp") || strings.Contains(raw, "app building") || strings.Contains(raw, "website building") || strings.Contains(raw, "no-code") || strings.Contains(raw, "low-code"):
		return "vibecoding"
	case strings.Contains(raw, "partner") || strings.Contains(raw, "ecosystem") || strings.Contains(raw, "connect") || strings.Contains(raw, "integration"):
		return "integrations"
	case strings.Contains(raw, "enterprise") || strings.Contains(raw, "pricing"):
		return "pricing"
	case strings.Contains(raw, "ai") || strings.Contains(raw, "llm") || strings.Contains(raw, "model"):
		return "ai"
	default:
		return phraseTheme(topicName)
	}
}

func scoreSignal(count int, gap int, specificity int) int {
	score := 22 + min(count, 6)*6 + min(gap, 6)*5 + min(specificity, 6)*4
	if count >= 4 {
		score += 8
	}
	if gap >= 3 {
		score += 6
	}
	return clampImpact(score)
}

func urlTokens(rawURL string) []string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	target := rawURL
	if err == nil {
		target = parsed.Path
	}
	target = strings.ToLower(target)
	parts := tokenSplitPattern.Split(target, -1)
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

func clampImpact(score int) int {
	if score < 1 {
		return 1
	}
	if score > 100 {
		return 100
	}
	return score
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func titleWord(raw string) string {
	if raw == "" {
		return raw
	}
	lower := strings.ToLower(raw)
	return strings.ToUpper(lower[:1]) + lower[1:]
}

func inferDateFromURL(rawURL string) *time.Time {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	target := rawURL
	if err == nil {
		target = parsed.Path
	}
	target = strings.TrimSpace(target)
	if target == "" {
		return nil
	}

	match := datedPathPattern.FindStringSubmatch(target)
	if len(match) == 4 {
		return makeDate(match[1], match[2], match[3])
	}
	match = datedSlugPattern.FindStringSubmatch(target)
	if len(match) == 4 {
		return makeDate(match[1], match[2], match[3])
	}
	return nil
}

func makeDate(year string, month string, day string) *time.Time {
	parsed, err := time.Parse("2006-01-02", year+"-"+month+"-"+day)
	if err != nil {
		return nil
	}
	utc := parsed.UTC()
	return &utc
}
