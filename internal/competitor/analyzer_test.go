package competitor

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestClassifyThemes(t *testing.T) {
	themes := classifyThemes("https://example.com/blog/ai-agents-mcp-security")
	require.Contains(t, themes, "ai")
	require.Contains(t, themes, "agents")
	require.Contains(t, themes, "mcp")
	require.Contains(t, themes, "security")
}

func TestDeriveOpportunitiesPrefersTopicSpecificSignals(t *testing.T) {
	ours := SiteSnapshot{
		Name:           "createos",
		RecentURLCount: 2,
		RecentURLs: []SitemapEntry{
			{URL: "https://createos.sh/blog/agent-workflows", ThemeTags: []string{"agents"}},
		},
	}
	competitor := SiteSnapshot{
		Name:           "vercel",
		RecentURLCount: 12,
		RecentURLs: []SitemapEntry{
			{URL: "https://vercel.com/blog/agent-workflows", ThemeTags: []string{"agents"}},
			{URL: "https://vercel.com/blog/agent-workflows-for-teams", ThemeTags: []string{"agents"}},
			{URL: "https://vercel.com/blog/security-incident-update", ThemeTags: []string{"security"}},
		},
	}

	results := deriveOpportunities(ours, []SiteSnapshot{competitor})
	require.NotEmpty(t, results)

	var hasTopicGap bool
	var hasIncident bool
	for _, result := range results {
		if result.OpportunityType == "topic-gap" && strings.Contains(result.Title, "agent workflows") {
			hasTopicGap = true
		}
		if result.OpportunityType == "incident-response" {
			hasIncident = true
		}
		require.GreaterOrEqual(t, result.ImpactScore, 1)
		require.LessOrEqual(t, result.ImpactScore, 100)
	}

	require.True(t, hasTopicGap)
	require.False(t, hasIncident)
}

func TestDeriveTopicOpportunitiesSkipsCoveredCreateOSThemes(t *testing.T) {
	ours := SiteSnapshot{
		Name: "createos",
		RecentURLs: []SitemapEntry{
			{URL: "https://createos.sh/blogs/agentic-deployments-ai-agents-createos", Title: "Agentic deployments for AI agents with CreateOS"},
		},
	}
	topics := []TopicSummary{
		{
			Competitor:           "vercel",
			Name:                 "AI gateway integrations with coding agents",
			PageCount:            8,
			RepresentativeTitles: []string{"Coding Agents · AI Gateway"},
			EvidenceURLs:         []string{"https://vercel.com/docs/ai-gateway/coding-agents"},
			WhyItMatters:         "Vercel is building agent infrastructure intent.",
		},
		{
			Competitor:           "lovable",
			Name:                 "Template gallery for SaaS apps",
			PageCount:            6,
			RepresentativeTitles: []string{"SaaS templates"},
			EvidenceURLs:         []string{"https://lovable.dev/templates/apps/saas"},
			WhyItMatters:         "Lovable is capturing ready-to-build SaaS intent.",
		},
	}

	opportunities, debug := deriveTopicOpportunitiesWithDebug(ours, topics)

	require.Len(t, opportunities, 1)
	require.Equal(t, "CreateOS should cover \"Template gallery for SaaS apps\"", opportunities[0].Title)
	require.Equal(t, "llm-topic-gap", opportunities[0].OpportunityType)
	require.Len(t, debug.SkippedTopics, 1)
	require.Equal(t, "AI gateway integrations with coding agents", debug.SkippedTopics[0].Topic)
	require.Equal(t, "covered-by-createos", debug.SkippedTopics[0].Reason)
	require.NotZero(t, debug.SkippedTopics[0].MatchedTokens)
}

func TestDeriveTopicOpportunitiesSkipsSingleKeywordCoveredTopic(t *testing.T) {
	ours := SiteSnapshot{
		Name: "createos",
		RecentURLs: []SitemapEntry{
			{URL: "https://createos.sh/blogs/mcp-server-setup", Title: "MCP server setup"},
		},
	}
	topics := []TopicSummary{
		{
			Competitor:   "vercel",
			Name:         "MCP",
			PageCount:    6,
			EvidenceURLs: []string{"https://vercel.com/docs/mcp"},
			WhyItMatters: "MCP demand is rising.",
		},
		{
			Competitor:   "vercel",
			Name:         "Template gallery for SaaS apps",
			PageCount:    6,
			EvidenceURLs: []string{"https://vercel.com/templates/saas"},
			WhyItMatters: "Template demand is rising.",
		},
	}

	opportunities := deriveTopicOpportunities(ours, topics)
	require.Len(t, opportunities, 1)
	require.Equal(t, "CreateOS should cover \"Template gallery for SaaS apps\"", opportunities[0].Title)
}

