package competitor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nodeops/seo-workflow/internal/config"
)

type CompetitorTarget struct {
	Name       string `json:"name"`
	SitemapURL string `json:"sitemapUrl"`
}

type SitemapEntry struct {
	URL       string   `json:"url"`
	Title     string   `json:"title,omitempty"`
	LastMod   *string  `json:"lastMod,omitempty"`
	ThemeTags []string `json:"themeTags,omitempty"`
}

type SiteSnapshot struct {
	Name           string         `json:"name"`
	SitemapURL     string         `json:"sitemapUrl"`
	TotalURLs      int            `json:"totalUrls"`
	RecentURLs     []SitemapEntry `json:"recentUrls"`
	RecentURLCount int            `json:"recentUrlCount"`
	ThemeCounts    map[string]int `json:"themeCounts"`
	Error          string         `json:"error,omitempty"`
}

type Opportunity struct {
	Title           string   `json:"title"`
	WhyItMatters    string   `json:"whyItMatters"`
	WhatToDo        string   `json:"whatToDo"`
	HowToExecute    []string `json:"howToExecute"`
	ImpactScore     int      `json:"impactScore"`
	Competitor      string   `json:"competitor"`
	Theme           string   `json:"theme"`
	OpportunityType string   `json:"opportunityType"`
	Evidence        []string `json:"evidence"`
}

type ContentRecommendation struct {
	Priority       int                         `json:"priority"`
	Opportunity    string                      `json:"opportunity"`
	Competitor     string                      `json:"competitor"`
	Theme          string                      `json:"theme"`
	PageType       string                      `json:"pageType"`
	SuggestedSlug  string                      `json:"suggestedSlug"`
	SuggestedTitle string                      `json:"suggestedTitle"`
	TargetIntent   string                      `json:"targetIntent"`
	ContentAngle   string                      `json:"contentAngle"`
	Pillar         string                      `json:"pillar"`
	ClusterPages   []ContentPageRecommendation `json:"clusterPages,omitempty"`
	SourceEvidence []string                    `json:"sourceEvidence,omitempty"`
	Draft          *BlogDraft                  `json:"draft,omitempty"`
}

type BlogDraft struct {
	Route               string              `json:"route"`
	Title               string              `json:"title"`
	TitleOptions        []string            `json:"titleOptions,omitempty"`
	SelectedTitleReason string              `json:"selectedTitleReason,omitempty"`
	MetaDescription     string              `json:"metaDescription"`
	BodyMarkdown        string              `json:"bodyMarkdown"`
	InternalLinks       []SEOLinkSuggestion `json:"internalLinks,omitempty"`
	CTA                 string              `json:"cta"`
	Status              string              `json:"status"`
}

type SEOLinkSuggestion struct {
	AnchorText string `json:"anchorText"`
	TargetPath string `json:"targetPath"`
	Placement  string `json:"placement"`
	Reason     string `json:"reason"`
	Status     string `json:"status,omitempty"`
}

type InternalLinkCandidate struct {
	URL      string `json:"url"`
	Path     string `json:"path"`
	Title    string `json:"title,omitempty"`
	PageType string `json:"pageType"`
	Category string `json:"category,omitempty"`
	Score    int    `json:"score"`
	Reason   string `json:"reason,omitempty"`
}

type ContentPageRecommendation struct {
	PageType     string `json:"pageType"`
	Slug         string `json:"slug"`
	Title        string `json:"title"`
	TargetIntent string `json:"targetIntent"`
}

type TopicSummary struct {
	Competitor           string   `json:"competitor"`
	Name                 string   `json:"name"`
	PageCount            int      `json:"pageCount"`
	RepresentativeTitles []string `json:"representativeTitles"`
	EvidenceURLs         []string `json:"evidenceUrls"`
	WhyItMatters         string   `json:"whyItMatters"`
}

