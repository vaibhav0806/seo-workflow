package oneshot

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nodeops/seo-workflow/internal/gsc"
	"github.com/stretchr/testify/require"
)

func TestQuerySearchPerformancePaginatesQueryPageDateRows(t *testing.T) {
	startRows := make([]int, 0)
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		var payload struct {
			Dimensions []string `json:"dimensions"`
			RowLimit   int      `json:"rowLimit"`
			StartRow   int      `json:"startRow"`
		}
		require.NoError(t, json.NewDecoder(request.Body).Decode(&payload))
		require.Equal(t, []string{"date", "query", "page"}, payload.Dimensions)
		require.Equal(t, 2, payload.RowLimit)
		startRows = append(startRows, payload.StartRow)
		response.Header().Set("Content-Type", "application/json")
		if payload.StartRow == 0 {
			_, _ = response.Write([]byte(`{"rows":[
                {"keys":["2026-08-20","deploy ai agent","https://createos.sh/blogs/deploy-ai-agent"],"clicks":2,"impressions":20,"ctr":0.1,"position":8},
                {"keys":["2026-08-20","mcp hosting","https://createos.sh/blogs/mcp-hosting"],"clicks":1,"impressions":10,"ctr":0.1,"position":6}
            ]}`))
			return
		}
		_, _ = response.Write([]byte(`{"rows":[
            {"keys":["2026-08-21","deploy ai agent","https://createos.sh/blogs/deploy-ai-agent"],"clicks":3,"impressions":30,"ctr":0.1,"position":7}
        ]}`))
	}))
	defer server.Close()
	previousEndpoint := searchAnalyticsEndpoint
	searchAnalyticsEndpoint = server.URL + "/%s"
	t.Cleanup(func() { searchAnalyticsEndpoint = previousEndpoint })
	adapter := NewGSCAdapter("token", "https://createos.sh/sitemap.xml", 7, 2, 5)

	metrics, startDate, endDate, err := adapter.querySearchPerformance(context.Background(), "sc-domain:createos.sh")

	require.NoError(t, err)
	require.Len(t, metrics, 3)
	require.Equal(t, []int{0, 2}, startRows)
	require.NotEmpty(t, startDate)
	require.NotEmpty(t, endDate)
	require.Equal(t, "deploy ai agent", metrics[0].Query)
	require.Equal(t, float64(2), metrics[0].Clicks)
}

func TestAggregatePerformanceURLsSumsImpressionsByPage(t *testing.T) {
	metrics := aggregatePerformanceURLs([]gsc.PerformanceMetric{
		{Page: "https://createos.sh/blogs/deploy-ai-agent", Impressions: 20},
		{Page: "https://createos.sh/blogs/deploy-ai-agent", Impressions: 30},
		{Page: "https://createos.sh/blogs/mcp-hosting", Impressions: 10},
	})

	require.Equal(t, int64(50), metrics[0].Impressions)
	require.Equal(t, "https://createos.sh/blogs/deploy-ai-agent", metrics[0].URL)
}
