package competitor_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nodeops/seo-workflow/internal/competitor"
	"github.com/nodeops/seo-workflow/internal/contentrepo"
	"github.com/stretchr/testify/require"
)

func TestE2EBlogInventoryDecisionAndPublishContract(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, "blogs"), 0o755))
	existing := `---
title: "Deploy AI Agents"
slug: deploy-ai-agents
description: "Deploy production AI agents."
author: "Naman Kabra"
primary_keyword: "deploy ai agents"
secondary_keywords:
  - "AI agent deployment"
---

# Deploy AI Agents

Existing CreateOS guidance.
`
	require.NoError(t, os.WriteFile(filepath.Join(root, "blogs", "deploy-ai-agents.md"), []byte(existing), 0o644))
	inventory, err := competitor.LoadContentInventoryFromDir(root)
	require.NoError(t, err)
	plan, report := competitor.DecideContentPlan([]competitor.ContentRecommendation{
		{SuggestedTitle: "Deploy AI Agents", SuggestedSlug: "/blogs/deploy-ai-agents", PrimaryKeyword: "deploy ai agents"},
		{SuggestedTitle: "MCP Hosting Guide", SuggestedSlug: "/blogs/mcp-hosting-guide", PrimaryKeyword: "mcp hosting", SecondaryKeywords: []string{"host MCP servers"}},
	}, inventory)

	require.Equal(t, 1, report.ExistingCount)
	require.Equal(t, competitor.ContentDecisionRefresh, plan[0].Decision)
	require.Nil(t, plan[0].Draft)
	require.Equal(t, competitor.ContentDecisionCreate, plan[1].Decision)
	plan[1].Draft = &competitor.BlogDraft{
		Route: "/blogs/mcp-hosting-guide", Title: "MCP Hosting Guide",
		MetaDescription: "A practical guide to hosting MCP servers with CreateOS.",
		BodyMarkdown:    "# MCP Hosting Guide\n\nDeploy and operate an MCP server.",
	}
	post, err := contentrepo.BuildBlogPost(plan[1], time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC), "Naman Kabra", "https://example.com/cover.png")
	require.NoError(t, err)
	require.Equal(t, "blogs/mcp-hosting-guide.md", post.FilePath())
	require.Contains(t, post.Markdown(), `primary_keyword: "mcp hosting"`)
	require.Contains(t, post.Markdown(), `author: "Naman Kabra"`)
}

func TestE2ELocalCreateOSContentInventory(t *testing.T) {
	root := strings.TrimSpace(os.Getenv("CREATEOS_CONTENT_E2E_PATH"))
	if root == "" {
		t.Skip("CREATEOS_CONTENT_E2E_PATH is not set")
	}
	inventory, err := competitor.LoadContentInventoryFromDir(root)
	require.NoError(t, err)
	require.NotEmpty(t, inventory.Pages)
	namanPosts := 0
	for _, page := range inventory.Pages {
		require.True(t, strings.HasPrefix(page.Route, "/blogs/"), page.Route)
		if page.Author == "Naman Kabra" {
			namanPosts++
		}
	}
	require.Positive(t, namanPosts)
}
