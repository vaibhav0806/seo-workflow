package competitor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const openRouterEndpoint = "https://openrouter.ai/api/v1/chat/completions"
const topicPromptPayloadByteCap = 120_000
const internalLinkCandidatePromptLimit = 20

type openRouterRequest struct {
	Model       string              `json:"model"`
	Temperature float64             `json:"temperature"`
	Messages    []openRouterMessage `json:"messages"`
}

type openRouterMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openRouterResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type llmOpportunityOutput struct {
	Opportunities []Opportunity `json:"opportunities"`
}

type llmTopicOutput struct {
	Topics []TopicSummary `json:"topics"`
}

type llmBlogDraftOutput struct {
	Drafts []BlogDraft `json:"drafts"`
}

type llmBlogDraftBriefOutput struct {
	Drafts []BlogDraft `json:"drafts"`
}

type topicPromptPage struct {
	Title          string   `json:"title"`
	URL            string   `json:"url"`
	PageType       string   `json:"pageType,omitempty"`
	RelevanceScore int      `json:"relevanceScore,omitempty"`
	WhySelected    []string `json:"whySelected,omitempty"`
}

type topicPromptCompetitor struct {
	Competitor string            `json:"competitor"`
	Pages      []topicPromptPage `json:"pages"`
}

func refineWithOpenRouter(
	ctx context.Context,
	apiKey string,
	model string,
	ours SiteSnapshot,
	competitors []SiteSnapshot,
	baseline []Opportunity,
) ([]Opportunity, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return baseline, nil
	}
	if strings.TrimSpace(model) == "" {
		model = "moonshotai/kimi-k2"
	}

	client := &http.Client{Timeout: 45 * time.Second}

	promptInput := map[string]any{
		"ours":        ours,
		"competitors": competitors,
		"baseline":    baseline,
	}
	inputBytes, err := json.Marshal(promptInput)
	if err != nil {
		return baseline, fmt.Errorf("marshal openrouter prompt input: %w", err)
	}

	systemPrompt := "You are a growth + SEO strategist. Return only strict JSON with key opportunities[]."
	userPrompt := "Given sitemap-based competitive signals, improve opportunities for CreateOS. Constraints: each item must include title, whyItMatters, whatToDo, howToExecute (3 concise steps), impactScore (1-100), competitor, theme, opportunityType, evidence (max 3). Prioritize actionable opportunities for AI/agents/vibecoding products. JSON only. Data: " + string(inputBytes)

	requestBody := openRouterRequest{
		Model:       model,
		Temperature: 0.2,
		Messages: []openRouterMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	}
	payload, err := json.Marshal(requestBody)
	if err != nil {
		return baseline, fmt.Errorf("marshal openrouter request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openRouterEndpoint, bytes.NewReader(payload))
	if err != nil {
		return baseline, fmt.Errorf("build openrouter request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return baseline, fmt.Errorf("execute openrouter request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return baseline, fmt.Errorf("openrouter status=%d body=%q", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed openRouterResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return baseline, fmt.Errorf("decode openrouter response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return baseline, fmt.Errorf("openrouter returned no choices")
	}

	content := strings.TrimSpace(parsed.Choices[0].Message.Content)
	if content == "" {
		return baseline, fmt.Errorf("openrouter returned empty content")
	}

	clean := extractJSONObject(content)
	if clean == "" {
		return baseline, fmt.Errorf("openrouter content had no json object")
	}

	var out llmOpportunityOutput
	if err := json.Unmarshal([]byte(clean), &out); err != nil {
		return baseline, fmt.Errorf("unmarshal openrouter json: %w", err)
	}
	if len(out.Opportunities) == 0 {
		return baseline, fmt.Errorf("openrouter returned zero opportunities")
	}

	for idx := range out.Opportunities {
		out.Opportunities[idx].ImpactScore = clampImpact(out.Opportunities[idx].ImpactScore)
		if len(out.Opportunities[idx].HowToExecute) > 3 {
			out.Opportunities[idx].HowToExecute = out.Opportunities[idx].HowToExecute[:3]
		}
		if len(out.Opportunities[idx].Evidence) > 3 {
			out.Opportunities[idx].Evidence = out.Opportunities[idx].Evidence[:3]
		}
	}

	return out.Opportunities, nil
}

func extractJSONObject(raw string) string {
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start == -1 || end == -1 || end <= start {
		return ""
	}
	return strings.TrimSpace(raw[start : end+1])
}

func extractTopicsWithOpenRouter(ctx context.Context, apiKey, model string, competitors []SiteSnapshot) ([]TopicSummary, []TopicPromptDebug, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, nil, nil
	}
	if strings.TrimSpace(model) == "" {
		model = "moonshotai/kimi-k2"
	}

	input, promptDebug := buildTopicPromptInputWithDebug(competitors, 40)
	if len(input) == 0 {
		return nil, promptDebug, nil
	}
	input, inputBytes, err := trimTopicPromptInputToBytes(input, topicPromptPayloadByteCap)
	if err != nil {
		return nil, promptDebug, fmt.Errorf("trim topic prompt input: %w", err)
	}
	if len(input) == 0 {
		return nil, promptDebug, nil
	}
	systemPrompt := "You are an SEO strategist. Return only strict JSON with key topics[]."
	userPrompt := "Analyze competitor page titles and URLs, then return 5-8 concrete themes per competitor. Ignore locale/translation, faq, careers/jobs, gallery, legal/privacy/terms, and company/about/team pages. Each theme item must include competitor, name, pageCount, representativeTitles, evidenceUrls, whyItMatters. JSON only. Data: " + string(inputBytes)

	requestBody := openRouterRequest{
		Model:       model,
		Temperature: 0.2,
		Messages: []openRouterMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	}
	payload, err := json.Marshal(requestBody)
	if err != nil {
		return nil, promptDebug, fmt.Errorf("marshal openrouter topic request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openRouterEndpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, promptDebug, fmt.Errorf("build openrouter topic request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, promptDebug, fmt.Errorf("execute openrouter topic request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, promptDebug, fmt.Errorf("openrouter topic status=%d body=%q", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed openRouterResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, promptDebug, fmt.Errorf("decode openrouter topic response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return nil, promptDebug, fmt.Errorf("openrouter topic response returned no choices")
	}

	content := strings.TrimSpace(parsed.Choices[0].Message.Content)
	if content == "" {
		return nil, promptDebug, fmt.Errorf("openrouter topic response returned empty content")
	}

	clean := extractJSONObject(content)
	if clean == "" {
		return nil, promptDebug, fmt.Errorf("openrouter topic response had no json object")
	}

	var out llmTopicOutput
	if err := json.Unmarshal([]byte(clean), &out); err != nil {
		return nil, promptDebug, fmt.Errorf("unmarshal openrouter topic json: %w", err)
	}

	return normalizeTopicSummaries(out.Topics), promptDebug, nil
}

func generateContentDraftsWithOpenRouter(ctx context.Context, apiKey, model string, recommendations []ContentRecommendation, limit int, createOSContext string, writingGuidelines string, timeoutSecs int, internalLinkInventory []InternalLinkCandidate) ([]BlogDraft, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" || limit <= 0 {
		return nil, nil
	}
	if timeoutSecs <= 0 {
		timeoutSecs = 240
	}
	if strings.TrimSpace(model) == "" {
		model = "moonshotai/kimi-k2"
	}
	input := draftPromptInput(recommendations, limit, internalLinkInventory)
	if len(input) == 0 {
		return nil, nil
	}
	drafts := make([]BlogDraft, 0, len(input))
	for _, item := range input {
		brief, err := generateBlogDraftBrief(ctx, apiKey, model, item, createOSContext, writingGuidelines, timeoutSecs)
		if err != nil {
			return nil, err
		}
		body, err := generateBlogDraftBody(ctx, apiKey, model, item, brief, createOSContext, writingGuidelines, timeoutSecs)
		if err != nil {
			return nil, err
		}
		brief.BodyMarkdown = body
		drafts = append(drafts, brief)
	}
	return normalizeBlogDrafts(drafts, limit), nil
}

func generateBlogDraftBrief(ctx context.Context, apiKey, model string, item blogDraftPromptItem, createOSContext string, writingGuidelines string, timeoutSecs int) (BlogDraft, error) {
	inputBytes, err := json.Marshal([]blogDraftPromptItem{item})
	if err != nil {
		return BlogDraft{}, fmt.Errorf("marshal blog draft brief input: %w", err)
	}
	content, err := executeOpenRouterChatWithRetry(ctx, apiKey, model, 0.3, "You are a product-led SEO strategist for CreateOS. Return only strict JSON with key drafts[].", blogDraftBriefUserPrompt(inputBytes, createOSContext, writingGuidelines), timeoutSecs)
	if err != nil {
		return BlogDraft{}, fmt.Errorf("openrouter blog draft brief skipped: %w", err)
	}
	clean := extractJSONObject(strings.TrimSpace(content))
	if clean == "" {
		return BlogDraft{}, fmt.Errorf("openrouter blog draft brief had no json object")
	}
	var out llmBlogDraftBriefOutput
	if err := json.Unmarshal([]byte(clean), &out); err != nil {
		return BlogDraft{}, fmt.Errorf("unmarshal openrouter blog draft brief json: %w", err)
	}
	briefs := normalizeBlogDraftsAllowEmptyBody(out.Drafts, 1)
	if len(briefs) == 0 {
		return BlogDraft{}, fmt.Errorf("openrouter blog draft brief returned zero usable drafts")
	}
	return briefs[0], nil
}

func generateBlogDraftBody(ctx context.Context, apiKey, model string, item blogDraftPromptItem, brief BlogDraft, createOSContext string, writingGuidelines string, timeoutSecs int) (string, error) {
	briefBytes, err := json.Marshal(brief)
	if err != nil {
		return "", fmt.Errorf("marshal blog draft body brief input: %w", err)
	}
	content, err := executeOpenRouterChatWithRetry(ctx, apiKey, model, 0.4, "You are a product-led SEO writer for CreateOS. Return markdown only.", blogDraftBodyUserPrompt(item, brief, briefBytes, createOSContext, writingGuidelines), timeoutSecs)
	if err != nil {
		return "", fmt.Errorf("openrouter blog draft body skipped: %w", err)
	}
	body := strings.TrimSpace(strings.Trim(content, "`"))
	if body == "" {
		return "", fmt.Errorf("openrouter blog draft body returned empty content")
	}
	return body, nil
}

func executeOpenRouterChatWithRetry(ctx context.Context, apiKey, model string, temperature float64, systemPrompt string, userPrompt string, timeoutSecs int) (string, error) {
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		content, err := executeOpenRouterChat(ctx, apiKey, model, temperature, systemPrompt, userPrompt, timeoutSecs)
		if err == nil {
			return content, nil
		}
		lastErr = err
		if !isTransientOpenRouterError(err.Error()) || attempt == 3 {
			break
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(time.Duration(attempt*3) * time.Second):
		}
	}
	return "", lastErr
}

func executeOpenRouterChat(ctx context.Context, apiKey, model string, temperature float64, systemPrompt string, userPrompt string, timeoutSecs int) (string, error) {
	requestBody := openRouterRequest{
		Model:       model,
		Temperature: temperature,
		Messages: []openRouterMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	}
	payload, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("marshal openrouter chat request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openRouterEndpoint, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("build openrouter chat request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: time.Duration(timeoutSecs) * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("execute openrouter chat request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("openrouter chat status=%d body=%q", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed openRouterResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", fmt.Errorf("decode openrouter chat response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("openrouter chat response returned no choices")
	}
	content := strings.TrimSpace(parsed.Choices[0].Message.Content)
	if content == "" {
		return "", fmt.Errorf("openrouter chat response returned empty content")
	}
	return content, nil
}

func isTransientOpenRouterError(message string) bool {
	lower := strings.ToLower(message)
	return strings.Contains(lower, "stream error") ||
		strings.Contains(lower, "internal_error") ||
		strings.Contains(lower, "unexpected eof") ||
		strings.Contains(lower, "connection reset") ||
		strings.Contains(lower, "context deadline exceeded") ||
		strings.Contains(lower, "client.timeout")
}

func blogDraftBriefUserPrompt(inputBytes []byte, createOSContext string, writingGuidelines string) string {
	contextInstruction := ""
	if strings.TrimSpace(createOSContext) != "" {
		contextInstruction = " Use the CreateOS context as positioning guidance, not as text to copy verbatim. Do not invent claims beyond the CreateOS context and recommendation input. CreateOS context: " + strings.TrimSpace(createOSContext)
	}
	guidelinesInstruction := ""
	if strings.TrimSpace(writingGuidelines) != "" {
		guidelinesInstruction = " Use the CreateOS writing guidelines as style and quality rules. Apply them without copying them verbatim. Writing guidelines: " + strings.TrimSpace(writingGuidelines)
	}
	return "Create a small SEO brief for each recommendation. Do not generate bodyMarkdown. Each draft object must include route, title, titleOptions, selectedTitleReason, metaDescription, internalLinks, cta, status. Generate 3-5 titleOptions before selecting title. Titles must hook readers while preserving search intent. Use one of these title patterns when appropriate: problem/tension, contrarian, value, story, or authority. Avoid bland titles like \"[Topic] with CreateOS\" unless that is genuinely strongest. Add a CreateOS-only SEO internal-link plan: internalLinks must include 3-5 links to createos.sh pages with anchorText, targetPath, placement, reason, and status. Use status=existing only for URLs provided in internalLinkCandidates. Prefer specific /blogs/* and /case-studies/* candidates over generic hub pages when relevant. Use status=planned only for recommended or cluster pages that may not exist yet. Do not create external citation plans or third-party backlink outreach ideas. Do not use em dashes. Avoid hype, clickbait, unsupported numeric claims, generic AI wording, and unsupported product claims." + contextInstruction + guidelinesInstruction + " status must be ai-generated-draft. JSON only. Data: " + string(inputBytes)
}

func blogDraftBodyUserPrompt(item blogDraftPromptItem, brief BlogDraft, briefBytes []byte, createOSContext string, writingGuidelines string) string {
	contextInstruction := ""
	if strings.TrimSpace(createOSContext) != "" {
		contextInstruction = " Use the CreateOS context as positioning guidance, not as text to copy verbatim. Do not invent claims beyond the CreateOS context and brief. CreateOS context: " + strings.TrimSpace(createOSContext)
	}
	guidelinesInstruction := ""
	if strings.TrimSpace(writingGuidelines) != "" {
		guidelinesInstruction = " Use the CreateOS writing guidelines as style and quality rules. Apply them without copying them verbatim. Writing guidelines: " + strings.TrimSpace(writingGuidelines)
	}
	itemBytes, _ := json.Marshal(item)
	return "Write the article body as markdown only. Do not wrap it in JSON. Do not use code fences. Body must read as polished blog prose, not an outline. Use paragraphs with clear transitions. Use bullets sparingly, max one bullet list. Each H2 section should have 2-4 paragraphs. Include an H1 matching the selected title, intro, 4-6 H2 sections, an honest tradeoffs section, and a closing CTA. Make it content-repo-ready publication markdown. Naturally include the selected existing CreateOS internal links from the brief as markdown links where relevant. Do not create external citation plans or third-party backlink outreach ideas. Do not use em dashes. Avoid hype/corporate language, generic AI tells, placeholder points, and unsupported absolute claims. Brief: " + string(briefBytes) + ". Recommendation input: " + string(itemBytes) + "." + contextInstruction + guidelinesInstruction
}

type blogDraftPromptItem struct {
	Route                  string                  `json:"route"`
	Title                  string                  `json:"title"`
	PageType               string                  `json:"pageType"`
	TargetIntent           string                  `json:"targetIntent"`
	Pillar                 string                  `json:"pillar"`
	ContentAngle           string                  `json:"contentAngle"`
	SourceEvidence         []string                `json:"sourceEvidence,omitempty"`
	InternalLinkCandidates []InternalLinkCandidate `json:"internalLinkCandidates,omitempty"`
}

func draftPromptInput(recommendations []ContentRecommendation, limit int, internalLinkInventory []InternalLinkCandidate) []blogDraftPromptItem {
	items := make([]blogDraftPromptItem, 0, min(len(recommendations), limit))
	for _, recommendation := range recommendations {
		if len(items) == limit {
			break
		}
		route := strings.TrimSpace(recommendation.SuggestedSlug)
		title := strings.TrimSpace(recommendation.SuggestedTitle)
		if route == "" || title == "" {
			continue
		}
		items = append(items, blogDraftPromptItem{
			Route:          route,
			Title:          title,
			PageType:       recommendation.PageType,
			TargetIntent:   recommendation.TargetIntent,
			Pillar:         recommendation.Pillar,
			ContentAngle:   recommendation.ContentAngle,
			SourceEvidence: limitStrings(recommendation.SourceEvidence, 3),
			InternalLinkCandidates: selectInternalLinkCandidatesForRecommendation(
				recommendation,
				internalLinkInventory,
				internalLinkCandidatePromptLimit,
			),
		})
	}
	return items
}

func normalizeBlogDraftsAllowEmptyBody(drafts []BlogDraft, limit int) []BlogDraft {
	out := make([]BlogDraft, 0, min(len(drafts), limit))
	for _, draft := range drafts {
		if len(out) == limit {
			break
		}
		route := strings.TrimSpace(draft.Route)
		title := strings.TrimSpace(draft.Title)
		if route == "" || title == "" {
			continue
		}
		status := strings.TrimSpace(draft.Status)
		if status == "" {
			status = "ai-generated-draft"
		}
		out = append(out, BlogDraft{
			Route:               route,
			Title:               title,
			TitleOptions:        normalizeTitleOptions(draft.TitleOptions, title),
			SelectedTitleReason: strings.TrimSpace(draft.SelectedTitleReason),
			MetaDescription:     strings.TrimSpace(draft.MetaDescription),
			InternalLinks:       normalizeSEOLinkSuggestions(draft.InternalLinks, 5),
			CTA:                 strings.TrimSpace(draft.CTA),
			Status:              status,
		})
	}
	return out
}

func normalizeBlogDrafts(drafts []BlogDraft, limit int) []BlogDraft {
	out := make([]BlogDraft, 0, min(len(drafts), limit))
	for _, draft := range drafts {
		if len(out) == limit {
			break
		}
		route := strings.TrimSpace(draft.Route)
		title := strings.TrimSpace(draft.Title)
		body := strings.TrimSpace(draft.BodyMarkdown)
		if route == "" || title == "" || body == "" {
			continue
		}
		status := strings.TrimSpace(draft.Status)
		if status == "" {
			status = "ai-generated-draft"
		}
		internalLinks := normalizeSEOLinkSuggestions(draft.InternalLinks, 5)
		out = append(out, BlogDraft{
			Route:               route,
			Title:               title,
			TitleOptions:        normalizeTitleOptions(draft.TitleOptions, title),
			SelectedTitleReason: strings.TrimSpace(draft.SelectedTitleReason),
			MetaDescription:     strings.TrimSpace(draft.MetaDescription),
			BodyMarkdown:        embedExistingInternalLinks(body, internalLinks),
			InternalLinks:       internalLinks,
			CTA:                 strings.TrimSpace(draft.CTA),
			Status:              status,
		})
	}
	return out
}

func embedExistingInternalLinks(body string, links []SEOLinkSuggestion) string {
	body = strings.TrimSpace(body)
	if body == "" || len(links) == 0 {
		return body
	}
	updated := body
	missing := make([]string, 0, len(links))
	for _, link := range links {
		if strings.ToLower(strings.TrimSpace(link.Status)) != "existing" {
			continue
		}
		anchor := strings.TrimSpace(link.AnchorText)
		url := createOSLinkURL(link.TargetPath)
		if anchor == "" || url == "" || strings.Contains(updated, url) {
			continue
		}
		markdownLink := "[" + anchor + "](" + url + ")"
		if strings.Contains(updated, markdownLink) {
			continue
		}
		if markdownLinkedAnchorExists(updated, anchor) {
			continue
		}
		if idx := strings.Index(updated, anchor); idx >= 0 {
			updated = updated[:idx] + markdownLink + updated[idx+len(anchor):]
			continue
		}
		missing = append(missing, markdownLink)
	}
	if len(missing) > 0 {
		updated += "\n\nRelated CreateOS pages: " + strings.Join(missing, ", ") + "."
	}
	return updated
}

func markdownLinkedAnchorExists(body string, anchor string) bool {
	anchor = strings.TrimSpace(anchor)
	if anchor == "" {
		return false
	}
	needle := "[" + anchor + "]("
	return strings.Contains(body, needle)
}

func createOSLinkURL(targetPath string) string {
	target := strings.TrimSpace(targetPath)
	if target == "" {
		return ""
	}
	if strings.HasPrefix(target, "https://createos.sh") {
		return target
	}
	if !strings.HasPrefix(target, "/") {
		target = "/" + target
	}
	return "https://createos.sh" + target
}

func normalizeSEOLinkSuggestions(suggestions []SEOLinkSuggestion, limit int) []SEOLinkSuggestion {
	if limit <= 0 {
		return nil
	}
	out := make([]SEOLinkSuggestion, 0, min(len(suggestions), limit))
	for _, suggestion := range suggestions {
		if len(out) == limit {
			break
		}
		anchor := strings.TrimSpace(suggestion.AnchorText)
		target := strings.TrimSpace(suggestion.TargetPath)
		if anchor == "" || target == "" {
			continue
		}
		status := strings.ToLower(strings.TrimSpace(suggestion.Status))
		if status != "existing" && status != "planned" {
			status = ""
		}
		out = append(out, SEOLinkSuggestion{
			AnchorText: anchor,
			TargetPath: target,
			Placement:  strings.TrimSpace(suggestion.Placement),
			Reason:     strings.TrimSpace(suggestion.Reason),
			Status:     status,
		})
	}
	return out
}

func normalizeTitleOptions(options []string, selectedTitle string) []string {
	out := make([]string, 0, min(len(options)+1, 5))
	seen := map[string]struct{}{}
	add := func(option string) {
		option = strings.TrimSpace(option)
		key := strings.ToLower(option)
		if option == "" {
			return
		}
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, option)
	}
	for _, option := range options {
		if len(out) == 5 {
			break
		}
		add(option)
	}
	if len(out) == 0 {
		add(selectedTitle)
	}
	return out
}

func attachDraftsToContentRecommendations(recommendations []ContentRecommendation, drafts []BlogDraft, limit int) []ContentRecommendation {
	out := make([]ContentRecommendation, len(recommendations))
	copy(out, recommendations)
	if limit <= 0 {
		return out
	}
	draftsByRoute := map[string]BlogDraft{}
	for _, draft := range normalizeBlogDrafts(drafts, limit) {
		draftsByRoute[draft.Route] = draft
	}
	for idx := range out {
		if idx >= limit {
			break
		}
		draft, ok := draftsByRoute[out[idx].SuggestedSlug]
		if !ok {
			continue
		}
		out[idx].Draft = &draft
	}
	return out
}

func buildTopicPromptInput(competitors []SiteSnapshot, limit int) []topicPromptCompetitor {
	input, _ := buildTopicPromptInputWithDebug(competitors, limit)
	return input
}

func buildTopicPromptInputWithDebug(competitors []SiteSnapshot, limit int) ([]topicPromptCompetitor, []TopicPromptDebug) {
	if limit <= 0 {
		return nil, nil
	}
	result := make([]topicPromptCompetitor, 0, len(competitors))
	debug := make([]TopicPromptDebug, 0, len(competitors))
	for _, competitor := range competitors {
		if strings.TrimSpace(competitor.Name) == "" || competitor.Error != "" {
			continue
		}
		entryDebug := TopicPromptDebug{Competitor: strings.TrimSpace(competitor.Name)}
		pages := make([]topicPromptPage, 0, limit)
		candidates := buildPageCandidates(competitor)
		for _, candidate := range candidates {
			if len(pages) >= limit {
				break
			}
			if strings.TrimSpace(candidate.URL) == "" {
				entryDebug.SkippedLowValue++
				entryDebug.RejectedPages = appendDebugCandidate(entryDebug.RejectedPages, candidate)
				continue
			}
			if candidate.RelevanceScore < promptCandidateThreshold || len(candidate.NegativeSignals) > 0 && candidate.RelevanceScore < 70 {
				entryDebug.SkippedLowValue++
				entryDebug.RejectedPages = appendDebugCandidate(entryDebug.RejectedPages, candidate)
				continue
			}
			if strings.TrimSpace(candidate.Title) == "" {
				entryDebug.SkippedNoTitle++
				entryDebug.RejectedPages = appendDebugCandidate(entryDebug.RejectedPages, candidate)
				continue
			}
			pages = append(pages, topicPromptPage{
				Title:          candidate.Title,
				URL:            candidate.URL,
				PageType:       candidate.PageType,
				RelevanceScore: candidate.RelevanceScore,
				WhySelected:    candidate.WhySelected,
			})
			entryDebug.SelectedPages = appendDebugCandidate(entryDebug.SelectedPages, candidate)
			if len(entryDebug.SampleURLs) < 5 {
				entryDebug.SampleURLs = append(entryDebug.SampleURLs, candidate.URL)
			}
		}
		entryDebug.PagesSent = len(pages)
		debug = append(debug, entryDebug)
		if len(pages) == 0 {
			continue
		}
		result = append(result, topicPromptCompetitor{
			Competitor: strings.TrimSpace(competitor.Name),
			Pages:      pages,
		})
	}
	return result, debug
}

func appendDebugCandidate(candidates []PageCandidate, candidate PageCandidate) []PageCandidate {
	if len(candidates) >= 20 {
		return candidates
	}
	return append(candidates, candidate)
}

func normalizeTopicSummaries(topics []TopicSummary) []TopicSummary {
	normalized := make([]TopicSummary, 0, len(topics))
	for _, topic := range topics {
		competitor := strings.TrimSpace(topic.Competitor)
		name := strings.TrimSpace(topic.Name)
		if competitor == "" || name == "" {
			continue
		}

		repTitles := make([]string, 0, 5)
		for _, title := range topic.RepresentativeTitles {
			title = strings.TrimSpace(title)
			if title == "" {
				continue
			}
			repTitles = append(repTitles, title)
			if len(repTitles) == 5 {
				break
			}
		}

		evidenceURLs := make([]string, 0, 5)
		for _, evidenceURL := range topic.EvidenceURLs {
			evidenceURL = strings.TrimSpace(evidenceURL)
			if evidenceURL == "" {
				continue
			}
			evidenceURLs = append(evidenceURLs, evidenceURL)
			if len(evidenceURLs) == 5 {
				break
			}
		}

		normalized = append(normalized, TopicSummary{
			Competitor:           competitor,
			Name:                 name,
			PageCount:            supportedPageCount(topic.PageCount, len(evidenceURLs)),
			RepresentativeTitles: repTitles,
			EvidenceURLs:         evidenceURLs,
			WhyItMatters:         strings.TrimSpace(topic.WhyItMatters),
		})
	}
	return normalized
}

func supportedPageCount(pageCount int, evidenceCount int) int {
	if pageCount < 0 {
		return 0
	}
	if evidenceCount > 0 && pageCount > evidenceCount {
		return evidenceCount
	}
	return pageCount
}

func trimTopicPromptInputToBytes(input []topicPromptCompetitor, capBytes int) ([]topicPromptCompetitor, []byte, error) {
	if capBytes <= 0 {
		return nil, nil, nil
	}
	trimmed := copyTopicPromptInput(input)
	inputBytes, err := json.Marshal(trimmed)
	if err != nil {
		return nil, nil, err
	}
	if len(inputBytes) <= capBytes {
		return trimmed, inputBytes, nil
	}

	for len(trimmed) > 0 {
		pageRemoved := false
		for idx := len(trimmed) - 1; idx >= 0; idx-- {
			if len(trimmed[idx].Pages) == 0 {
				continue
			}
			trimmed[idx].Pages = trimmed[idx].Pages[:len(trimmed[idx].Pages)-1]
			pageRemoved = true
			if len(trimmed[idx].Pages) == 0 {
				trimmed = append(trimmed[:idx], trimmed[idx+1:]...)
			}
			break
		}
		if !pageRemoved {
			return nil, nil, nil
		}

		inputBytes, err = json.Marshal(trimmed)
		if err != nil {
			return nil, nil, err
		}
		if len(inputBytes) <= capBytes {
			return trimmed, inputBytes, nil
		}
	}

	return nil, nil, nil
}

func copyTopicPromptInput(input []topicPromptCompetitor) []topicPromptCompetitor {
	out := make([]topicPromptCompetitor, 0, len(input))
	for _, competitor := range input {
		pages := make([]topicPromptPage, len(competitor.Pages))
		copy(pages, competitor.Pages)
		out = append(out, topicPromptCompetitor{
			Competitor: competitor.Competitor,
			Pages:      pages,
		})
	}
	return out
}
