package competitor

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLLMTopicOutputParsesTopicsArrayShape(t *testing.T) {
	raw := `{
		"topics": [
			{
				"competitor": "vercel",
				"name": "agent deployment workflows",
				"pageCount": 7,
				"representativeTitles": ["Deploy AI Agents on Edge", "Agent CI/CD for Teams"],
				"evidenceUrls": ["https://vercel.com/blog/agents-edge", "https://vercel.com/docs/agents-cicd"],
				"whyItMatters": "High-intent implementation cluster with repeated shipping cadence."
			}
		]
	}`

	var out llmTopicOutput
	err := json.Unmarshal([]byte(raw), &out)

	require.NoError(t, err)
	require.Len(t, out.Topics, 1)
	require.Equal(t, "vercel", out.Topics[0].Competitor)
	require.Equal(t, "agent deployment workflows", out.Topics[0].Name)
	require.Equal(t, 7, out.Topics[0].PageCount)
	require.Len(t, out.Topics[0].RepresentativeTitles, 2)
	require.Len(t, out.Topics[0].EvidenceURLs, 2)
}

func TestLLMBlogDraftOutputParsesDraftsArrayShape(t *testing.T) {
	raw := `{
		"drafts": [
			{
				"route": "/use-cases/rapid-prototyping-mvp",
				"title": "Rapid Prototyping & MVP with CreateOS",
				"metaDescription": "Build and validate MVPs faster with CreateOS.",
				"bodyMarkdown": "# Rapid Prototyping & MVP with CreateOS\n\nDraft body.",
				"cta": "Start building with CreateOS.",
				"status": "ai-generated-draft"
			}
		]
	}`

	var out llmBlogDraftOutput
	err := json.Unmarshal([]byte(raw), &out)

	require.NoError(t, err)
	require.Len(t, out.Drafts, 1)
	require.Equal(t, "/use-cases/rapid-prototyping-mvp", out.Drafts[0].Route)
	require.Equal(t, "Rapid Prototyping & MVP with CreateOS", out.Drafts[0].Title)
	require.Contains(t, out.Drafts[0].BodyMarkdown, "Draft body")
}

func TestAttachDraftsToContentRecommendationsCapsTopN(t *testing.T) {
	recommendations := []ContentRecommendation{
		{Priority: 1, SuggestedSlug: "/one"},
		{Priority: 2, SuggestedSlug: "/two"},
		{Priority: 3, SuggestedSlug: "/three"},
	}
	drafts := []BlogDraft{
		{Route: "/one", Title: "One", BodyMarkdown: "# One"},
		{Route: "/two", Title: "Two", BodyMarkdown: "# Two"},
		{Route: "/three", Title: "Three", BodyMarkdown: "# Three"},
	}

	out := attachDraftsToContentRecommendations(recommendations, drafts, 2)

	require.NotNil(t, out[0].Draft)
	require.NotNil(t, out[1].Draft)
	require.Nil(t, out[2].Draft)
	require.Equal(t, "ai-generated-draft", out[0].Draft.Status)
}

func TestBlogDraftPromptRequestsProseNotOutline(t *testing.T) {
	prompt := blogDraftUserPrompt([]byte(`{"recommendations":[]}`), "CreateOS is the workspace where ideas become applications.")

	require.Contains(t, prompt, "polished blog prose")
	require.Contains(t, prompt, "not an outline")
	require.Contains(t, prompt, "Use bullets sparingly")
	require.Contains(t, prompt, "Each H2 section should have 2-4 paragraphs")
	require.Contains(t, prompt, "Use the CreateOS context as positioning guidance")
	require.Contains(t, prompt, "CreateOS is the workspace where ideas become applications.")
}

func TestBuildTopicPromptInputUsesTitlesAndCapsLimit(t *testing.T) {
	competitors := []SiteSnapshot{
		{
			Name: "vercel",
			RecentURLs: []SitemapEntry{
				{URL: "https://vercel.com/i/llm-agent", Title: "What is an LLM agent? A developer's guide"},
				{URL: "https://vercel.com/i/how-ai-is-changing-seo", Title: "How AI is changing SEO"},
				{URL: "https://vercel.com/3", Title: ""},
			},
		},
	}

	out := buildTopicPromptInput(competitors, 1)

	require.Len(t, out, 1)
	require.Equal(t, "vercel", out[0].Competitor)
	require.Len(t, out[0].Pages, 1)
	require.Equal(t, "What is an LLM agent? A developer's guide", out[0].Pages[0].Title)
	require.Equal(t, "https://vercel.com/i/llm-agent", out[0].Pages[0].URL)
}

