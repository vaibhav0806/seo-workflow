package competitor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/nodeops/seo-workflow/internal/config"
)

type ApprovalStatus string

const (
	ApprovalPending  ApprovalStatus = "pending"
	ApprovalApproved ApprovalStatus = "approved"
	ApprovalRejected ApprovalStatus = "rejected"
)

type ApprovalQueue struct {
	GeneratedAtUTC string                  `json:"generatedAtUtc"`
	Instructions   string                  `json:"instructions"`
	Items          []ContentRecommendation `json:"items"`
}

func BuildApprovalQueue(plan []ContentRecommendation, generatedAt time.Time) ApprovalQueue {
	items := make([]ContentRecommendation, 0, len(plan))
	for _, recommendation := range plan {
		if !isCreateOSFocusTopic(recommendation) || recommendation.Decision == ContentDecisionSkip {
			continue
		}
		recommendation.ApprovalID = approvalID(recommendation)
		recommendation.ApprovalStatus = ApprovalPending
		recommendation.RankingScore = blogOpportunityRank(recommendation)
		recommendation.Draft = nil
		items = append(items, recommendation)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].RankingScore == items[j].RankingScore {
			return strings.ToLower(items[i].SuggestedTitle) < strings.ToLower(items[j].SuggestedTitle)
		}
		return items[i].RankingScore > items[j].RankingScore
	})
	for idx := range items {
		items[idx].Priority = idx + 1
	}
	return ApprovalQueue{
		GeneratedAtUTC: generatedAt.UTC().Format(time.RFC3339),
		Instructions:   "Set approvalStatus to approved or rejected. Only approved items with decision=create generate drafts and content pull requests; refresh and consolidate items require editorial updates to existing pages.",
		Items:          items,
	}
}

func RunApprovedContent(ctx context.Context, cfg *config.Config) (Summary, error) {
	if cfg == nil {
		return Summary{}, fmt.Errorf("approved content config is nil")
	}
	queue, err := ReadApprovalQueue(cfg.ContentApprovalPath)
	if err != nil {
		return Summary{}, err
	}
	approved, err := ApprovedRecommendations(queue)
	if err != nil {
		return Summary{}, err
	}
	draftable := make([]ContentRecommendation, 0, len(approved))
	for _, recommendation := range approved {
		if recommendation.Decision == "" || recommendation.Decision == ContentDecisionCreate {
			recommendation.Decision = ContentDecisionCreate
			draftable = append(draftable, recommendation)
		}
	}
	if len(draftable) == 0 {
		return Summary{}, fmt.Errorf("approval queue has no approved create decisions")
	}
	inventory, err := LoadContentInventoryWithOpenPullRequests(ctx, cfg.ContentInventoryPath, cfg.GitHubToken, cfg.ContentRepo, cfg.ContentBaseBranch)
	if err != nil {
		return Summary{}, fmt.Errorf("load content inventory before approved drafting: %w", err)
	}
	draftable, inventoryReport := DecideContentPlan(draftable, inventory)
	createDecisions := make([]ContentRecommendation, 0, len(draftable))
	for _, recommendation := range draftable {
		if recommendation.Decision == ContentDecisionCreate {
			createDecisions = append(createDecisions, recommendation)
		}
	}
	if len(createDecisions) == 0 {
		return Summary{}, fmt.Errorf("approved titles now overlap existing content; rediscover and approve the refresh or consolidation action")
	}
	draftable = createDecisions
	internalLinkInventory := buildContentInventoryInternalLinks(inventory, draftable)

	draftModel := strings.TrimSpace(cfg.OpenRouterDraftModel)
	if draftModel == "" {
		draftModel = cfg.OpenRouterModel
	}
	createOSContext, contextErr := readGuidanceFile(createOSContextPath)
	guidelines, guidelinesErr := readGuidanceFile(createOSWritingGuidesPath)
	warnings := make([]string, 0, 2)
	if contextErr != nil {
		warnings = append(warnings, fmt.Sprintf("createos context skipped: %v", contextErr))
	}
	if guidelinesErr != nil {
		warnings = append(warnings, fmt.Sprintf("createos writing guidelines skipped: %v", guidelinesErr))
	}
	drafts, err := generateContentDraftsWithOpenRouter(
		ctx,
		cfg.OpenRouterAPIKey,
		draftModel,
		cfg.OpenRouterDraftFallbackModel,
		draftable,
		cfg.CompetitorContentDraftLimit,
		createOSContext,
		guidelines,
		cfg.OpenRouterDraftTimeoutSecs,
		internalLinkInventory,
	)
	if err != nil {
		return Summary{}, fmt.Errorf("generate approved content drafts: %w", err)
	}
	draftable = attachDraftsToContentRecommendations(draftable, drafts, cfg.CompetitorContentDraftLimit)
	return Summary{
		GeneratedAtUTC:  time.Now().UTC().Format(time.RFC3339),
		ContentPlan:     draftable,
		InventoryReport: inventoryReport,
		OpenRouterModel: draftModel,
		Warnings:        warnings,
	}, nil
}