type Summary struct {
	GeneratedAtUTC  string                  `json:"generatedAtUtc"`
	WindowDays      int                     `json:"windowDays"`
	WindowStartUTC  string                  `json:"windowStartUtc"`
	OurSite         SiteSnapshot            `json:"ourSite"`
	Competitors     []SiteSnapshot          `json:"competitors"`
	ExtractedTopics []TopicSummary          `json:"extractedTopics,omitempty"`
	Opportunities   []Opportunity           `json:"opportunities"`
	ContentPlan     []ContentRecommendation `json:"recommendedContentPlan,omitempty"`
	Warnings        []string                `json:"warnings"`
	OpenRouterModel string                  `json:"openRouterModel,omitempty"`
	Debug           DebugSummary            `json:"debug,omitempty"`
}

type DebugSummary struct {
	TitleEnrichment []TitleEnrichmentDebug `json:"titleEnrichment,omitempty"`
	TopicPrompt     []TopicPromptDebug     `json:"topicPrompt,omitempty"`
	TopicScoring    []TopicScoringDebug    `json:"topicScoring,omitempty"`
	SkippedTopics   []SkippedTopicDebug    `json:"skippedTopics,omitempty"`
}

type TitleEnrichmentDebug struct {
	Site             string `json:"site"`
	RecentURLCount   int    `json:"recentUrlCount"`
	Attempted        int    `json:"attempted"`
	Titled           int    `json:"titled"`
	EmptyTitle       int    `json:"emptyTitle"`
	SkippedByFilter  int    `json:"skippedByFilter"`
	EnrichmentLimit  int    `json:"enrichmentLimit"`
	EnrichmentReason string `json:"enrichmentReason,omitempty"`
}

type TopicPromptDebug struct {
	Competitor      string          `json:"competitor"`
	PagesSent       int             `json:"pagesSent"`
	SkippedNoTitle  int             `json:"skippedNoTitle"`
	SkippedLowValue int             `json:"skippedLowValue"`
	SampleURLs      []string        `json:"sampleUrls,omitempty"`
	SelectedPages   []PageCandidate `json:"selectedPages,omitempty"`
	RejectedPages   []PageCandidate `json:"rejectedPages,omitempty"`
}

type TopicScoringDebug struct {
	Competitor           string         `json:"competitor"`
	Topic                string         `json:"topic"`
	Theme                string         `json:"theme"`
	PageCount            int            `json:"pageCount"`
	EffectivePages       int            `json:"effectivePages"`
	EvidenceCount        int            `json:"evidenceCount"`
	EvidenceQualityScore int            `json:"evidenceQualityScore"`
	MatchedTokens        int            `json:"matchedTokens"`
	TotalTokens          int            `json:"totalTokens"`
	UncoveredTokens      int            `json:"uncoveredTokens"`
	ScoreBreakdown       ScoreBreakdown `json:"scoreBreakdown"`
	Score                int            `json:"score"`
}

type TopicAnalysisDebug struct {
	ScoredTopics  []TopicScoringDebug `json:"scoredTopics,omitempty"`
	SkippedTopics []SkippedTopicDebug `json:"skippedTopics,omitempty"`
}

type SkippedTopicDebug struct {
	Competitor      string   `json:"competitor"`
	Topic           string   `json:"topic"`
	Theme           string   `json:"theme"`
	Reason          string   `json:"reason"`
	PageCount       int      `json:"pageCount"`
	EvidenceCount   int      `json:"evidenceCount"`
	MatchedTokens   int      `json:"matchedTokens"`
	TotalTokens     int      `json:"totalTokens"`
	UncoveredTokens int      `json:"uncoveredTokens"`
	EvidenceURLs    []string `json:"evidenceUrls,omitempty"`
}

type ScoreBreakdown struct {
	GapCoverage     int `json:"gapCoverage"`
	EvidenceQuality int `json:"evidenceQuality"`
	ThemePriority   int `json:"themePriority"`
	Relevance       int `json:"relevance"`
}

var defaultCompetitors = []CompetitorTarget{
	{Name: "vercel", SitemapURL: "https://vercel.com/sitemap.xml"},
	{Name: "lovable", SitemapURL: "https://lovable.dev/sitemap.xml"},
	{Name: "replit", SitemapURL: "https://replit.com/sitemap.xml"},
	{Name: "emergent", SitemapURL: "https://emergent.sh/sitemap.xml"},
}

const (
	titleEnrichmentLimit       = 40
	titleEnrichmentConcurrency = 8
	titleEnrichmentTimeout     = 20 * time.Second
	createOSContextPath        = "docs/createos-context.md"
	createOSWritingGuidesPath  = "docs/createos-writing-guidelines.md"
)