func TestBuildTopicPromptInputSkipsLowValuePagesBeforeLimit(t *testing.T) {
	competitors := []SiteSnapshot{
		{
			Name: "lovable",
			RecentURLs: []SitemapEntry{
				{URL: "https://lovable.dev/careers/account-executive", Title: "Account Executive - Lovable Careers"},
				{URL: "https://lovable.dev/brand", Title: "Lovable Press Enquiries"},
				{URL: "https://lovable.dev/cookie-policy", Title: "Lovable Cookie Policy"},
				{URL: "https://lovable.dev/consultancy-services-terms", Title: "Lovable Consultancy Services Terms"},
				{URL: "https://lovable.dev/blog", Title: "Blog - Lovable"},
				{URL: "https://lovable.dev/bolt-vs-lovable", Title: "Comparing Bolt vs. Lovable"},
				{URL: "https://lovable.dev/ailpbuilder", Title: "Build Landing Pages in Minutes with AI"},
			},
		},
	}

	out := buildTopicPromptInput(competitors, 2)

	require.Len(t, out, 1)
	require.Equal(t, "Comparing Bolt vs. Lovable", out[0].Pages[0].Title)
	require.Equal(t, "https://lovable.dev/bolt-vs-lovable", out[0].Pages[0].URL)
	require.Equal(t, "comparison", out[0].Pages[0].PageType)
	require.Len(t, out[0].Pages, 2)
	require.Greater(t, out[0].Pages[0].RelevanceScore, out[0].Pages[1].RelevanceScore)
}

func TestBuildTopicPromptInputDebugCountsLowValueBeforeMissingTitle(t *testing.T) {
	competitors := []SiteSnapshot{
		{
			Name: "replit",
			RecentURLs: []SitemapEntry{
				{URL: "https://replit.com/partners/stripe", Title: "Access Private Deployment"},
				{URL: "https://replit.com/edu/events", Title: ""},
				{URL: "https://replit.com/usecases/product-managers", Title: "AI for Product Managers: From PRD to Prototype"},
			},
		},
	}

	out, debug := buildTopicPromptInputWithDebug(competitors, 10)

	require.Len(t, out, 1)
	require.Equal(t, "https://replit.com/usecases/product-managers", out[0].Pages[0].URL)
	require.Len(t, debug, 1)
	require.Equal(t, 1, debug[0].PagesSent)
	require.Equal(t, 0, debug[0].SkippedNoTitle)
	require.Equal(t, 2, debug[0].SkippedLowValue)
}

func TestBuildPageCandidatesScoresAndRanksActionablePages(t *testing.T) {
	snapshot := SiteSnapshot{
		Name: "lovable",
		RecentURLs: []SitemapEntry{
			{URL: "https://lovable.dev/cookie-policy", Title: "Lovable Cookie Policy"},
			{URL: "https://lovable.dev/guides/best-ai-app-builders", Title: "Best AI App Builders in 2026: Top 6 Tools Compared"},
			{URL: "https://lovable.dev/bolt-vs-lovable", Title: "Comparing Bolt vs. Lovable"},
			{URL: "https://lovable.dev/", Title: "AI App Builder | Vibe Code Apps & Websites with AI, Fast"},
		},
	}

	candidates := buildPageCandidates(snapshot)

	require.Len(t, candidates, 4)
	require.Contains(t, []string{
		"https://lovable.dev/bolt-vs-lovable",
		"https://lovable.dev/guides/best-ai-app-builders",
	}, candidates[0].URL)
	require.Equal(t, "comparison", candidates[0].PageType)
	require.Contains(t, candidates[0].PositiveSignals, "comparison")
	require.Greater(t, candidates[0].RelevanceScore, candidates[3].RelevanceScore)
	require.Equal(t, "legal", candidates[3].PageType)
	require.Contains(t, candidates[3].NegativeSignals, "low-value-path")
}

