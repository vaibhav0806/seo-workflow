package contentrepo

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAssessSEORiskFlagsHighRiskListicleWithoutMethodology(t *testing.T) {
	post := BlogPost{
		Title:       "Top AI Agent Platforms for Enterprise Teams in 2026",
		Slug:        "ai-agent-platforms-enterprise-teams-2026",
		Description: "Compare AI agent platforms for enterprise teams.",
		Cover:       "https://example.com/cover.jpg",
		PublishedAt: time.Date(2026, 6, 15, 11, 0, 0, 0, time.UTC),
		Destination: "createos",
		BodyMarkdown: strings.Repeat("AI agent platforms help enterprise teams evaluate AI agent platforms. ", 18) +
			"\n\n## Deployment\n\nAI agent platforms need deployment controls.",
	}

	report := AssessSEORisk(post)

	require.Equal(t, SEORiskHigh, report.Severity)
	require.True(t, report.Risky)
	require.Contains(t, report.Summary, "HIGH")
	require.Contains(t, report.FindingCodes(), "keyword-repetition")
	require.Contains(t, report.FindingCodes(), "missing-listicle-methodology")
	require.Contains(t, report.FindingCodes(), "thin-content")
}

func TestSEORiskReportMarkdownIncludesProofAndHumanReviewLanguage(t *testing.T) {
	report := SEORiskReport{
		Severity: SEORiskHigh,
		Risky:    true,
		Summary:  "HIGH risk: 2 findings require human review.",
		Findings: []SEORiskFinding{
			{
				Code:          "keyword-repetition",
				Risk:          "Keyword stuffing",
				Severity:      SEORiskMedium,
				Evidence:      "`AI agent platforms` appears 18 times in 220 words.",
				SuggestedFix:  "Reduce exact-match repetition.",
				GooglePolicy:  "Keyword stuffing",
				AffectedField: "body",
			},
		},
	}

	markdown := report.Markdown()

	require.Contains(t, markdown, "## SEO Risk Review: HIGH")
	require.Contains(t, markdown, "This is an automated risk review, not a merge decision.")
	require.Contains(t, markdown, "| Keyword stuffing | Medium | body |")
	require.Contains(t, markdown, "`AI agent platforms` appears 18 times")
}

func TestMitigateSEORiskAddsReviewMethodologyWithoutChangingTitle(t *testing.T) {
	post := BlogPost{
		Title:        "Top AI Agent Platforms for Enterprise Teams in 2026",
		Slug:         "ai-agent-platforms-enterprise-teams-2026",
		Description:  "Compare AI agent platforms for enterprise teams.",
		Author:       "CreateOS",
		ReadTime:     "5 min",
		Tags:         []string{"createos", "comparison"},
		Cover:        "https://example.com/cover.jpg",
		PublishedAt:  time.Date(2026, 6, 15, 11, 0, 0, 0, time.UTC),
		Destination:  "createos",
		BodyMarkdown: "# Top AI Agent Platforms for Enterprise Teams in 2026\n\nAI agent platforms help enterprise teams evaluate AI agent platforms.",
	}
	report := SEORiskReport{Severity: SEORiskHigh, Risky: true}

	mitigated := MitigateSEORisk(post, report)

	require.Equal(t, post.Title, mitigated.Title)
	require.Equal(t, "ai-agent-platforms-enterprise-teams-2026-mitigated", mitigated.Slug)
	require.Contains(t, mitigated.BodyMarkdown, "## How We Evaluated These Platforms")
	require.Contains(t, mitigated.BodyMarkdown, "## Editorial Risk Mitigation Notes")
	require.Contains(t, mitigated.BodyMarkdown, "human review")
}
