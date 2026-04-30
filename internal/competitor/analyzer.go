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
	"breach": {}, "incident": {}, "vulnerability": {}, "outage": {}, "downtime": {}, "security": {},
}

var genericPathTokens = map[string]struct{}{
	"blog": {}, "blogs": {}, "docs": {}, "doc": {}, "post": {}, "posts": {}, "news": {},
	"updates": {}, "update": {}, "changelog": {}, "release": {}, "releases": {}, "learn": {},
	"product": {}, "products": {}, "platform": {}, "page": {}, "pages": {}, "article": {},
	"articles": {}, "guide": {}, "guides": {}, "how": {}, "what": {}, "why": {}, "best": {},
	"new": {}, "top": {}, "home": {}, "homepage": {}, "terms": {}, "privacy": {}, "legal": {},
	"overview": {}, "intro": {}, "introduction": {}, "index": {}, "category": {}, "categories": {},
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
		if len(incidentURLs) > 0 {
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

		if competitor.RecentURLCount > ours.RecentURLCount*2 && competitor.RecentURLCount-ours.RecentURLCount >= 10 {
			competitorOpportunities = append(competitorOpportunities, Opportunity{
				Title:           fmt.Sprintf("%s is publishing much faster than CreateOS", titleWord(competitor.Name)),
				WhyItMatters:    fmt.Sprintf("%s shipped %d recent URLs while CreateOS shipped %d.", competitor.Name, competitor.RecentURLCount, ours.RecentURLCount),
				WhatToDo:        "Match their content velocity with a narrower, higher-intent CreateOS publishing loop.",
				HowToExecute:    buildVelocityPlan(competitor.Name),
				ImpactScore:     clampImpact(62 + (competitor.RecentURLCount-ours.RecentURLCount)/3),
				Competitor:      competitor.Name,
				Theme:           "distribution",
				OpportunityType: "velocity-gap",
				Evidence: []string{
					fmt.Sprintf("%s recent URLs=%d", competitor.Name, competitor.RecentURLCount),
					fmt.Sprintf("createos recent URLs=%d", ours.RecentURLCount),
				},
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
		if len(token) < 2 {
			return false
		}
		if _, generic := genericPathTokens[token]; generic {
			return false
		}
		if _, specific := specificPathTokens[token]; specific {
			hasSpecific = true
		}
	}
	return hasSpecific || len(tokens) >= 3
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
