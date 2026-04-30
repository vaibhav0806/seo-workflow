package competitor

import (
	"encoding/json"
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

func TestBuildTopicPromptInputUsesTitlesAndCapsLimit(t *testing.T) {
	competitors := []SiteSnapshot{
		{
			Name: "vercel",
			RecentURLs: []SitemapEntry{
				{URL: "https://vercel.com/1", Title: "Title 1"},
				{URL: "https://vercel.com/2", Title: "Title 2"},
				{URL: "https://vercel.com/3", Title: ""},
			},
		},
	}

	out := buildTopicPromptInput(competitors, 1)

	require.Len(t, out, 1)
	require.Equal(t, "vercel", out[0].Competitor)
	require.Len(t, out[0].Pages, 1)
	require.Equal(t, "Title 1", out[0].Pages[0].Title)
	require.Equal(t, "https://vercel.com/1", out[0].Pages[0].URL)
}

func TestNormalizeTopicSummariesTrimsDropsInvalidAndCapsEvidence(t *testing.T) {
	topics := []TopicSummary{
		{
			Competitor: "  vercel ",
			Name:       "  agent workflows ",
			PageCount:  -2,
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
	require.Equal(t, 0, out[0].PageCount)
	require.Equal(t, []string{"A", "B", "C", "D", "E"}, out[0].RepresentativeTitles)
	require.Equal(t, []string{"https://a", "https://b", "https://c", "https://d", "https://e"}, out[0].EvidenceURLs)
	require.Equal(t, "repeated demand", out[0].WhyItMatters)
}

