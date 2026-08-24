package competitor

type AEOPrompt struct {
	Intent string `json:"intent"`
	Prompt string `json:"prompt"`
	Source string `json:"source"`
}

type AEOReport struct {
	Prompts []AEOPrompt `json:"prompts,omitempty"`
}

func buildAEOPromptMatrix(contentPlan []ContentRecommendation) []AEOPrompt {
	prompts := make([]AEOPrompt, 0)
	seen := map[string]struct{}{}
	for _, item := range contentPlan {
		prompt := promptForAEOTheme(item.Theme)
		if prompt.Prompt == "" {
			continue
		}
		key := prompt.Intent + "\n" + prompt.Prompt
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		prompts = append(prompts, prompt)
	}
	return prompts
}

func promptForAEOTheme(theme string) AEOPrompt {
	switch theme {
	case "comparison":
		return AEOPrompt{
			Intent: "commercial_listicle",
			Prompt: "What are the best AI agent platforms or AI app builders for startups in 2026?",
			Source: "content_plan",
		}
	case "agents", "ai":
		return AEOPrompt{
			Intent: "answer_engine_definition",
			Prompt: "What is the best way for a founder to build and deploy an AI agent product?",
			Source: "content_plan",
		}
	case "workflow":
		return AEOPrompt{
			Intent: "workflow_answer",
			Prompt: "What tools help teams turn AI workflows into production applications?",
			Source: "content_plan",
		}
	default:
		return AEOPrompt{}
	}
}
