package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/nodeops/seo-workflow/internal/competitor"
	"github.com/nodeops/seo-workflow/internal/config"
	"github.com/stretchr/testify/require"
)

func TestWriteCompetitorNotionReportSkipsWhenUnconfigured(t *testing.T) {
	err := writeCompetitorNotionReport(context.Background(), &config.Config{}, competitor.Summary{})

	require.NoError(t, err)
}

func TestWriteCompetitorReportCreatesParentDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "competitor-report.json")

	err := writeCompetitorReport(&config.Config{CompetitorReportPath: path}, competitor.Summary{
		GeneratedAtUTC: "2026-08-24T12:00:00Z",
	})

	require.NoError(t, err)
	data, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	require.Contains(t, string(data), `"generatedAtUtc": "2026-08-24T12:00:00Z"`)
}

func TestWriteCompetitorNotionReportRequiresBothEnvValues(t *testing.T) {
	err := writeCompetitorNotionReport(context.Background(), &config.Config{NotionAPIKey: "ntn_test"}, competitor.Summary{})

	require.Error(t, err)
	require.Contains(t, err.Error(), "NOTION_API_KEY")
	require.Contains(t, err.Error(), "NOTION_COMPETITOR_REPORT_PARENT_PAGE_ID")
}