func TestBuildTopicPromptInputIncludesCandidateContext(t *testing.T) {
	competitors := []SiteSnapshot{
		{
			Name: "replit",
			RecentURLs: []SitemapEntry{
				{URL: "https://replit.com/usecases/rapid-prototyping", Title: "Rapid Prototyping with AI: Quickly Move From Idea to MVP"},
			},
		},
	}

	out, debug := buildTopicPromptInputWithDebug(competitors, 10)

	require.Len(t, out, 1)
	require.Equal(t, "usecase", out[0].Pages[0].PageType)
	require.GreaterOrEqual(t, out[0].Pages[0].RelevanceScore, promptCandidateThreshold)
	require.Contains(t, out[0].Pages[0].WhySelected, "usecase")
	require.Len(t, debug, 1)
	require.NotEmpty(t, debug[0].SelectedPages)
	require.Equal(t, "https://replit.com/usecases/rapid-prototyping", debug[0].SelectedPages[0].URL)
}

func TestTitleEnrichmentIndexesSelectsURLOnlyCandidates(t *testing.T) {
	snapshot := SiteSnapshot{
		Name: "vercel",
		RecentURLs: []SitemapEntry{
			{URL: "https://vercel.com/careers/backend-engineer"},
			{URL: "https://vercel.com/ai-gateway/models"},
			{URL: "https://vercel.com/i/llm-agent"},
		},
	}

	indexes := titleEnrichmentIndexes(snapshot, 10)

	require.ElementsMatch(t, []int{1, 2}, indexes)
}

func TestNormalizeTopicSummariesTrimsDropsInvalidAndCapsEvidence(t *testing.T) {
	topics := []TopicSummary{
		{
			Competitor: "  vercel ",
			Name:       "  agent workflows ",
			PageCount:  25,
			RepresentativeTitles: []string{
				" A ", "B", "C", "D", "E", "F",
			},
			EvidenceURLs: []string{
				" https://a ", "https://b", "https://c", "https://d", "https://e", "https://f",
			},
			WhyItMatters: "  repeated demand  ",
		},
		{
			Competitor: " ",
			Name:       "invalid",
		},
		{
			Competitor: "replit",
			Name:       " ",
		},
	}

	out := normalizeTopicSummaries(topics)

	require.Len(t, out, 1)
	require.Equal(t, "vercel", out[0].Competitor)
	require.Equal(t, "agent workflows", out[0].Name)
	require.Equal(t, 5, out[0].PageCount)
	require.Equal(t, []string{"A", "B", "C", "D", "E"}, out[0].RepresentativeTitles)
	require.Equal(t, []string{"https://a", "https://b", "https://c", "https://d", "https://e"}, out[0].EvidenceURLs)
	require.Equal(t, "repeated demand", out[0].WhyItMatters)
}

func TestBuildTopicPromptInputReturnsEmptyWhenNoTitles(t *testing.T) {
	competitors := []SiteSnapshot{
		{
			Name: "vercel",
			RecentURLs: []SitemapEntry{
				{URL: "https://vercel.com/1", Title: ""},
				{URL: "https://vercel.com/2", Title: " "},
			},
		},
	}

	out := buildTopicPromptInput(competitors, 40)
	require.Empty(t, out)
}

func TestTrimTopicPromptInputToBytesStaysUnderCap(t *testing.T) {
	longTitle := strings.Repeat("A", 500)
	longURL := "https://example.com/" + strings.Repeat("segment-", 20)
	input := []topicPromptCompetitor{
		{
			Competitor: "a",
			Pages: []topicPromptPage{
				{Title: longTitle, URL: longURL + "1"},
				{Title: longTitle, URL: longURL + "2"},
				{Title: longTitle, URL: longURL + "3"},
			},
		},
		{
			Competitor: "b",
			Pages: []topicPromptPage{
				{Title: longTitle, URL: longURL + "4"},
				{Title: longTitle, URL: longURL + "5"},
				{Title: longTitle, URL: longURL + "6"},
			},
		},
	}

	trimmed, payload, err := trimTopicPromptInputToBytes(input, 2500)
	require.NoError(t, err)
	require.NotEmpty(t, trimmed)
	require.LessOrEqual(t, len(payload), 2500)
	require.Equal(t, "a", trimmed[0].Competitor)
	require.NotEmpty(t, trimmed[0].Pages)
}
