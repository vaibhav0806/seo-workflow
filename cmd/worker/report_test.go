package main

import (
	"context"
	"testing"

	"github.com/nodeops/seo-workflow/internal/competitor"
	"github.com/nodeops/seo-workflow/internal/config"
	"github.com/stretchr/testify/require"
)

func TestWriteCompetitorNotionReportSkipsWhenUnconfigured(t *testing.T) {
	err := writeCompetitorNotionReport(context.Background(), &config.Config{}, competitor.Summary{})

	require.NoError(t, err)
}

func TestWriteCompetitorNotionReportRequiresBothEnvValues(t *testing.T) {
	err := writeCompetitorNotionReport(context.Background(), &config.Config{NotionAPIKey: "ntn_test"}, competitor.Summary{})

	require.Error(t, err)
	require.Contains(t, err.Error(), "NOTION_API_KEY")
	require.Contains(t, err.Error(), "NOTION_COMPETITOR_REPORT_PARENT_PAGE_ID")
}
