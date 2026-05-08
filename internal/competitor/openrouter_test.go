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
				"titleOptions": ["AI Coding Got Faster. Shipping Still Got Messier.", "Rapid Prototyping Still Needs an Execution Layer"],
				"selectedTitleReason": "It uses tension while preserving the rapid prototyping intent.",
				"metaDescription": "Build and validate MVPs faster with CreateOS.",
				"bodyMarkdown": "# Rapid Prototyping & MVP with CreateOS\n\nDraft body.",
				"internalLinks": [
					{"anchorText": "unified execution layer", "targetPath": "/", "placement": "intro", "reason": "Connects the article to the product positioning.", "status": "existing"}
				],
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
	require.Contains(t, out.Drafts[0].TitleOptions, "AI Coding Got Faster. Shipping Still Got Messier.")
	require.Equal(t, "It uses tension while preserving the rapid prototyping intent.", out.Drafts[0].SelectedTitleReason)
	require.Equal(t, "unified execution layer", out.Drafts[0].InternalLinks[0].AnchorText)
	require.Equal(t, "/", out.Drafts[0].InternalLinks[0].TargetPath)
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
	prompt := blogDraftBodyUserPrompt(
		blogDraftPromptItem{Route: "/blogs/test", Title: "Test Draft"},
		BlogDraft{Route: "/blogs/test", Title: "Test Draft"},
		[]byte(`{"recommendations":[]}`),
		"CreateOS is the workspace where ideas become applications.",
		"Use Naman/CreateOS voice. Ban hype phrases like revolutionary and game-changing.",
	)

	require.Contains(t, prompt, "polished blog prose")
	require.Contains(t, prompt, "not an outline")
	require.Contains(t, prompt, "Use bullets sparingly")
	require.Contains(t, prompt, "Each H2 section should have 2-4 paragraphs")
	require.Contains(t, prompt, "Use the CreateOS context as positioning guidance")
	require.Contains(t, prompt, "CreateOS is the workspace where ideas become applications.")
	require.Contains(t, prompt, "Use the CreateOS writing guidelines as style and quality rules")
	require.Contains(t, prompt, "Naman/CreateOS voice")
	require.Contains(t, prompt, "content-repo-ready")
	require.Contains(t, prompt, "honest tradeoffs section")
	require.Contains(t, prompt, "Do not use em dashes")
	require.Contains(t, prompt, "markdown only")
	require.Contains(t, prompt, "Do not create external citation plans or third-party backlink outreach ideas")
}

func TestBlogDraftBriefPromptRequestsSmallJSONOnly(t *testing.T) {
	prompt := blogDraftBriefUserPrompt(
		[]byte(`{"recommendations":[]}`),
		"CreateOS is the workspace where ideas become applications.",
		"Use Naman/CreateOS voice.",
	)

	require.Contains(t, prompt, "small SEO brief")
	require.Contains(t, prompt, "Do not generate bodyMarkdown")
	require.Contains(t, prompt, "titleOptions")
	require.Contains(t, prompt, "selectedTitleReason")
	require.Contains(t, prompt, "internalLinks")
	require.Contains(t, prompt, "Every internalLinks targetPath must exactly match a path from internalLinkCandidates")
	require.Contains(t, prompt, "Use status=existing for every internal link")
	require.Contains(t, prompt, "Do not create planned links")
	require.Contains(t, prompt, "Do not invent future routes")
	require.Contains(t, prompt, "JSON only")
}

func TestNormalizeBlogDraftBriefKeepsOnlyCandidateLinks(t *testing.T) {
	item := blogDraftPromptItem{
		InternalLinkCandidates: []InternalLinkCandidate{
			{Path: "/blogs/createos-single-intelligent-workspace", URL: "https://createos.sh/blogs/createos-single-intelligent-workspace"},
		},
	}
	draft := BlogDraft{
		Route: "/use-cases/rapid-prototyping-mvp",
		Title: "From Idea to Working MVP",
		InternalLinks: []SEOLinkSuggestion{
			{AnchorText: "single intelligent workspace", TargetPath: "/blogs/createos-single-intelligent-workspace", Status: "existing"},
			{AnchorText: "marketplace distribution", TargetPath: "/use-cases/monetize-applications", Status: "planned"},
		},
	}

	normalized := normalizeBlogDraftBriefForItem(draft, item)

	require.Len(t, normalized.InternalLinks, 1)
	require.Equal(t, "/blogs/createos-single-intelligent-workspace", normalized.InternalLinks[0].TargetPath)
	require.Equal(t, "existing", normalized.InternalLinks[0].Status)
}

