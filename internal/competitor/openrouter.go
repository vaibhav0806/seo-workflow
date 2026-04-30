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

type topicPromptPage struct {
	Title string `json:"title"`
	URL   string `json:"url"`
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

func extractTopicsWithOpenRouter(ctx context.Context, apiKey, model string, competitors []SiteSnapshot) ([]TopicSummary, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, nil
	}
	if strings.TrimSpace(model) == "" {
		model = "moonshotai/kimi-k2"
	}

	input := buildTopicPromptInput(competitors, 40)
	if len(input) == 0 {
		return nil, nil
	}
	input, inputBytes, err := trimTopicPromptInputToBytes(input, topicPromptPayloadByteCap)
	if err != nil {
		return nil, fmt.Errorf("trim topic prompt input: %w", err)
	}
	if len(input) == 0 {
		return nil, nil
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
		return nil, fmt.Errorf("marshal openrouter topic request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openRouterEndpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build openrouter topic request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute openrouter topic request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("openrouter topic status=%d body=%q", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed openRouterResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("decode openrouter topic response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return nil, fmt.Errorf("openrouter topic response returned no choices")
	}

	content := strings.TrimSpace(parsed.Choices[0].Message.Content)
	if content == "" {
		return nil, fmt.Errorf("openrouter topic response returned empty content")
	}

	clean := extractJSONObject(content)
	if clean == "" {
		return nil, fmt.Errorf("openrouter topic response had no json object")
	}

	var out llmTopicOutput
	if err := json.Unmarshal([]byte(clean), &out); err != nil {
		return nil, fmt.Errorf("unmarshal openrouter topic json: %w", err)
	}

	return normalizeTopicSummaries(out.Topics), nil
}

func buildTopicPromptInput(competitors []SiteSnapshot, limit int) []topicPromptCompetitor {
	if limit <= 0 {
		return nil
	}
	result := make([]topicPromptCompetitor, 0, len(competitors))
	for _, competitor := range competitors {
		if strings.TrimSpace(competitor.Name) == "" || competitor.Error != "" {
			continue
		}
		pages := make([]topicPromptPage, 0, limit)
		for _, entry := range competitor.RecentURLs {
			if len(pages) >= limit {
				break
			}
			title := strings.TrimSpace(entry.Title)
			url := strings.TrimSpace(entry.URL)
			if title == "" || url == "" {
				continue
			}
			pages = append(pages, topicPromptPage{
				Title: title,
				URL:   url,
			})
		}
		if len(pages) == 0 {
			continue
		}
		result = append(result, topicPromptCompetitor{
			Competitor: strings.TrimSpace(competitor.Name),
			Pages:      pages,
		})
	}
	return result
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
			PageCount:            max(0, topic.PageCount),
			RepresentativeTitles: repTitles,
			EvidenceURLs:         evidenceURLs,
			WhyItMatters:         strings.TrimSpace(topic.WhyItMatters),
		})
	}
	return normalized
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
