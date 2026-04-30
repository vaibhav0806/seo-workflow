package db

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPhase1CoreTablesMigrationContainsRequiredTables(t *testing.T) {
	migrationPath := filepath.Join("..", "..", "db", "migrations", "0001_phase1_core_tables.sql")

	content, err := os.ReadFile(migrationPath)
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS properties")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS scans")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS findings")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS pull_requests")

	require.Contains(t, sql, "gsc_property TEXT NOT NULL UNIQUE")
	require.Contains(t, sql, "property_id BIGINT NOT NULL REFERENCES properties(id)")
	require.Contains(t, sql, "scan_id BIGINT NOT NULL REFERENCES scans(id)")
	require.Contains(t, sql, "status TEXT NOT NULL CHECK (status IN ('running', 'success', 'failed'))")
	require.Contains(t, sql, "'running'")
	require.Contains(t, sql, "'success'")
	require.Contains(t, sql, "'failed'")

	require.Contains(t, sql, "url TEXT NOT NULL")
	require.Contains(t, sql, "bucket TEXT NOT NULL")
	require.Contains(t, sql, "coverage_state TEXT NOT NULL")
	require.Contains(t, sql, "page_fetch_state TEXT NOT NULL")
	require.Contains(t, sql, "pr_url TEXT NOT NULL")
	require.Contains(t, sql, "branch_name TEXT NOT NULL")
}