func Run(ctx context.Context, cfg *config.Config) (Summary, error) {
	if cfg == nil {
		return Summary{}, fmt.Errorf("competitor config is nil")
	}

	windowStart := time.Now().UTC().AddDate(0, 0, -cfg.CompetitorWindowDays)
	fetcher := NewSitemapFetcher(cfg.HTTPTimeoutSecs)
	titleFetcher := NewTitleFetcher(cfg.HTTPTimeoutSecs)
	warnings := make([]string, 0)
	debug := DebugSummary{}

	ourEntries, err := fetcher.Fetch(ctx, cfg.OurSitemapURL)
	if err != nil {
		return Summary{}, fmt.Errorf("fetch our sitemap: %w", err)
	}
	ourSnapshot := buildSnapshot("createos", cfg.OurSitemapURL, ourEntries, windowStart)
	ourSnapshot, titleWarnings, titleDebug, err := enrichSnapshotTitles(ctx, titleFetcher, ourSnapshot, titleEnrichmentLimit)
	if err != nil {
		return Summary{}, fmt.Errorf("title enrichment failed for createos: %w", err)
	}
	debug.TitleEnrichment = append(debug.TitleEnrichment, titleDebug)
	warnings = append(warnings, titleWarnings...)

	competitorSnapshots := make([]SiteSnapshot, 0, len(defaultCompetitors))
	for _, target := range defaultCompetitors {
		entries, fetchErr := fetcher.Fetch(ctx, target.SitemapURL)
		if fetchErr != nil {
			if ctx.Err() != nil {
				return Summary{}, fmt.Errorf("fetch competitor sitemap %q: %w", target.Name, ctx.Err())
			}
			warnings = append(warnings, fmt.Sprintf("%s sitemap fetch failed: %v", target.Name, fetchErr))
			competitorSnapshots = append(competitorSnapshots, SiteSnapshot{
				Name:        target.Name,
				SitemapURL:  target.SitemapURL,
				ThemeCounts: map[string]int{},
				Error:       fetchErr.Error(),
			})
			continue
		}
		snapshot := buildSnapshot(target.Name, target.SitemapURL, entries, windowStart)
		snapshot, titleWarnings, titleDebug, err = enrichSnapshotTitles(ctx, titleFetcher, snapshot, titleEnrichmentLimit)
		if err != nil {
			return Summary{}, fmt.Errorf("title enrichment failed for %s: %w", target.Name, err)
		}
		debug.TitleEnrichment = append(debug.TitleEnrichment, titleDebug)
		warnings = append(warnings, titleWarnings...)
		competitorSnapshots = append(competitorSnapshots, snapshot)
	}

	opportunities := deriveOpportunities(ourSnapshot, competitorSnapshots)
	extractedTopics := []TopicSummary{}
	if cfg.OpenRouterAPIKey != "" {
		topics, promptDebug, topicErr := extractTopicsWithOpenRouter(ctx, cfg.OpenRouterAPIKey, cfg.OpenRouterModel, competitorSnapshots)
		debug.TopicPrompt = promptDebug
		if topicErr != nil {
			warnings = append(warnings, fmt.Sprintf("openrouter topic extraction skipped: %v", topicErr))
		} else {
			extractedTopics = topics
			topicOpportunities, analysisDebug := deriveTopicOpportunitiesWithDebug(ourSnapshot, topics)
			debug.TopicScoring = analysisDebug.ScoredTopics
			debug.SkippedTopics = analysisDebug.SkippedTopics
			if len(topicOpportunities) > 0 {
				opportunities = topicOpportunities
			}
		}
	}

	sort.Slice(opportunities, func(i, j int) bool {
		if opportunities[i].ImpactScore == opportunities[j].ImpactScore {
			return strings.ToLower(opportunities[i].Title) < strings.ToLower(opportunities[j].Title)
		}
		return opportunities[i].ImpactScore > opportunities[j].ImpactScore
	})
	contentPlan := buildContentRecommendations(opportunities)
	if cfg.OpenRouterAPIKey != "" && len(contentPlan) > 0 {
		draftModel := strings.TrimSpace(cfg.OpenRouterDraftModel)
		if draftModel == "" {
			draftModel = cfg.OpenRouterModel
		}
		createOSContext, contextErr := readGuidanceFile(createOSContextPath)
		if contextErr != nil {
			warnings = append(warnings, fmt.Sprintf("createos context skipped: %v", contextErr))
		}
		createOSWritingGuidelines, guidelinesErr := readGuidanceFile(createOSWritingGuidesPath)
		if guidelinesErr != nil {
			warnings = append(warnings, fmt.Sprintf("createos writing guidelines skipped: %v", guidelinesErr))
		}
		internalLinkInventory := buildCreateOSInternalLinkInventory(ourEntries, contentPlan)
		drafts, draftErr := generateContentDraftsWithOpenRouter(ctx, cfg.OpenRouterAPIKey, draftModel, contentPlan, cfg.CompetitorContentDraftLimit, createOSContext, createOSWritingGuidelines, cfg.OpenRouterDraftTimeoutSecs, internalLinkInventory)
		if draftErr != nil {
			warnings = append(warnings, fmt.Sprintf("openrouter blog draft generation skipped: %v", draftErr))
		} else {
			contentPlan = attachDraftsToContentRecommendations(contentPlan, drafts, cfg.CompetitorContentDraftLimit)
		}
	}

	return Summary{
		GeneratedAtUTC:  time.Now().UTC().Format(time.RFC3339),
		WindowDays:      cfg.CompetitorWindowDays,
		WindowStartUTC:  windowStart.Format(time.RFC3339),
		OurSite:         ourSnapshot,
		Competitors:     competitorSnapshots,
		ExtractedTopics: extractedTopics,
		Opportunities:   opportunities,
		ContentPlan:     contentPlan,
		Warnings:        warnings,
		OpenRouterModel: strings.TrimSpace(cfg.OpenRouterModel),
		Debug:           debug,
	}, nil
}