func TestDeriveTopicOpportunitiesAvoidsSubstringFalsePositive(t *testing.T) {
	ours := SiteSnapshot{
		Name: "createos",
		RecentURLs: []SitemapEntry{
			{URL: "https://createos.sh/blogs/capital-structure-basics", Title: "Capital structure basics"},
		},
	}
	topics := []TopicSummary{
		{
			Competitor:   "lovable",
			Name:         "API integrations",
			PageCount:    4,
			EvidenceURLs: []string{"https://lovable.dev/docs/api/integrations"},
			WhyItMatters: "Integration intent.",
		},
	}

	opportunities := deriveTopicOpportunities(ours, topics)
	require.Len(t, opportunities, 1)
	require.Equal(t, "CreateOS should cover \"API integrations\"", opportunities[0].Title)
}

func TestDeriveTopicOpportunitiesScoresLessCoveredTopicHigher(t *testing.T) {
	ours := SiteSnapshot{
		Name: "createos",
		RecentURLs: []SitemapEntry{
			{URL: "https://createos.sh/blogs/ai-agent-deployment-guide", Title: "AI agent deployment guide"},
		},
	}
	topics := []TopicSummary{
		{
			Competitor:   "vercel",
			Name:         "AI agent deployment",
			PageCount:    6,
			EvidenceURLs: []string{"https://vercel.com/docs/ai/agent/deployment"},
			WhyItMatters: "Covered-ish topic.",
		},
		{
			Competitor:   "vercel",
			Name:         "MCP security hardening",
			PageCount:    6,
			EvidenceURLs: []string{"https://vercel.com/docs/mcp/security"},
			WhyItMatters: "Less covered topic.",
		},
	}

	opportunities := deriveTopicOpportunities(ours, topics)
	require.Len(t, opportunities, 1)
	require.Equal(t, "CreateOS should cover \"MCP security hardening\"", opportunities[0].Title)

	coveredScore := scoreLLMTopicGap(6, 1, 3)
	uncoveredScore := scoreLLMTopicGap(6, 3, 3)
	require.Greater(t, uncoveredScore, coveredScore)
}

func TestDeriveTopicOpportunitiesCapsUnsupportedPageCountAndAvoidsPerfectGenericScores(t *testing.T) {
	ours := SiteSnapshot{Name: "createos"}
	topics := []TopicSummary{
		{
			Competitor:   "replit",
			Name:         "Strategic Partnerships",
			PageCount:    25,
			EvidenceURLs: []string{"https://replit.com/partners/google"},
			WhyItMatters: "Partnership positioning.",
		},
	}

	opportunities := deriveTopicOpportunities(ours, topics)

	require.Len(t, opportunities, 1)
	require.Less(t, opportunities[0].ImpactScore, 100)
}

func TestDeriveTopicOpportunitiesCapsBroadTwoEvidenceTopicsBelowPerfectScore(t *testing.T) {
	ours := SiteSnapshot{Name: "createos"}
	topics := []TopicSummary{
		{
			Competitor:   "replit",
			Name:         "Rapid Prototyping & MVP",
			PageCount:    2,
			EvidenceURLs: []string{"https://replit.com/usecases/rapid-prototyping", "https://replit.com/usecases/product-managers"},
			WhyItMatters: "Time to market angle.",
		},
	}

	opportunities := deriveTopicOpportunities(ours, topics)

	require.Len(t, opportunities, 1)
	require.Less(t, opportunities[0].ImpactScore, 100)
}

func TestLLMTopicThemeUsesTopicSemantics(t *testing.T) {
	require.Equal(t, "vibecoding", llmTopicTheme("Rapid Prototyping & MVP"))
	require.Equal(t, "agents", llmTopicTheme("AI Coding Tools & Agentic Workflows"))
	require.Equal(t, "integrations", llmTopicTheme("Ecosystem & Partner Growth"))
	require.Equal(t, "security", llmTopicTheme("Security & Codebase Integrity"))
	require.Equal(t, "ai", llmTopicTheme("AI Gateway & Model Infrastructure"))
	require.Equal(t, "comparison", llmTopicTheme("Competitive Comparison & Benchmarking"))
	require.Equal(t, "enterprise", llmTopicTheme("Enterprise AI Development"))
	require.Equal(t, "usecases", llmTopicTheme("Industry-Specific Use Case Guides"))
}