func TestNormalizeBlogDraftsKeepsTitleOptions(t *testing.T) {
	drafts := normalizeBlogDrafts([]BlogDraft{
		{
			Route:               "/blogs/context-switching",
			Title:               "AI Coding Got Faster. Shipping Still Got Messier.",
			TitleOptions:        []string{"AI Coding Got Faster. Shipping Still Got Messier.", "", "Why Context Switching Still Slows Builders Down", "AI Coding Got Faster. Shipping Still Got Messier."},
			SelectedTitleReason: "The title creates tension and keeps the execution angle.",
			BodyMarkdown:        "# Draft\n\nBody.",
			InternalLinks: []SEOLinkSuggestion{
				{AnchorText: "CreateOS services", TargetPath: "/services", Status: "existing"},
				{AnchorText: "", TargetPath: "/blogs", Status: "existing"},
			},
		},
	}, 1)

	require.Len(t, drafts, 1)
	require.Equal(t, []string{"AI Coding Got Faster. Shipping Still Got Messier.", "Why Context Switching Still Slows Builders Down"}, drafts[0].TitleOptions)
	require.Equal(t, "The title creates tension and keeps the execution angle.", drafts[0].SelectedTitleReason)
	require.Len(t, drafts[0].InternalLinks, 1)
	require.Equal(t, "existing", drafts[0].InternalLinks[0].Status)
}

func TestNormalizeBlogDraftsEmbedsExistingInternalLinksInBody(t *testing.T) {
	drafts := normalizeBlogDrafts([]BlogDraft{
		{
			Route:        "/use-cases/rapid-prototyping-mvp",
			Title:        "From Sketch to Shipped",
			BodyMarkdown: "# From Sketch to Shipped\n\nCreateOS gives builders one execution layer for moving from idea to deployed product.",
			InternalLinks: []SEOLinkSuggestion{
				{AnchorText: "execution layer", TargetPath: "/", Status: "existing"},
				{AnchorText: "CreateOS Marketplace", TargetPath: "/marketplace", Status: "planned"},
				{AnchorText: "case studies", TargetPath: "/case-studies", Status: "existing"},
			},
		},
	}, 1)

	require.Len(t, drafts, 1)
	require.Contains(t, drafts[0].BodyMarkdown, "[execution layer](https://createos.sh/)")
	require.Contains(t, drafts[0].BodyMarkdown, "[case studies](https://createos.sh/case-studies)")
	require.NotContains(t, drafts[0].BodyMarkdown, "https://createos.sh/marketplace")
}

func TestNormalizeBlogDraftsDoesNotDoubleWrapExistingMarkdownLinks(t *testing.T) {
	drafts := normalizeBlogDrafts([]BlogDraft{
		{
			Route:        "/solutions/enterprise-security-compliance",
			Title:        "Enterprise Security Is Not a Feature List",
			BodyMarkdown: "Teams need [container security posture](/blogs/container-security-on-nodeops-network-compute) across runtime environments.",
			InternalLinks: []SEOLinkSuggestion{
				{AnchorText: "container security posture", TargetPath: "/blogs/container-security-on-nodeops-network-compute", Status: "existing"},
			},
		},
	}, 1)

	require.Len(t, drafts, 1)
	require.Contains(t, drafts[0].BodyMarkdown, "[container security posture](/blogs/container-security-on-nodeops-network-compute)")
	require.NotContains(t, drafts[0].BodyMarkdown, "[[container security posture]")
	require.NotContains(t, drafts[0].BodyMarkdown, ")](/blogs/container-security-on-nodeops-network-compute)")
}

func TestNormalizeBlogDraftsStripsUnapprovedMarkdownLinks(t *testing.T) {
	drafts := normalizeBlogDrafts([]BlogDraft{
		{
			Route:        "/use-cases/rapid-prototyping-mvp",
			Title:        "From Idea to Working MVP",
			BodyMarkdown: "Explore [marketplace distribution](/use-cases/monetize-applications) after launch and [single intelligent workspace](/blogs/createos-single-intelligent-workspace).",
			InternalLinks: []SEOLinkSuggestion{
				{AnchorText: "single intelligent workspace", TargetPath: "/blogs/createos-single-intelligent-workspace", Status: "existing"},
			},
		},
	}, 1)

	require.Len(t, drafts, 1)
	require.Contains(t, drafts[0].BodyMarkdown, "marketplace distribution")
	require.NotContains(t, drafts[0].BodyMarkdown, "](/use-cases/monetize-applications)")
	require.Contains(t, drafts[0].BodyMarkdown, "[single intelligent workspace](/blogs/createos-single-intelligent-workspace)")
}

func TestDraftPromptInputIncludesRelevantCreateOSInternalLinkCandidates(t *testing.T) {
	recommendations := []ContentRecommendation{
		{
			SuggestedSlug:  "/use-cases/rapid-prototyping-mvp",
			SuggestedTitle: "Rapid Prototyping & MVP",
			Theme:          "vibecoding",
			ContentAngle:   "Move from idea to shipped MVP without tool sprawl.",
		},
	}
	inventory := []InternalLinkCandidate{
		{URL: "https://createos.sh/blogs/solo-founder-workflow-context-switching", Path: "/blogs/solo-founder-workflow-context-switching", Title: "Solo founder workflow context switching", PageType: "blog", Category: "developer-productivity-tool-sprawl", Score: 97},
		{URL: "https://createos.sh/blogs/nodeops-network-mint-and-burn-strategies", Path: "/blogs/nodeops-network-mint-and-burn-strategies", Title: "NodeOps mint and burn strategies", PageType: "blog", Category: "penalized-nodeops-web3", Score: 18},
		{URL: "https://createos.sh/case-studies/justref", Path: "/case-studies/justref", Title: "JustRef", PageType: "case-study", Category: "case-study", Score: 65},
	}

	input := draftPromptInput(recommendations, 1, inventory)

	require.Len(t, input, 1)
	require.Equal(t, "/blogs/solo-founder-workflow-context-switching", input[0].InternalLinkCandidates[0].Path)
	require.Equal(t, "/case-studies/justref", input[0].InternalLinkCandidates[1].Path)
	require.NotContains(t, internalLinkCandidatePaths(input[0].InternalLinkCandidates), "/blogs/nodeops-network-mint-and-burn-strategies")
}