func readGuidanceFile(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(content)), nil
}

func enrichSnapshotTitles(ctx context.Context, fetcher *TitleFetcher, snapshot SiteSnapshot, limit int) (SiteSnapshot, []string, TitleEnrichmentDebug, error) {
	debug := TitleEnrichmentDebug{
		Site:            snapshot.Name,
		RecentURLCount:  len(snapshot.RecentURLs),
		EnrichmentLimit: limit,
	}
	if fetcher == nil || limit <= 0 {
		debug.EnrichmentReason = "disabled"
		return snapshot, nil, debug, nil
	}

	indexes := titleEnrichmentIndexes(snapshot, limit)
	debug.Attempted = len(indexes)
	debug.SkippedByFilter = len(snapshot.RecentURLs) - len(indexes)
	if len(indexes) == 0 {
		debug.EnrichmentReason = "no relevant recent urls"
		return snapshot, nil, debug, nil
	}

	enrichCtx, cancel := context.WithTimeout(ctx, titleEnrichmentTimeout)
	defer cancel()

	workerCount := titleEnrichmentConcurrency
	if len(indexes) < workerCount {
		workerCount = len(indexes)
	}

	jobs := make(chan int, len(indexes))
	for _, idx := range indexes {
		jobs <- idx
	}
	close(jobs)

	warningsByIndex := make([]string, len(snapshot.RecentURLs))
	var firstParentErr error
	var mu sync.Mutex
	var wg sync.WaitGroup

	for worker := 0; worker < workerCount; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				if enrichCtx.Err() != nil {
					if ctx.Err() != nil {
						recordTitleEnrichmentError(&mu, &firstParentErr, ctx.Err())
					}
					continue
				}

				url := snapshot.RecentURLs[idx].URL
				title, err := fetcher.FetchTitle(enrichCtx, url)
				if err != nil {
					if isContextError(err) {
						if ctx.Err() != nil {
							recordTitleEnrichmentError(&mu, &firstParentErr, ctx.Err())
						}
					} else {
						warningsByIndex[idx] = fmt.Sprintf("title fetch failed for %s: %v", url, err)
					}
					continue
				}
				snapshot.RecentURLs[idx].Title = title
			}
		}()
	}
	wg.Wait()

	mu.Lock()
	err := firstParentErr
	mu.Unlock()
	if err != nil {
		return snapshot, nil, debug, err
	}
	if ctx.Err() != nil {
		return snapshot, nil, debug, ctx.Err()
	}

	warnings := make([]string, 0)
	if errors.Is(enrichCtx.Err(), context.DeadlineExceeded) {
		warnings = append(warnings, fmt.Sprintf("title enrichment timed out for %s; using partial titles", snapshot.Name))
	}
	for _, idx := range indexes {
		if strings.TrimSpace(snapshot.RecentURLs[idx].Title) == "" {
			debug.EmptyTitle++
		} else {
			debug.Titled++
		}
	}
	for _, warning := range warningsByIndex {
		if warning == "" {
			continue
		}
		warnings = append(warnings, warning)
	}
	return snapshot, warnings, debug, nil
}