func ReadApprovalQueue(path string) (ApprovalQueue, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ApprovalQueue{}, fmt.Errorf("read approval queue %q: %w", path, err)
	}
	var queue ApprovalQueue
	if err := json.Unmarshal(data, &queue); err != nil {
		return ApprovalQueue{}, fmt.Errorf("decode approval queue %q: %w", path, err)
	}
	return queue, nil
}

func WriteApprovalQueue(path string, queue ApprovalQueue) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("approval queue path is required")
	}
	if existing, err := ReadApprovalQueue(path); err == nil {
		decisions := make(map[string]ApprovalStatus, len(existing.Items))
		for _, item := range existing.Items {
			if item.ApprovalStatus == ApprovalApproved || item.ApprovalStatus == ApprovalRejected {
				decisions[item.ApprovalID] = item.ApprovalStatus
			}
		}
		for idx := range queue.Items {
			if decision, ok := decisions[queue.Items[idx].ApprovalID]; ok {
				queue.Items[idx].ApprovalStatus = decision
			}
		}
	}
	data, err := marshalApprovalQueue(queue)
	if err != nil {
		return err
	}
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create approval queue directory %q: %w", dir, err)
		}
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write approval queue %q: %w", path, err)
	}
	return nil
}

func ApprovedRecommendations(queue ApprovalQueue) ([]ContentRecommendation, error) {
	approved := make([]ContentRecommendation, 0, len(queue.Items))
	for _, item := range queue.Items {
		if item.ApprovalStatus == ApprovalApproved {
			approved = append(approved, item)
		}
	}
	if len(approved) == 0 {
		return nil, fmt.Errorf("approval queue has no approved blog opportunities")
	}
	return approved, nil
}

func marshalApprovalQueue(queue ApprovalQueue) ([]byte, error) {
	data, err := json.MarshalIndent(queue, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode approval queue: %w", err)
	}
	return append(data, '\n'), nil
}

func approvalID(recommendation ContentRecommendation) string {
	slug := strings.Trim(strings.TrimSpace(recommendation.SuggestedSlug), "/")
	slug = strings.TrimPrefix(slug, "blogs/")
	if slug != "" {
		return slug
	}
	return safeSlug(recommendation.SuggestedTitle)
}

func isCreateOSFocusTopic(recommendation ContentRecommendation) bool {
	raw := strings.ToLower(strings.Join([]string{
		recommendation.SuggestedTitle,
		recommendation.PrimaryKeyword,
		recommendation.Theme,
		recommendation.ContentAngle,
		recommendation.Pillar,
	}, " "))
	focusTerms := []string{
		"agentic", "ai agent", "agent workflow", "sandbox", "enterprise ai",
		"enterprise agent", "production workflow", "production ai", "workflow automation",
	}
	for _, term := range focusTerms {
		if strings.Contains(raw, term) {
			return true
		}
	}
	return recommendation.Theme == "agents" || recommendation.Theme == "ai" || recommendation.Theme == "enterprise" || recommendation.Theme == "workflow" || recommendation.Theme == "sandbox"
}

func blogOpportunityRank(recommendation ContentRecommendation) int {
	score := recommendation.OpportunityScore
	raw := strings.ToLower(recommendation.SuggestedTitle + " " + recommendation.PrimaryKeyword)
	weights := []struct {
		term   string
		weight int
	}{
		{"enterprise agentic ai", 25},
		{"agentic ai", 22},
		{"ai agent", 20},
		{"sandbox", 18},
		{"production workflow", 16},
		{"enterprise", 12},
		{"workflow", 6},
	}
	for _, item := range weights {
		if strings.Contains(raw, item.term) {
			score += item.weight
		}
	}
	score += min(len(recommendation.SourceEvidence), 3) * 3
	score += min(len(recommendation.SearchSignals), 3) * 5
	if recommendation.Decision == ContentDecisionRefresh {
		score += 10
	}
	return score
}
