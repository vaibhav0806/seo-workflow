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