func titleEnrichmentIndexes(snapshot SiteSnapshot, limit int) []int {
	if limit <= 0 {
		return nil
	}
	indexes := make([]int, 0, limit)
	type indexedCandidate struct {
		idx       int
		candidate PageCandidate
	}
	candidates := make([]indexedCandidate, 0, len(snapshot.RecentURLs))
	for idx, entry := range snapshot.RecentURLs {
		candidate := pageCandidate(snapshot.Name, entry)
		enrichmentScore := candidate.RelevanceScore
		if containsString(candidate.NegativeSignals, "missing-title") {
			enrichmentScore += 30
		}
		if enrichmentScore < promptCandidateThreshold || containsString(candidate.NegativeSignals, "low-value-path") {
			continue
		}
		candidates = append(candidates, indexedCandidate{idx: idx, candidate: candidate})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].candidate.RelevanceScore == candidates[j].candidate.RelevanceScore {
			return candidates[i].candidate.URL < candidates[j].candidate.URL
		}
		return candidates[i].candidate.RelevanceScore > candidates[j].candidate.RelevanceScore
	})
	for _, candidate := range candidates {
		indexes = append(indexes, candidate.idx)
		if len(indexes) == limit {
			return indexes
		}
	}
	return indexes
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func recordTitleEnrichmentError(mu *sync.Mutex, firstErr *error, err error) {
	if err == nil {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	if *firstErr == nil {
		*firstErr = err
	}
}

func isContextError(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func buildSnapshot(name string, sitemapURL string, entries []rawSitemapEntry, windowStart time.Time) SiteSnapshot {
	recent := make([]SitemapEntry, 0)
	themeCounts := map[string]int{}

	for _, entry := range entries {
		if strings.TrimSpace(entry.URL) == "" {
			continue
		}
		if isJunkPath(name, entry.URL) {
			continue
		}
		themes := classifyThemes(entry.URL)
		effectiveLastMod := entry.LastMod
		if effectiveLastMod == nil {
			effectiveLastMod = inferDateFromURL(entry.URL)
		}
		if effectiveLastMod == nil {
			continue
		}
		if effectiveLastMod.Before(windowStart) {
			continue
		}
		for _, theme := range themes {
			themeCounts[theme]++
		}
		recentEntry := SitemapEntry{URL: entry.URL, ThemeTags: themes}
		v := effectiveLastMod.UTC().Format(time.RFC3339)
		recentEntry.LastMod = &v
		recent = append(recent, recentEntry)
	}

	sort.Slice(recent, func(i, j int) bool {
		left := recent[i].URL
		right := recent[j].URL
		if recent[i].LastMod != nil && recent[j].LastMod != nil && *recent[i].LastMod != *recent[j].LastMod {
			return *recent[i].LastMod > *recent[j].LastMod
		}
		if recent[i].LastMod != nil && recent[j].LastMod == nil {
			return true
		}
		if recent[i].LastMod == nil && recent[j].LastMod != nil {
			return false
		}
		return left < right
	})

	if len(recent) > 200 {
		recent = recent[:200]
	}

	return SiteSnapshot{
		Name:           name,
		SitemapURL:     sitemapURL,
		TotalURLs:      len(entries),
		RecentURLs:     recent,
		RecentURLCount: len(recent),
		ThemeCounts:    themeCounts,
	}
}