func TestEvidenceQualityPenalizesMixedAndGenericEvidence(t *testing.T) {
	high := evidenceQualityForTopic(TopicSummary{
		Name: "Rapid Prototyping & MVP",
		EvidenceURLs: []string{
			"https://replit.com/usecases/rapid-prototyping",
			"https://replit.com/usecases/product-managers",
		},
		RepresentativeTitles: []string{
			"Rapid Prototyping with AI: Quickly Move From Idea to MVP",
			"AI for Product Managers: From PRD to Prototype",
		},
	})
	low := evidenceQualityForTopic(TopicSummary{
		Name: "Platform Security & Reliability",
		EvidenceURLs: []string{
			"https://vercel.com/blog/introducing-deepsec-find-and-fix-vulnerabilities-in-your-code-base",
			"https://vercel.com/docs/rest-api/project-routes/promote-restore-or-discard-a-routing-rule-version",
		},
		RepresentativeTitles: []string{
			"Introducing deepsec: The security harness for finding vulnerabilities in your codebase",
			"Promote, restore, or discard a routing rule version",
		},
	})

	require.Greater(t, high.Score, low.Score)
	require.Equal(t, 0, high.MixedEvidencePenalty)
	require.Greater(t, low.MixedEvidencePenalty, 0)
}

func TestDeriveTopicOpportunitiesIncludesScoreBreakdown(t *testing.T) {
	ours := SiteSnapshot{Name: "createos"}
	topics := []TopicSummary{
		{
			Competitor:           "replit",
			Name:                 "Rapid Prototyping & MVP",
			PageCount:            2,
			EvidenceURLs:         []string{"https://replit.com/usecases/rapid-prototyping", "https://replit.com/usecases/product-managers"},
			RepresentativeTitles: []string{"Rapid Prototyping with AI", "AI for Product Managers"},
			WhyItMatters:         "Time to market angle.",
		},
	}

	opportunities, debug := deriveTopicOpportunitiesWithDebug(ours, topics)

	require.Len(t, opportunities, 1)
	require.Len(t, debug.ScoredTopics, 1)
	require.Equal(t, opportunities[0].ImpactScore, debug.ScoredTopics[0].Score)
	require.NotZero(t, debug.ScoredTopics[0].EvidenceQualityScore)
	require.NotZero(t, debug.ScoredTopics[0].ScoreBreakdown.EvidenceQuality)
	require.Equal(t, "vibecoding", debug.ScoredTopics[0].Theme)
}

func TestDeriveTopicOpportunitiesDebugsSkippedSinglePageTopics(t *testing.T) {
	ours := SiteSnapshot{Name: "createos"}
	topics := []TopicSummary{
		{
			Competitor:   "vercel",
			Name:         "Security & Vulnerability Management",
			PageCount:    1,
			EvidenceURLs: []string{"https://vercel.com/blog/security"},
			WhyItMatters: "Enterprise trust.",
		},
	}

	opportunities, debug := deriveTopicOpportunitiesWithDebug(ours, topics)

	require.Empty(t, opportunities)
	require.Len(t, debug.SkippedTopics, 1)
	require.Equal(t, "single-page-topic", debug.SkippedTopics[0].Reason)
	require.Equal(t, "security", debug.SkippedTopics[0].Theme)
}

func TestDeriveTopicOpportunitiesBuildsActionableWhatToDo(t *testing.T) {
	ours := SiteSnapshot{Name: "createos"}
	topics := []TopicSummary{
		{
			Competitor:           "lovable",
			Name:                 "AI Tool Comparison & Benchmarking",
			PageCount:            3,
			EvidenceURLs:         []string{"https://lovable.dev/guides/cursor-vs-bolt-vs-lovable-comparison", "https://lovable.dev/guides/claude-vs-lovable-ai-platform-comparison"},
			RepresentativeTitles: []string{"Cursor vs Bolt vs Lovable 2026: Which AI Builder Wins?", "Claude vs Lovable: Which AI Platform Wins in 2026?"},
			WhyItMatters:         "Directly addresses which tool to choose.",
		},
	}

	opportunities := deriveTopicOpportunities(ours, topics)

	require.Len(t, opportunities, 1)
	require.Contains(t, opportunities[0].WhatToDo, "comparison page")
	require.Contains(t, opportunities[0].WhatToDo, "/compare/ai-tool-comparison-benchmarking")
	require.NotContains(t, opportunities[0].WhatToDo, "Ship one focused page or article")
	require.Contains(t, strings.Join(opportunities[0].HowToExecute, " "), "Cursor vs Bolt vs Lovable")
}

