package competitor

import (
	"net/url"
	"sort"
	"strings"

	"github.com/nodeops/seo-workflow/internal/gsc"
)

func loadConfiguredPerformance(path string) (gsc.PerformanceReport, []string) {
	if strings.TrimSpace(path) == "" {
		return gsc.PerformanceReport{}, nil
	}
	state, err := gsc.LoadPerformanceState(path)
	if err != nil {
		return gsc.PerformanceReport{}, []string{"search performance skipped: " + err.Error()}
	}
	current, previous := gsc.LatestPerformanceSnapshots(state)
	return gsc.AnalyzePerformance(current, previous), nil
}

func applyPerformanceSignals(plan []ContentRecommendation, report gsc.PerformanceReport) []ContentRecommendation {
	out := append([]ContentRecommendation(nil), plan...)
	for idx := range out {
		for _, signal := range report.Opportunities {
			if !keywordSetsOverlap(
				[]string{out[idx].PrimaryKeyword, out[idx].SuggestedTitle},
				[]string{signal.Query},
			) {
				continue
			}
			out[idx].SearchSignals = append(out[idx].SearchSignals, signal)
			if out[idx].Decision == ContentDecisionCreate {
				if route := blogRoute(signal.Page); route != "" {
					out[idx].Decision = ContentDecisionRefresh
					out[idx].ExistingRoute = route
					out[idx].DecisionReason = "Search Console already shows demand for an existing CreateOS blog targeting this query."
					out[idx].Draft = nil
				}
			}
		}
		sort.Slice(out[idx].SearchSignals, func(i, j int) bool {
			if out[idx].SearchSignals[i].Score == out[idx].SearchSignals[j].Score {
				return out[idx].SearchSignals[i].Query < out[idx].SearchSignals[j].Query
			}
			return out[idx].SearchSignals[i].Score > out[idx].SearchSignals[j].Score
		})
	}
	return out
}

func blogRoute(raw string) string {
	value := strings.TrimSpace(raw)
	if parsed, err := url.Parse(value); err == nil && parsed.Path != "" {
		value = parsed.Path
	}
	value = "/" + strings.TrimLeft(value, "/")
	if !strings.HasPrefix(value, "/blogs/") {
		return ""
	}
	return strings.TrimSuffix(value, "/")
}