func TestDraftPromptInputCapsInternalLinkCandidatesAtTwenty(t *testing.T) {
	recommendations := []ContentRecommendation{{SuggestedSlug: "/blogs/test", SuggestedTitle: "Test Draft"}}
	inventory := make([]InternalLinkCandidate, 0, 25)
	for idx := 0; idx < 25; idx++ {
		inventory = append(inventory, InternalLinkCandidate{
			URL:      "https://createos.sh/blogs/test",
			Path:     "/blogs/test",
			Title:    "Test",
			PageType: "blog",
			Score:    80 - idx,
		})
	}

	input := draftPromptInput(recommendations, 1, inventory)

	require.Len(t, input, 1)
	require.Len(t, input[0].InternalLinkCandidates, 20)
}

func TestOpenRouterTransientErrorsAreRetryable(t *testing.T) {
	require.True(t, isTransientOpenRouterError("decode openrouter blog draft response: stream error: stream ID 3; INTERNAL_ERROR; received from peer"))
	require.True(t, isTransientOpenRouterError("unexpected EOF"))
	require.True(t, isTransientOpenRouterError("context deadline exceeded"))
	require.True(t, isTransientOpenRouterError("openrouter chat response returned empty content"))
	require.True(t, isTransientOpenRouterError("openrouter chat response returned no choices"))
	require.False(t, isTransientOpenRouterError("unmarshal openrouter blog draft json: invalid character"))
}

func TestOpenRouterModelChainUsesPrimaryAndFallback(t *testing.T) {
	require.Equal(t, []string{"gemini/gemini-3.1-flash", "deepseek/deepseek-chat"}, openRouterModelChain("gemini/gemini-3.1-flash", "deepseek/deepseek-chat"))
	require.Equal(t, []string{"moonshotai/kimi-k2"}, openRouterModelChain("moonshotai/kimi-k2", "moonshotai/kimi-k2"))
	require.Equal(t, []string{"moonshotai/kimi-k2"}, openRouterModelChain("", "moonshotai/kimi-k2"))
}

func TestBuildInternalLinkInventoryScoresCreateOSSitemapCategories(t *testing.T) {
	entries := []rawSitemapEntry{
		{URL: "https://createos.sh/"},
		{URL: "https://createos.sh/blogs/solo-founder-workflow-context-switching"},
		{URL: "https://createos.sh/blogs/deploy-an-api-in-2-minutes-with-createos-cli"},
		{URL: "https://createos.sh/blogs/staking-pol-with-nodeops-staking-hub"},
		{URL: "https://createos.sh/case-studies/justref"},
	}
	recommendations := []ContentRecommendation{
		{SuggestedTitle: "Rapid Prototyping & MVP", Theme: "vibecoding", ContentAngle: "solo founder workflow from idea to deployment"},
	}

	inventory := buildCreateOSInternalLinkInventory(entries, recommendations)

	require.NotEmpty(t, inventory)
	require.Equal(t, "https://createos.sh/blogs/solo-founder-workflow-context-switching", inventory[0].URL)
	require.Greater(t, findInternalLinkCandidate(t, inventory, "/blogs/solo-founder-workflow-context-switching").Score, findInternalLinkCandidate(t, inventory, "/blogs/staking-pol-with-nodeops-staking-hub").Score)
	require.Equal(t, "developer-productivity-tool-sprawl", findInternalLinkCandidate(t, inventory, "/blogs/solo-founder-workflow-context-switching").Category)
	require.Equal(t, "deployment-cli-devops", findInternalLinkCandidate(t, inventory, "/blogs/deploy-an-api-in-2-minutes-with-createos-cli").Category)
	require.Equal(t, "penalized-nodeops-web3", findInternalLinkCandidate(t, inventory, "/blogs/staking-pol-with-nodeops-staking-hub").Category)
	require.Equal(t, "case-study", findInternalLinkCandidate(t, inventory, "/case-studies/justref").PageType)
}

func findInternalLinkCandidate(t *testing.T, inventory []InternalLinkCandidate, path string) InternalLinkCandidate {
	t.Helper()
	for _, candidate := range inventory {
		if candidate.Path == path {
			return candidate
		}
	}
	require.FailNowf(t, "missing internal link candidate", "path=%s", path)
	return InternalLinkCandidate{}
}

func internalLinkCandidatePaths(inventory []InternalLinkCandidate) []string {
	out := make([]string, 0, len(inventory))
	for _, candidate := range inventory {
		out = append(out, candidate.Path)
	}
	return out
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
