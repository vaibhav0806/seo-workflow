package competitor

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/nodeops/seo-workflow/internal/config"
	"github.com/stretchr/testify/require"
)

func TestLoadContentInventoryWithOpenPullRequestsMergesPendingBlogs(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, "blogs"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "blogs", "existing.md"), []byte(`---
title: Existing Blog
slug: existing
primary_keyword: existing topic
---
# Existing Blog
`), 0o644))
	pending := `---
title: Role-Specific AI Productivity Tools
slug: role-specific-ai-productivity-tools
primary_keyword: role specific ai productivity tools
---
# Role-Specific AI Productivity Tools
`
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/repos/NodeOps-app/createos-content/pulls":
			require.Equal(t, "open", request.URL.Query().Get("state"))
			require.Equal(t, "main", request.URL.Query().Get("base"))
			_, _ = response.Write([]byte(`[{"number":146,"head":{"sha":"pending-sha"}}]`))
		case "/repos/NodeOps-app/createos-content/pulls/146/files":
			_, _ = response.Write([]byte(`[{"filename":"blogs/role-specific-ai-productivity-tools.md","status":"added"}]`))
		case "/repos/NodeOps-app/createos-content/contents/blogs/role-specific-ai-productivity-tools.md":
			require.Equal(t, "pending-sha", request.URL.Query().Get("ref"))
			_, _ = response.Write([]byte(`{"encoding":"base64","content":"` + base64.StdEncoding.EncodeToString([]byte(pending)) + `"}`))
		default:
			http.Error(response, "unexpected path", http.StatusTeapot)
		}
	}))
	defer server.Close()
	previousBase := githubInventoryAPIBase
	githubInventoryAPIBase = server.URL
	t.Cleanup(func() { githubInventoryAPIBase = previousBase })

	inventory, err := LoadContentInventoryWithOpenPullRequests(
		context.Background(), root, "ghp_test", "NodeOps-app/createos-content", "main",
	)

	require.NoError(t, err)
	require.Len(t, inventory.Pages, 2)
	require.Equal(t, "/blogs/existing", inventory.Pages[0].Route)
	require.Equal(t, "/blogs/role-specific-ai-productivity-tools", inventory.Pages[1].Route)
}

func TestLoadContentInventoryFetchesCreateOSContentArchive(t *testing.T) {
	archive := contentArchive(t, map[string]string{
		"createos-content-main/blogs/ai-agent-governance.md": `---
title: AI Agent Governance
slug: ai-agent-governance
author: Naman Kabra
primary_keyword: ai agent governance
---
# AI Agent Governance
`,
	})
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		require.Equal(t, "/repos/NodeOps-app/createos-content/tarball/main", request.URL.Path)
		require.Equal(t, "Bearer ghp_test", request.Header.Get("Authorization"))
		response.Header().Set("Content-Type", "application/gzip")
		_, _ = response.Write(archive)
	}))
	defer server.Close()
	previousBase := githubInventoryAPIBase
	githubInventoryAPIBase = server.URL
	t.Cleanup(func() { githubInventoryAPIBase = previousBase })

	inventory, err := LoadContentInventory(context.Background(), "", "ghp_test", "NodeOps-app/createos-content", "main")

	require.NoError(t, err)
	require.Len(t, inventory.Pages, 1)
	require.Equal(t, "/blogs/ai-agent-governance", inventory.Pages[0].Route)
}

func contentArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var compressed bytes.Buffer
	gzipWriter := gzip.NewWriter(&compressed)
	tarWriter := tar.NewWriter(gzipWriter)
	for name, content := range files {
		require.NoError(t, tarWriter.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(content))}))
		_, err := io.WriteString(tarWriter, content)
		require.NoError(t, err)
	}
	require.NoError(t, tarWriter.Close())
	require.NoError(t, gzipWriter.Close())
	return compressed.Bytes()
}

