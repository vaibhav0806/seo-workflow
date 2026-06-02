package competitor

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBuildStateDiffMarksFirstSeenURLsAsRecent(t *testing.T) {
	now := time.Date(2026, 6, 2, 8, 0, 0, 0, time.UTC)
	previous := CompetitorState{}
	entries := []rawSitemapEntry{
		{URL: "https://www.stackai.com/solutions/finance"},
		{URL: "https://www.stackai.com/insights/best-ai-agent-building-platforms-in-2026"},
	}

	diff := buildStateDiff(previous, "stackai", entries, now)

	require.Len(t, diff.NewURLs, 2)
	require.Equal(t, "first_seen", diff.NewURLs[0].FreshnessSource)
	require.Equal(t, now.Format(time.RFC3339), diff.NewURLs[0].FirstSeenAt)
}

func TestBuildStateDiffDetectsRemovedURLs(t *testing.T) {
	now := time.Date(2026, 6, 2, 8, 0, 0, 0, time.UTC)
	previous := CompetitorState{
		Sites: map[string]SiteURLState{
			"stackai": {
				URLs: map[string]URLState{
					"https://www.stackai.com/old": {
						URL:         "https://www.stackai.com/old",
						FirstSeenAt: "2026-05-01T00:00:00Z",
					},
				},
			},
		},
	}

	diff := buildStateDiff(previous, "stackai", nil, now)

	require.Len(t, diff.RemovedURLs, 1)
	require.Equal(t, "https://www.stackai.com/old", diff.RemovedURLs[0].URL)
	require.Equal(t, now.Format(time.RFC3339), diff.RemovedURLs[0].RemovedAt)
}

func TestCompetitorStateRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "competitor-state.json")
	state := CompetitorState{
		Sites: map[string]SiteURLState{
			"stackai": {
				URLs: map[string]URLState{
					"https://www.stackai.com/": {
						URL:             "https://www.stackai.com/",
						FirstSeenAt:     "2026-06-02T08:00:00Z",
						LastSeenAt:      "2026-06-02T08:00:00Z",
						FreshnessSource: "first_seen",
					},
				},
			},
		},
	}

	require.NoError(t, saveCompetitorState(path, state))
	loaded, err := loadCompetitorState(path)

	require.NoError(t, err)
	require.Equal(t, state, loaded)
}
