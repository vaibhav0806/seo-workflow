package gsc

import (
	"sort"
	"strings"
)

type PerformanceMetric struct {
	Date        string  `json:"date"`
	Query       string  `json:"query"`
	Page        string  `json:"page"`
	Clicks      float64 `json:"clicks"`
	Impressions float64 `json:"impressions"`
	CTR         float64 `json:"ctr"`
	Position    float64 `json:"position"`
}

type PerformanceSnapshot struct {
	GeneratedAtUTC string              `json:"generatedAtUtc"`
	StartDate      string              `json:"startDate"`
	EndDate        string              `json:"endDate"`
	Rows           []PerformanceMetric `json:"rows"`
}

type OpportunityType string

const (
	OpportunityStrikingDistance OpportunityType = "striking_distance"
	OpportunityLowCTR           OpportunityType = "low_ctr"
	OpportunityDeclining        OpportunityType = "declining"
)

type SearchOpportunity struct {
	Type        OpportunityType `json:"type"`
	Query       string          `json:"query"`
	Page        string          `json:"page"`
	Score       int             `json:"score"`
	Clicks      float64         `json:"clicks"`
	Impressions float64         `json:"impressions"`
	CTR         float64         `json:"ctr"`
	Position    float64         `json:"position"`
	Reason      string          `json:"reason"`
}

type QueryCannibalization struct {
	Query string   `json:"query"`
	Pages []string `json:"pages"`
}

type PerformanceReport struct {
	Current         PerformanceSnapshot    `json:"current"`
	Previous        *PerformanceSnapshot   `json:"previous,omitempty"`
	Opportunities   []SearchOpportunity    `json:"opportunities,omitempty"`
	Cannibalization []QueryCannibalization `json:"cannibalization,omitempty"`
}

type aggregateKey struct {
	query string
	page  string
}

func AnalyzePerformance(current PerformanceSnapshot, previous PerformanceSnapshot) PerformanceReport {
	report := PerformanceReport{Current: current}
	if len(previous.Rows) > 0 {
		copyPrevious := previous
		report.Previous = &copyPrevious
	}
	currentAggregates := aggregatePerformance(current.Rows)
	previousAggregates := aggregatePerformance(previous.Rows)
	for key, metric := range currentAggregates {
		if metric.Impressions >= 10 && metric.Position >= 4 && metric.Position <= 15 {
			report.Opportunities = append(report.Opportunities, SearchOpportunity{
				Type: OpportunityStrikingDistance, Query: key.query, Page: key.page, Score: 80,
				Clicks: metric.Clicks, Impressions: metric.Impressions, CTR: metric.CTR, Position: metric.Position,
				Reason: "Query ranks within positions 4-15 and has enough impressions to justify improving the existing page.",
			})
		}
		if metric.Impressions >= 20 && metric.Position <= 10 && metric.CTR < 0.02 {
			report.Opportunities = append(report.Opportunities, SearchOpportunity{
				Type: OpportunityLowCTR, Query: key.query, Page: key.page, Score: 78,
				Clicks: metric.Clicks, Impressions: metric.Impressions, CTR: metric.CTR, Position: metric.Position,
				Reason: "Query has first-page visibility but fewer than two clicks per hundred impressions.",
			})
		}
		if old, exists := previousAggregates[key]; exists && old.Clicks >= 4 && metric.Clicks <= old.Clicks*0.75 {
			report.Opportunities = append(report.Opportunities, SearchOpportunity{
				Type: OpportunityDeclining, Query: key.query, Page: key.page, Score: 86,
				Clicks: metric.Clicks, Impressions: metric.Impressions, CTR: metric.CTR, Position: metric.Position,
				Reason: "Clicks declined by at least 25 percent compared with the previous snapshot.",
			})
		}
	}
	report.Cannibalization = performanceCannibalization(currentAggregates)
	sort.Slice(report.Opportunities, func(i, j int) bool {
		if report.Opportunities[i].Score == report.Opportunities[j].Score {
			if report.Opportunities[i].Query == report.Opportunities[j].Query {
				return report.Opportunities[i].Page < report.Opportunities[j].Page
			}
			return report.Opportunities[i].Query < report.Opportunities[j].Query
		}
		return report.Opportunities[i].Score > report.Opportunities[j].Score
	})
	return report
}

func aggregatePerformance(rows []PerformanceMetric) map[aggregateKey]PerformanceMetric {
	aggregates := map[aggregateKey]PerformanceMetric{}
	positionWeights := map[aggregateKey]float64{}
	for _, row := range rows {
		query := strings.TrimSpace(strings.ToLower(row.Query))
		page := strings.TrimSpace(row.Page)
		if query == "" || page == "" {
			continue
		}
		key := aggregateKey{query: query, page: page}
		metric := aggregates[key]
		metric.Query = query
		metric.Page = page
		metric.Clicks += row.Clicks
		metric.Impressions += row.Impressions
		weight := row.Impressions
		if weight <= 0 {
			weight = 1
		}
		metric.Position += row.Position * weight
		positionWeights[key] += weight
		aggregates[key] = metric
	}
	for key, metric := range aggregates {
		if metric.Impressions > 0 {
			metric.CTR = metric.Clicks / metric.Impressions
		}
		if positionWeights[key] > 0 {
			metric.Position /= positionWeights[key]
		}
		aggregates[key] = metric
	}
	return aggregates
}

func performanceCannibalization(aggregates map[aggregateKey]PerformanceMetric) []QueryCannibalization {
	queryPages := map[string][]string{}
	for key, metric := range aggregates {
		if metric.Impressions <= 0 {
			continue
		}
		queryPages[key.query] = append(queryPages[key.query], key.page)
	}
	findings := make([]QueryCannibalization, 0)
	for query, pages := range queryPages {
		if len(pages) < 2 {
			continue
		}
		sort.Strings(pages)
		findings = append(findings, QueryCannibalization{Query: query, Pages: pages})
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].Query < findings[j].Query })
	return findings
}
