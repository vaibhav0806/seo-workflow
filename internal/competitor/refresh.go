package competitor

type RefreshRecommendation struct {
	Path          string   `json:"path"`
	Title         string   `json:"title"`
	Cadence       string   `json:"cadence"`
	PriorityScore int      `json:"priorityScore"`
	Reason        string   `json:"reason"`
	Evidence      []string `json:"evidence,omitempty"`
}

func buildRefreshRecommendations(contentPlan []ContentRecommendation) []RefreshRecommendation {
	recommendations := make([]RefreshRecommendation, 0)
	for _, item := range contentPlan {
		score := refreshPriorityForTheme(item.Theme)
		if score < 60 {
			continue
		}
		recommendations = append(recommendations, RefreshRecommendation{
			Path:          item.SuggestedSlug,
			Title:         item.SuggestedTitle,
			Cadence:       refreshCadenceForTheme(item.Theme),
			PriorityScore: score,
			Reason:        refreshReasonForTheme(item.Theme),
			Evidence:      limitStrings(item.SourceEvidence, 3),
		})
	}
	return recommendations
}

func refreshPriorityForTheme(theme string) int {
	switch theme {
	case "comparison":
		return 90
	case "agents", "ai", "workflow":
		return 82
	case "integrations", "pricing":
		return 75
	default:
		return 50
	}
}

func refreshCadenceForTheme(theme string) string {
	switch theme {
	case "comparison", "agents", "ai", "workflow":
		return "monthly"
	case "integrations", "pricing":
		return "quarterly"
	default:
		return "semiannual"
	}
}

func refreshReasonForTheme(theme string) string {
	switch theme {
	case "comparison":
		return "Comparison and listicle pages decay quickly because AI tools change frequently."
	case "agents", "ai", "workflow":
		return "AI agent and workflow automation topics are volatile and need freshness signals."
	case "integrations", "pricing":
		return "Integration and pricing blogs change often enough to require scheduled review."
	default:
		return "Refresh when rankings or competitor coverage changes."
	}
}
