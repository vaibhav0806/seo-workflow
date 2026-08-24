package competitor

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/nodeops/seo-workflow/internal/config"
)

func TestBuildApprovalQueueKeepsFocusedTopicsAndRanksStrongestFirst(t *testing.T) {
	queue := BuildApprovalQueue([]ContentRecommendation{
		{
			Priority:         1,
			SuggestedSlug:    "/blogs/consumer-photo-editing",
			SuggestedTitle:   "Consumer Photo Editing Apps",
			Theme:            "general",
			OpportunityScore: 99,
			Decision:         ContentDecisionCreate,
		},
		{
			Priority:         2,
			SuggestedSlug:    "/blogs/enterprise-agentic-ai-production",
			SuggestedTitle:   "Enterprise Agentic AI: Architecture, Governance, and Production",
			Theme:            "enterprise",
			OpportunityScore: 91,
			Decision:         ContentDecisionRefresh,
			ExistingRoute:    "/blogs/enterprise-agentic-ai",
			SourceEvidence:   []string{"https://example.com/enterprise-agents"},
		},
		{
			Priority:         3,
			SuggestedSlug:    "/blogs/ai-agent-sandboxes",
			SuggestedTitle:   "AI Agent Sandboxes for Safe Production Workflows",
			Theme:            "sandbox",
			OpportunityScore: 84,
			Decision:         ContentDecisionCreate,
		},
	}, time.Date(2026, 8, 24, 9, 0, 0, 0, time.UTC))

	require.Equal(t, "2026-08-24T09:00:00Z", queue.GeneratedAtUTC)
	require.Len(t, queue.Items, 2)
	require.Equal(t, "Enterprise Agentic AI: Architecture, Governance, and Production", queue.Items[0].SuggestedTitle)
	require.Equal(t, ApprovalPending, queue.Items[0].ApprovalStatus)
	require.Equal(t, "enterprise-agentic-ai-production", queue.Items[0].ApprovalID)
	require.Equal(t, 1, queue.Items[0].Priority)
	require.Equal(t, "AI Agent Sandboxes for Safe Production Workflows", queue.Items[1].SuggestedTitle)
}

func TestWriteApprovalQueuePreservesHumanDecisionAcrossDiscoveryRuns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "approvals.json")
	first := BuildApprovalQueue([]ContentRecommendation{{
		SuggestedSlug: "/blogs/ai-agent-sandboxes", SuggestedTitle: "AI Agent Sandboxes", Theme: "sandbox", Decision: ContentDecisionCreate,
	}}, time.Date(2026, 8, 24, 9, 0, 0, 0, time.UTC))
	require.NoError(t, WriteApprovalQueue(path, first))

	stored, err := ReadApprovalQueue(path)
	require.NoError(t, err)
	stored.Items[0].ApprovalStatus = ApprovalApproved
	data, err := marshalApprovalQueue(stored)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0o644))

	second := BuildApprovalQueue([]ContentRecommendation{{
		SuggestedSlug: "/blogs/ai-agent-sandboxes", SuggestedTitle: "AI Agent Sandboxes", Theme: "sandbox", Decision: ContentDecisionCreate,
	}}, time.Date(2026, 8, 25, 9, 0, 0, 0, time.UTC))
	require.NoError(t, WriteApprovalQueue(path, second))

	updated, err := ReadApprovalQueue(path)
	require.NoError(t, err)
	require.Equal(t, ApprovalApproved, updated.Items[0].ApprovalStatus)
}

func TestApprovedRecommendationsReturnsOnlyExplicitHumanApprovals(t *testing.T) {
	approved, err := ApprovedRecommendations(ApprovalQueue{Items: []ContentRecommendation{
		{ApprovalID: "pending", ApprovalStatus: ApprovalPending, SuggestedTitle: "Pending"},
		{ApprovalID: "approved", ApprovalStatus: ApprovalApproved, SuggestedTitle: "Approved"},
		{ApprovalID: "rejected", ApprovalStatus: ApprovalRejected, SuggestedTitle: "Rejected"},
	}})

	require.NoError(t, err)
	require.Len(t, approved, 1)
	require.Equal(t, "Approved", approved[0].SuggestedTitle)
}

func TestApprovedRecommendationsRejectsQueueWithoutApprovals(t *testing.T) {
	_, err := ApprovedRecommendations(ApprovalQueue{Items: []ContentRecommendation{{
		ApprovalID: "pending", ApprovalStatus: ApprovalPending, SuggestedTitle: "Pending",
	}}})

	require.EqualError(t, err, "approval queue has no approved blog opportunities")
}

func TestRunApprovedContentStopsBeforeDraftingWhenNothingWasApproved(t *testing.T) {
	path := filepath.Join(t.TempDir(), "approvals.json")
	require.NoError(t, WriteApprovalQueue(path, ApprovalQueue{Items: []ContentRecommendation{{
		ApprovalID: "pending", ApprovalStatus: ApprovalPending, SuggestedTitle: "AI Agent Sandboxes",
	}}}))

	_, err := RunApprovedContent(context.Background(), &config.Config{
		ContentApprovalPath: path,
		OpenRouterAPIKey:    "would-call-external-service-without-the-gate",
	})

	require.EqualError(t, err, "approval queue has no approved blog opportunities")
}

func TestBuildContentInventoryInternalLinksUsesExistingBlogRoutes(t *testing.T) {
	links := buildContentInventoryInternalLinks(ContentInventory{Pages: []ExistingContent{
		{Route: "/blogs/enterprise-agentic-ai", Title: "Enterprise Agentic AI"},
		{Route: "/blogs/ai-agent-sandboxes", Title: "AI Agent Sandboxes"},
	}}, []ContentRecommendation{{
		SuggestedTitle: "Production AI Agent Workflows",
		PrimaryKeyword: "production ai agent workflows",
	}})

	require.Len(t, links, 2)
	require.Equal(t, "https://createos.sh"+links[0].Path, links[0].URL)
	require.Contains(t, []string{"Enterprise Agentic AI", "AI Agent Sandboxes"}, links[0].Title)
}