func TestLoadContentInventoryReadsNamanKeywordContract(t *testing.T) {
	root := t.TempDir()
	blogs := filepath.Join(root, "blogs")
	require.NoError(t, os.MkdirAll(blogs, 0o755))
	markdown := `---
title: "AI Agent Governance: Runtime Controls"
slug: ai-agent-governance
description: "Map AI policy to runtime controls."
author: "Naman Kabra"
primary_keyword: "AI agent governance"
secondary_keywords:
  - "agent governance"
  - "AI policy enforcement"
unique_perspective: "Policy-to-control mapping."
target_funnel: b2b
internal_link_targets:
  - url: /blogs/ai-agent-audit-trails-enterprise
    anchor: audit trail guide
---

# AI Agent Governance: Runtime Controls

## Runtime approval gates

Read the [audit trail guide](/blogs/ai-agent-audit-trails-enterprise).
`
	require.NoError(t, os.WriteFile(filepath.Join(blogs, "ai-agent-governance.md"), []byte(markdown), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(blogs, "ai-agent-governance.schema.json"), []byte(`{}`), 0o644))

	inventory, err := LoadContentInventoryFromDir(root)

	require.NoError(t, err)
	require.Len(t, inventory.Pages, 1)
	page := inventory.Pages[0]
	require.Equal(t, "/blogs/ai-agent-governance", page.Route)
	require.Equal(t, "Naman Kabra", page.Author)
	require.Equal(t, "AI agent governance", page.PrimaryKeyword)
	require.Equal(t, []string{"agent governance", "AI policy enforcement"}, page.SecondaryKeywords)
	require.Equal(t, "Policy-to-control mapping.", page.UniquePerspective)
	require.Equal(t, "b2b", page.TargetFunnel)
	require.Contains(t, page.Headings, "Runtime approval gates")
	require.Contains(t, page.InternalLinks, "/blogs/ai-agent-audit-trails-enterprise")
}

func TestDecideContentPlanRefreshesExistingCanonicalInsteadOfDraftingDuplicate(t *testing.T) {
	inventory := ContentInventory{Pages: []ExistingContent{
		{Path: "blogs/ai-agent-governance.md", Route: "/blogs/ai-agent-governance", Title: "AI Agent Governance", PrimaryKeyword: "AI agent governance"},
	}}
	plan := []ContentRecommendation{
		{SuggestedSlug: "/blogs/enterprise-agent-governance", SuggestedTitle: "Enterprise AI Agent Governance", PrimaryKeyword: "AI agent governance"},
		{SuggestedSlug: "/blogs/mcp-deployment", SuggestedTitle: "MCP Deployment Guide", PrimaryKeyword: "MCP deployment"},
	}

	decided, report := DecideContentPlan(plan, inventory)

	require.Equal(t, ContentDecisionRefresh, decided[0].Decision)
	require.Equal(t, "/blogs/ai-agent-governance", decided[0].ExistingRoute)
	require.Nil(t, decided[0].Draft)
	require.Equal(t, ContentDecisionCreate, decided[1].Decision)
	require.Empty(t, decided[1].ExistingRoute)
	require.Empty(t, report.Cannibalization)
}

func TestDecideContentPlanTargetsAgenticPillarInsteadOfGenericEnterpriseDeploymentPost(t *testing.T) {
	inventory := ContentInventory{Pages: []ExistingContent{
		{
			Route:             "/blogs/createos-openai-select-partner",
			Title:             "CreateOS Is Now an OpenAI Select Partner",
			PrimaryKeyword:    "OpenAI Select Partner",
			SecondaryKeywords: []string{"enterprise AI deployment", "governed AI agents"},
		},
		{
			Route:             "/blogs/enterprise-ai-execution-layer",
			Title:             "Enterprise Agentic AI Strategy: Architecture to Production",
			PrimaryKeyword:    "enterprise agentic AI strategy",
			SecondaryKeywords: []string{"enterprise AI agent architecture", "enterprise AI governance"},
		},
	}}

	decided, _ := DecideContentPlan([]ContentRecommendation{{
		SuggestedSlug:  "/blogs/enterprise-agentic-ai-deployment",
		SuggestedTitle: "Enterprise Agentic AI Deployment: Architecture, Governance, and Production",
		PrimaryKeyword: "enterprise agentic AI deployment",
	}}, inventory)

	require.Equal(t, ContentDecisionRefresh, decided[0].Decision)
	require.Equal(t, "/blogs/enterprise-ai-execution-layer", decided[0].ExistingRoute)
}