func TestBuildContentRecommendationsCreatesClusterPlan(t *testing.T) {
	opportunities := []Opportunity{
		{
			Title:           "CreateOS should cover \"AI Tool Comparison & Benchmarking\"",
			WhyItMatters:    "High-intent comparison traffic.",
			ImpactScore:     86,
			Competitor:      "lovable",
			Theme:           "comparison",
			OpportunityType: "llm-topic-gap",
			Evidence:        []string{"https://lovable.dev/guides/cursor-vs-bolt-vs-lovable-comparison"},
		},
		{
			Title:           "CreateOS should cover \"AI-Driven Use Cases\"",
			WhyItMatters:    "Persona-led use-case demand.",
			ImpactScore:     85,
			Competitor:      "replit",
			Theme:           "usecases",
			OpportunityType: "llm-topic-gap",
			Evidence:        []string{"https://replit.com/usecases/product-managers"},
		},
	}

	recommendations := buildContentRecommendations(opportunities)

	require.Len(t, recommendations, 2)
	require.Equal(t, 1, recommendations[0].Priority)
	require.Equal(t, "comparison page", recommendations[0].PageType)
	require.Equal(t, "/compare/ai-tool-comparison-benchmarking", recommendations[0].SuggestedSlug)
	require.NotEmpty(t, recommendations[0].ClusterPages)
	require.Equal(t, "use-case landing page", recommendations[1].PageType)
	require.Contains(t, recommendations[1].SuggestedSlug, "/use-cases/")
}

func TestRecommendationCopyAvoidsDuplicatedIntentAndPreservesAcronyms(t *testing.T) {
	topic := TopicSummary{
		Competitor:   "replit",
		Name:         "AI-Powered Rapid Prototyping",
		PageCount:    3,
		EvidenceURLs: []string{"https://replit.com/usecases/rapid-prototyping", "https://replit.com/usecases/product-managers"},
	}
	theme := llmTopicTheme(topic.Name)

	require.Equal(t, "vibecoding", theme)
	require.Contains(t, topicWhatToDo(topic, theme), "AI-Powered Rapid Prototyping")
	require.NotContains(t, topicWhatToDo(topic, theme), "intent intent")
	require.Equal(t, "AI-Powered Creative And Business Guides", guideTitleFromTopic("AI-Powered Creative & Business Guides"))
}

func TestTopicExecutionPlanUsesSafeSlug(t *testing.T) {
	plan := topicExecutionPlan(TopicSummary{Name: "AI-Driven Development & Agentic Workflows"})

	require.Contains(t, plan[0], "/blogs/ai-driven-development-agentic-workflows")
	require.NotContains(t, plan[0], "&")
}

func TestRelevantPromptPageFiltersNonActionablePages(t *testing.T) {
	require.False(t, isRelevantPromptPage("lovable", "https://lovable.dev/cookie-policy", "Lovable Cookie Policy", false))
	require.False(t, isRelevantPromptPage("lovable", "https://lovable.dev/consultancy-services-terms", "Lovable Consultancy Services Terms", false))
	require.False(t, isRelevantPromptPage("vercel", "https://vercel.com/docs/errors", "Error Codes", false))
	require.False(t, isRelevantPromptPage("replit", "https://replit.com/birthday", "Replit turns 10! Bring your app to life for FREE", false))
	require.False(t, isRelevantPromptPage("replit", "https://replit.com/partners/stripe", "Access Private Deployment", false))

	require.True(t, isRelevantPromptPage("lovable", "https://lovable.dev/designers", "AI Tools for Designers", false))
	require.True(t, isRelevantPromptPage("replit", "https://replit.com/usecases/rapid-prototyping", "Rapid Prototyping with AI", false))
	require.True(t, isRelevantPromptPage("vercel", "https://vercel.com/i/llm-agent", "What is an LLM agent?", false))
}

func TestInferDateFromURL(t *testing.T) {
	dt := inferDateFromURL("https://example.com/blog/2026/04/29/agent-release")
	require.NotNil(t, dt)
	require.Equal(t, "2026-04-29", dt.Format("2006-01-02"))

	dt = inferDateFromURL("https://example.com/blog/agent-release-2026-04-15")
	require.NotNil(t, dt)
	require.Equal(t, "2026-04-15", dt.Format("2006-01-02"))

	dt = inferDateFromURL("https://example.com/docs/agent-release")
	require.Nil(t, dt)
}

func TestBuildSnapshotSkipsUndatedEntries(t *testing.T) {
	windowStart := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	entries := []rawSitemapEntry{
		{URL: "https://createos.sh/blog/agents"},
		{URL: "https://createos.sh/blog/2026/04/21/agents"},
	}

	snapshot := buildSnapshot("createos", "https://createos.sh/sitemap.xml", entries, windowStart)
	require.Equal(t, 2, snapshot.TotalURLs)
	require.Equal(t, 1, snapshot.RecentURLCount)
}
