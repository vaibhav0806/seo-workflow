package competitor

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
	"time"
)

type CompetitorState struct {
	Sites map[string]SiteURLState `json:"sites"`
}

type SiteURLState struct {
	URLs map[string]URLState `json:"urls"`
}

type URLState struct {
	URL             string `json:"url"`
	FirstSeenAt     string `json:"firstSeenAt"`
	LastSeenAt      string `json:"lastSeenAt"`
	RemovedAt       string `json:"removedAt,omitempty"`
	FreshnessSource string `json:"freshnessSource"`
}

type StateDiff struct {
	NewURLs     []URLState      `json:"newUrls"`
	RemovedURLs []URLState      `json:"removedUrls"`
	NextState   CompetitorState `json:"nextState"`
}

func loadCompetitorState(path string) (CompetitorState, error) {
	if strings.TrimSpace(path) == "" {
		return emptyCompetitorState(), nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return emptyCompetitorState(), nil
		}
		return CompetitorState{}, err
	}
	var state CompetitorState
	if err := json.Unmarshal(raw, &state); err != nil {
		return CompetitorState{}, err
	}
	if state.Sites == nil {
		state.Sites = map[string]SiteURLState{}
	}
	return state, nil
}

func saveCompetitorState(path string, state CompetitorState) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}

func buildStateDiff(previous CompetitorState, site string, entries []rawSitemapEntry, now time.Time) StateDiff {
	if previous.Sites == nil {
		previous.Sites = map[string]SiteURLState{}
	}
	prevSite := previous.Sites[site]
	if prevSite.URLs == nil {
		prevSite.URLs = map[string]URLState{}
	}

	nextURLs := map[string]URLState{}
	newURLs := make([]URLState, 0)
	seen := map[string]struct{}{}
	nowValue := now.UTC().Format(time.RFC3339)

	for _, entry := range entries {
		rawURL := strings.TrimSpace(entry.URL)
		if rawURL == "" {
			continue
		}
		seen[rawURL] = struct{}{}
		state, exists := prevSite.URLs[rawURL]
		if !exists || strings.TrimSpace(state.FirstSeenAt) == "" {
			state = URLState{
				URL:             rawURL,
				FirstSeenAt:     nowValue,
				FreshnessSource: "first_seen",
			}
			newURLs = append(newURLs, state)
		}
		state.URL = rawURL
		state.LastSeenAt = nowValue
		state.RemovedAt = ""
		if strings.TrimSpace(state.FreshnessSource) == "" {
			state.FreshnessSource = "first_seen"
		}
		nextURLs[rawURL] = state
	}

	removedURLs := make([]URLState, 0)
	for rawURL, state := range prevSite.URLs {
		if _, ok := seen[rawURL]; ok {
			continue
		}
		state.URL = rawURL
		state.RemovedAt = nowValue
		removedURLs = append(removedURLs, state)
	}

	sort.Slice(newURLs, func(i, j int) bool { return newURLs[i].URL < newURLs[j].URL })
	sort.Slice(removedURLs, func(i, j int) bool { return removedURLs[i].URL < removedURLs[j].URL })

	next := copyCompetitorState(previous)
	next.Sites[site] = SiteURLState{URLs: nextURLs}
	return StateDiff{NewURLs: newURLs, RemovedURLs: removedURLs, NextState: next}
}

func emptyCompetitorState() CompetitorState {
	return CompetitorState{Sites: map[string]SiteURLState{}}
}

func copyCompetitorState(state CompetitorState) CompetitorState {
	out := emptyCompetitorState()
	for site, siteState := range state.Sites {
		urls := map[string]URLState{}
		for rawURL, urlState := range siteState.URLs {
			urls[rawURL] = urlState
		}
		out.Sites[site] = SiteURLState{URLs: urls}
	}
	return out
}