func TestDecideContentPlanConsolidatesWhenKeywordHasMultipleCanonicalPages(t *testing.T) {
	inventory := ContentInventory{Pages: []ExistingContent{
		{Route: "/blogs/ai-agent-governance", Title: "AI Agent Governance", PrimaryKeyword: "AI agent governance"},
		{Route: "/blogs/governance-that-works", Title: "Governance That Works", PrimaryKeyword: "AI agent governance"},
	}}

	decided, report := DecideContentPlan([]ContentRecommendation{{
		SuggestedSlug:  "/blogs/enterprise-agent-governance",
		SuggestedTitle: "Enterprise AI Agent Governance",
		PrimaryKeyword: "AI agent governance",
	}}, inventory)

	require.Equal(t, ContentDecisionConsolidate, decided[0].Decision)
	require.Len(t, report.Cannibalization, 1)
	require.Equal(t, "AI agent governance", report.Cannibalization[0].Keyword)
	require.ElementsMatch(t, []string{"/blogs/ai-agent-governance", "/blogs/governance-that-works"}, report.Cannibalization[0].Routes)
}

func TestDraftPromptInputSkipsRefreshAndConsolidationWork(t *testing.T) {
	input := draftPromptInput([]ContentRecommendation{
		{SuggestedSlug: "/blogs/new", SuggestedTitle: "New", Decision: ContentDecisionCreate},
		{SuggestedSlug: "/blogs/existing", SuggestedTitle: "Existing", Decision: ContentDecisionRefresh},
		{SuggestedSlug: "/blogs/duplicate", SuggestedTitle: "Duplicate", Decision: ContentDecisionConsolidate},
	}, 3, nil)

	require.Len(t, input, 1)
	require.Equal(t, "/blogs/new", input[0].Route)
}

func TestApplyConfiguredInventoryUsesLocalCreateOSContentCheckout(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "blogs"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "blogs", "governance.md"), []byte(`---
title: AI Agent Governance
slug: governance
primary_keyword: ai agent governance
---
# AI Agent Governance
`), 0o644))

	plan, report, warnings := applyConfiguredInventory(context.Background(), &config.Config{ContentInventoryPath: root}, []ContentRecommendation{{
		SuggestedSlug:  "/blogs/agent-governance",
		SuggestedTitle: "AI Agent Governance",
		PrimaryKeyword: "ai agent governance",
	}})

	require.Empty(t, warnings)
	require.Equal(t, 1, report.ExistingCount)
	require.Equal(t, ContentDecisionRefresh, plan[0].Decision)
}

func TestApplyConfiguredInventoryKeepsLocalPagesWhenOpenPRLookupFails(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "blogs"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "blogs", "governance.md"), []byte(`---
title: AI Agent Governance
slug: governance
primary_keyword: ai agent governance
---
# AI Agent Governance
`), 0o644))
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		http.Error(response, "bad credentials", http.StatusUnauthorized)
	}))
	defer server.Close()
	previousBase := githubInventoryAPIBase
	githubInventoryAPIBase = server.URL
	t.Cleanup(func() { githubInventoryAPIBase = previousBase })

	plan, report, warnings := applyConfiguredInventory(context.Background(), &config.Config{
		ContentInventoryPath: root,
		GitHubToken:          "expired-token",
		ContentRepo:          "NodeOps-app/createos-content",
		ContentBaseBranch:    "main",
	}, []ContentRecommendation{{
		SuggestedSlug: "/blogs/agent-governance", SuggestedTitle: "AI Agent Governance", PrimaryKeyword: "ai agent governance",
	}})

	require.Equal(t, 1, report.ExistingCount)
	require.Equal(t, ContentDecisionRefresh, plan[0].Decision)
	require.Len(t, warnings, 1)
	require.Contains(t, warnings[0], "open content pull requests")
}
