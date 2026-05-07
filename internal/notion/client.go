package notion

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/nodeops/seo-workflow/internal/competitor"
)

const defaultBaseURL = "https://api.notion.com"
const notionVersion = "2026-03-11"

type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

type Option func(*Client)

type PageResult struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

func NewClient(apiKey string, opts ...Option) *Client {
	client := &Client{
		apiKey:     strings.TrimSpace(apiKey),
		baseURL:    defaultBaseURL,
		httpClient: http.DefaultClient,
	}
	for _, opt := range opts {
		opt(client)
	}
	return client
}

func WithBaseURL(baseURL string) Option {
	return func(client *Client) {
		if strings.TrimSpace(baseURL) != "" {
			client.baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
		}
	}
}

func WithHTTPClient(httpClient *http.Client) Option {
	return func(client *Client) {
		if httpClient != nil {
			client.httpClient = httpClient
		}
	}
}

func (client *Client) CreateCompetitorReportPage(ctx context.Context, parentPageID string, report competitor.ReportSummary) (PageResult, error) {
	if client == nil {
		return PageResult{}, fmt.Errorf("notion client is nil")
	}
	result, err := client.createPage(ctx, parentPageID, report.Title, nil)
	if err != nil {
		return PageResult{}, err
	}
	draftPages, err := client.createDraftPages(ctx, parentPageID, report.RecommendedContent)
	if err != nil {
		return PageResult{}, err
	}
	if err := client.appendBlocks(ctx, result.ID, reportBlocks(report, draftPages)); err != nil {
		return PageResult{}, err
	}
	return result, nil
}

func (client *Client) createPage(ctx context.Context, parentPageID string, title string, children []block) (PageResult, error) {
	if client.apiKey == "" {
		return PageResult{}, fmt.Errorf("notion api key is empty")
	}
	normalizedParentID := normalizePageID(parentPageID)
	if normalizedParentID == "" {
		return PageResult{}, fmt.Errorf("notion parent page id is empty")
	}

	payload := createPagePayload{
		Parent: pageParent{
			Type:   "page_id",
			PageID: normalizedParentID,
		},
		Properties: pageProperties{
			Title: titleProperty{
				Title: []richText{textRichText(title)},
			},
		},
		Children: children,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return PageResult{}, fmt.Errorf("marshal notion page payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, client.baseURL+"/v1/pages", bytes.NewReader(body))
	if err != nil {
		return PageResult{}, fmt.Errorf("build notion request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+client.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Notion-Version", notionVersion)

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return PageResult{}, fmt.Errorf("create notion page: %w", err)
	}
	defer resp.Body.Close()

	respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if readErr != nil {
		return PageResult{}, fmt.Errorf("read notion response: %w", readErr)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return PageResult{}, fmt.Errorf("notion create page failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var result PageResult
	if err := json.Unmarshal(respBody, &result); err != nil {
		return PageResult{}, fmt.Errorf("parse notion response: %w", err)
	}
	return result, nil
}

func (client *Client) appendBlocks(ctx context.Context, parentBlockID string, children []block) error {
	if len(children) == 0 {
		return nil
	}
	payload := appendBlocksPayload{Children: children}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal notion append blocks payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, client.baseURL+"/v1/blocks/"+normalizePageID(parentBlockID)+"/children", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build notion append blocks request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+client.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Notion-Version", notionVersion)

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("append notion blocks: %w", err)
	}
	defer resp.Body.Close()

	respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if readErr != nil {
		return fmt.Errorf("read notion append blocks response: %w", readErr)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("notion append blocks failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}

type draftPageLink struct {
	Title  string
	PageID string
	URL    string
}

func (client *Client) createDraftPages(ctx context.Context, parentPageID string, recommendations []competitor.ReportContentRecommendation) ([]draftPageLink, error) {
	links := make([]draftPageLink, 0, len(recommendations))
	for _, recommendation := range recommendations {
		if recommendation.Draft == nil {
			continue
		}
		title := strings.TrimSpace(recommendation.Draft.Title)
		if title == "" {
			title = strings.TrimSpace(recommendation.SuggestedTitle)
		}
		if title == "" {
			title = fmt.Sprintf("Draft: Priority %d", recommendation.Priority)
		}
		pageTitle := "Draft: " + title
		page, err := client.createPage(ctx, parentPageID, pageTitle, draftPageBlocks(*recommendation.Draft))
		if err != nil {
			return nil, fmt.Errorf("create notion draft page %q: %w", title, err)
		}
		links = append(links, draftPageLink{Title: pageTitle, PageID: page.ID, URL: page.URL})
	}
	return links, nil
}

type createPagePayload struct {
	Parent     pageParent     `json:"parent"`
	Properties pageProperties `json:"properties"`
	Children   []block        `json:"children,omitempty"`
}

type appendBlocksPayload struct {
	Children []block `json:"children"`
}

type pageParent struct {
	Type   string `json:"type"`
	PageID string `json:"page_id"`
}

type pageProperties struct {
	Title titleProperty `json:"title"`
}

type titleProperty struct {
	Title []richText `json:"title"`
}

type richText struct {
	Type        string       `json:"type"`
	Text        richTextText `json:"text"`
	Annotations *annotations `json:"annotations,omitempty"`
}

type richTextText struct {
	Content string `json:"content"`
	Link    *link  `json:"link,omitempty"`
}

type link struct {
	URL string `json:"url"`
}

type annotations struct {
	Bold  bool   `json:"bold,omitempty"`
	Code  bool   `json:"code,omitempty"`
	Color string `json:"color,omitempty"`
}

type block struct {
	Object           string           `json:"object"`
	Type             string           `json:"type"`
	Heading2         *richTextBlock   `json:"heading_2,omitempty"`
	Heading3         *richTextBlock   `json:"heading_3,omitempty"`
	Paragraph        *richTextBlock   `json:"paragraph,omitempty"`
	BulletedListItem *richTextBlock   `json:"bulleted_list_item,omitempty"`
	NumberedListItem *richTextBlock   `json:"numbered_list_item,omitempty"`
	Divider          *struct{}        `json:"divider,omitempty"`
	Callout          *calloutBlock    `json:"callout,omitempty"`
	Quote            *richTextBlock   `json:"quote,omitempty"`
	Toggle           *toggleBlock     `json:"toggle,omitempty"`
	Table            *tableBlock      `json:"table,omitempty"`
	TableRow         *tableRowBlock   `json:"table_row,omitempty"`
	Code             *codeBlock       `json:"code,omitempty"`
	LinkToPage       *linkToPageBlock `json:"link_to_page,omitempty"`
}

type richTextBlock struct {
	RichText []richText `json:"rich_text"`
}

type calloutBlock struct {
	RichText []richText  `json:"rich_text"`
	Icon     calloutIcon `json:"icon"`
	Color    string      `json:"color,omitempty"`
}

type calloutIcon struct {
	Type  string `json:"type"`
	Emoji string `json:"emoji"`
}

type toggleBlock struct {
	RichText []richText `json:"rich_text"`
	Color    string     `json:"color,omitempty"`
	Children []block    `json:"children,omitempty"`
}

type tableBlock struct {
	TableWidth      int     `json:"table_width"`
	HasColumnHeader bool    `json:"has_column_header"`
	HasRowHeader    bool    `json:"has_row_header"`
	Children        []block `json:"children,omitempty"`
}

type tableRowBlock struct {
	Cells [][]richText `json:"cells"`
}

type codeBlock struct {
	RichText []richText `json:"rich_text"`
	Language string     `json:"language"`
}

type linkToPageBlock struct {
	Type   string `json:"type"`
	PageID string `json:"page_id"`
}

func reportBlocks(report competitor.ReportSummary, draftPages []draftPageLink) []block {
	blocks := dashboardBlocks(report, draftPages)
	if len(report.RecommendedContent) > 0 {
		blocks = append(blocks, divider(), heading2("Recommended Content Plan"), contentPlanTable(report.RecommendedContent))
		if hasDrafts(report.RecommendedContent) {
			blocks = append(blocks,
				heading2("Draft Hub"),
				paragraph("Open each generated draft page to review route, metadata, source angle, and rendered blog content."),
				draftHubTable(report.RecommendedContent, draftPages),
			)
			for _, draftPage := range draftPages {
				if strings.TrimSpace(draftPage.PageID) == "" {
					continue
				}
				blocks = append(blocks, linkToPage(draftPage.PageID))
			}
		}
		blocks = append(blocks, executionPlanBlocks(report)...)
	}
	for _, opportunity := range report.TopOpportunities {
		blocks = append(blocks,
			divider(),
			heading2(fmt.Sprintf("Priority %d: %s", opportunity.Priority, opportunity.Topic)),
			opportunityScoreCallout(opportunity),
			quote("Why it matters: "+opportunity.Why),
			bulletedListItem("Action: "+opportunity.WhatToDo),
		)
		if len(opportunity.Evidence) > 0 {
			evidenceBlocks := make([]block, 0, len(opportunity.Evidence))
			for _, evidenceURL := range opportunity.Evidence {
				evidenceBlocks = append(evidenceBlocks, bulletedListItem(evidenceURL))
			}
			blocks = append(blocks, paragraph(fmt.Sprintf("Evidence: %d competitor URLs support this topic.", len(opportunity.Evidence))), toggle("Evidence URLs", evidenceBlocks))
		}
	}

	if len(report.SkippedTopics) > 0 {
		blocks = append(blocks, divider(), heading2("Skipped Topics Worth Reviewing"))
		skippedBlocks := make([]block, 0, len(report.SkippedTopics))
		for _, skipped := range report.SkippedTopics {
			skippedBlocks = append(skippedBlocks, bulletedListItem(fmt.Sprintf(
				"%s / %s: %s (%s, pages=%d, evidence=%d)",
				skipped.Competitor,
				skipped.Topic,
				skipped.Reason,
				skipped.Theme,
				skipped.PageCount,
				skipped.EvidenceCount,
			)))
		}
		blocks = append(blocks, toggle("Open skipped-topic diagnostics", skippedBlocks))
	}

	if len(report.Warnings) > 0 {
		blocks = append(blocks, divider(), heading2("Warnings"))
		for _, warning := range report.Warnings {
			blocks = append(blocks, bulletedListItem(warning))
		}
	}

	return limitBlocks(blocks, 90)
}

func dashboardBlocks(report competitor.ReportSummary, draftPages []draftPageLink) []block {
	blocks := []block{
		heading2("Dashboard"),
		paragraph(fmt.Sprintf("Window: %d days | CreateOS recent URLs: %d | Opportunities: %d | Skipped topics: %d | Warnings: %d",
			report.WindowDays,
			report.OurRecentURLCount,
			report.OpportunityCount,
			report.SkippedTopicCount,
			report.WarningCount,
		)),
		bestNextActionCallout(report),
	}
	if len(report.TopOpportunities) > 0 {
		top := report.TopOpportunities[0]
		blocks = append(blocks, calloutWithColor(
			fmt.Sprintf("Top opportunity: %s (%d/100, %s, %s)", top.Topic, top.Score, titleWord(top.Competitor), top.Theme),
			"🏆",
			"green_background",
		))
	}
	blocks = append(blocks,
		calloutWithColor(fmt.Sprintf("Drafts generated: %d ready for review", len(draftPages)), "📝", "blue_background"),
		calloutWithColor(fmt.Sprintf("Competitors scanned: %d", report.CompetitorCount), "🔎", "gray_background"),
	)
	return blocks
}

func executionPlanBlocks(report competitor.ReportSummary) []block {
	if len(report.RecommendedContent) == 0 {
		return nil
	}
	top := report.RecommendedContent[0]
	return []block{
		divider(),
		heading2("Next 7 Days"),
		numberedListItem(fmt.Sprintf("Review the top draft for %q and validate product claims.", top.SuggestedTitle)),
		numberedListItem(fmt.Sprintf("Edit and publish `%s` as the first page.", top.SuggestedSlug)),
		numberedListItem("Add internal links from relevant CreateOS pages to the new page."),
		numberedListItem("Use the remaining draft pages as the next publishing queue."),
	}
}

func hasDrafts(recommendations []competitor.ReportContentRecommendation) bool {
	for _, recommendation := range recommendations {
		if recommendation.Draft != nil {
			return true
		}
	}
	return false
}

func bestNextActionCallout(report competitor.ReportSummary) block {
	if len(report.RecommendedContent) == 0 {
		return callout("Best next action: review the top opportunity and choose one page to publish first.", "💡")
	}
	top := report.RecommendedContent[0]
	return callout(fmt.Sprintf(
		"Best next action: publish %q at `%s` as a %s.",
		top.SuggestedTitle,
		top.SuggestedSlug,
		top.PageType,
	), "💡")
}

func opportunityScoreCallout(opportunity competitor.ReportOpportunity) block {
	color := "green_background"
	if opportunity.Score < 80 {
		color = "yellow_background"
	}
	if opportunity.Score < 75 {
		color = "orange_background"
	}
	return calloutWithColor(
		fmt.Sprintf("Score %d | %s | %s", opportunity.Score, titleWord(opportunity.Competitor), opportunity.Theme),
		"📌",
		color,
	)
}

func contentPlanTable(recommendations []competitor.ReportContentRecommendation) block {
	rows := []block{
		tableRow("Priority", "Slug", "Page type", "Intent", "Pillar"),
	}
	for _, recommendation := range recommendations {
		rows = append(rows, tableRow(
			fmt.Sprintf("%d", recommendation.Priority),
			recommendation.SuggestedSlug,
			recommendation.PageType,
			recommendation.TargetIntent,
			recommendation.Pillar,
		))
	}
	return table(5, rows)
}

func draftHubTable(recommendations []competitor.ReportContentRecommendation, draftPages []draftPageLink) block {
	draftsByTitle := map[string]draftPageLink{}
	for _, draftPage := range draftPages {
		draftsByTitle[strings.TrimPrefix(draftPage.Title, "Draft: ")] = draftPage
	}
	rows := []block{
		tableRow("Priority", "Draft", "Slug", "Type", "Status"),
	}
	for _, recommendation := range recommendations {
		if recommendation.Draft == nil {
			continue
		}
		draftTitle := strings.TrimSpace(recommendation.Draft.Title)
		if draftTitle == "" {
			draftTitle = recommendation.SuggestedTitle
		}
		draftPage := draftsByTitle[draftTitle]
		rows = append(rows, tableRowRich([][]richText{
			{textRichText(fmt.Sprintf("%d", recommendation.Priority))},
			{linkRichText("Open Draft", draftPage.URL)},
			{textRichText(recommendation.SuggestedSlug)},
			{textRichText(recommendation.PageType)},
			{textRichText("Ready for review")},
		}))
	}
	return table(5, rows)
}

func draftPageBlocks(draft competitor.BlogDraft) []block {
	blocks := []block{
		calloutWithColor("AI-generated draft. Review for accuracy, product claims, and brand voice before publishing.", "⚠️", "orange_background"),
		bulletedListItem("Route: `" + draft.Route + "`"),
		bulletedListItem("Title: " + draft.Title),
	}
	if len(draft.TitleOptions) > 0 {
		children := make([]block, 0, len(draft.TitleOptions)+1)
		for _, option := range draft.TitleOptions {
			if strings.TrimSpace(option) == "" {
				continue
			}
			children = append(children, bulletedListItem(option))
		}
		if strings.TrimSpace(draft.SelectedTitleReason) != "" {
			children = append(children, paragraph("Why selected: "+draft.SelectedTitleReason))
		}
		if len(children) > 0 {
			blocks = append(blocks, toggle("Title options", children))
		}
	}
	if strings.TrimSpace(draft.MetaDescription) != "" {
		blocks = append(blocks, bulletedListItem("Meta description: "+draft.MetaDescription))
	}
	if strings.TrimSpace(draft.CTA) != "" {
		blocks = append(blocks, bulletedListItem("CTA: "+draft.CTA))
	}
	if len(draft.InternalLinks) > 0 {
		blocks = append(blocks, seoLinkPlanToggle(draft))
	}
	blocks = append(blocks, divider())
	blocks = append(blocks, markdownToBlocks(draft.BodyMarkdown)...)
	return limitBlocks(blocks, 100)
}

func seoLinkPlanToggle(draft competitor.BlogDraft) block {
	children := make([]block, 0, len(draft.InternalLinks)+1)
	if len(draft.InternalLinks) > 0 {
		children = append(children, heading3("CreateOS Internal Links"))
		for _, link := range draft.InternalLinks {
			children = append(children, bulletedListItem(formatSEOLinkSuggestion(link)))
		}
	}
	return toggle("CreateOS internal link plan", children)
}

func formatSEOLinkSuggestion(link competitor.SEOLinkSuggestion) string {
	parts := []string{link.AnchorText + " -> `" + link.TargetPath + "`"}
	if strings.TrimSpace(link.Status) != "" {
		parts = append(parts, "status: "+link.Status)
	}
	if strings.TrimSpace(link.Placement) != "" {
		parts = append(parts, "placement: "+link.Placement)
	}
	if strings.TrimSpace(link.Reason) != "" {
		parts = append(parts, "reason: "+link.Reason)
	}
	return strings.Join(parts, " | ")
}

func markdownToBlocks(markdown string) []block {
	lines := strings.Split(strings.TrimSpace(markdown), "\n")
	blocks := make([]block, 0, len(lines))
	var paragraphLines []string
	flushParagraph := func() {
		if len(paragraphLines) == 0 {
			return
		}
		blocks = append(blocks, paragraphRich(markdownInlineRichText(strings.Join(paragraphLines, " "))))
		paragraphLines = nil
	}

	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			flushParagraph()
			continue
		}
		switch {
		case strings.HasPrefix(line, "### "):
			flushParagraph()
			blocks = append(blocks, heading3(strings.TrimSpace(strings.TrimPrefix(line, "### "))))
		case strings.HasPrefix(line, "## "):
			flushParagraph()
			blocks = append(blocks, heading3(strings.TrimSpace(strings.TrimPrefix(line, "## "))))
		case strings.HasPrefix(line, "# "):
			flushParagraph()
			blocks = append(blocks, heading2(strings.TrimSpace(strings.TrimPrefix(line, "# "))))
		case strings.HasPrefix(line, "- "):
			flushParagraph()
			blocks = append(blocks, bulletedListItemRich(markdownInlineRichText(strings.TrimSpace(strings.TrimPrefix(line, "- ")))))
		case strings.HasPrefix(line, "* "):
			flushParagraph()
			blocks = append(blocks, bulletedListItemRich(markdownInlineRichText(strings.TrimSpace(strings.TrimPrefix(line, "* ")))))
		default:
			paragraphLines = append(paragraphLines, line)
		}
	}
	flushParagraph()
	return blocks
}

func heading2(content string) block {
	return block{Object: "block", Type: "heading_2", Heading2: &richTextBlock{RichText: []richText{textRichText(content)}}}
}

func heading3(content string) block {
	return block{Object: "block", Type: "heading_3", Heading3: &richTextBlock{RichText: []richText{textRichText(content)}}}
}

func paragraph(content string) block {
	return block{Object: "block", Type: "paragraph", Paragraph: &richTextBlock{RichText: []richText{textRichText(content)}}}
}

func paragraphRich(rich []richText) block {
	return block{Object: "block", Type: "paragraph", Paragraph: &richTextBlock{RichText: rich}}
}

func bulletedListItem(content string) block {
	return block{Object: "block", Type: "bulleted_list_item", BulletedListItem: &richTextBlock{RichText: []richText{textRichText(content)}}}
}

func bulletedListItemRich(rich []richText) block {
	return block{Object: "block", Type: "bulleted_list_item", BulletedListItem: &richTextBlock{RichText: rich}}
}

func numberedListItem(content string) block {
	return block{Object: "block", Type: "numbered_list_item", NumberedListItem: &richTextBlock{RichText: []richText{textRichText(content)}}}
}

func divider() block {
	return block{Object: "block", Type: "divider", Divider: &struct{}{}}
}

func callout(content string, emoji string) block {
	return calloutWithColor(content, emoji, "yellow_background")
}

func calloutWithColor(content string, emoji string, color string) block {
	return block{
		Object: "block",
		Type:   "callout",
		Callout: &calloutBlock{
			RichText: []richText{textRichText(content)},
			Icon:     calloutIcon{Type: "emoji", Emoji: emoji},
			Color:    color,
		},
	}
}

func quote(content string) block {
	return block{Object: "block", Type: "quote", Quote: &richTextBlock{RichText: []richText{textRichText(content)}}}
}

func toggle(content string, children []block) block {
	return block{
		Object: "block",
		Type:   "toggle",
		Toggle: &toggleBlock{
			RichText: []richText{textRichText(content)},
			Color:    "gray_background",
			Children: children,
		},
	}
}

func table(width int, children []block) block {
	return block{
		Object: "block",
		Type:   "table",
		Table: &tableBlock{
			TableWidth:      width,
			HasColumnHeader: true,
			HasRowHeader:    false,
			Children:        children,
		},
	}
}

func tableRow(cells ...string) block {
	out := make([][]richText, 0, len(cells))
	for _, cell := range cells {
		out = append(out, []richText{textRichText(cell)})
	}
	return block{Object: "block", Type: "table_row", TableRow: &tableRowBlock{Cells: out}}
}

func tableRowRich(cells [][]richText) block {
	return block{Object: "block", Type: "table_row", TableRow: &tableRowBlock{Cells: cells}}
}

func code(content string, language string) block {
	return block{
		Object: "block",
		Type:   "code",
		Code: &codeBlock{
			RichText: []richText{textRichText(content)},
			Language: language,
		},
	}
}

func linkToPage(pageID string) block {
	return block{
		Object: "block",
		Type:   "link_to_page",
		LinkToPage: &linkToPageBlock{
			Type:   "page_id",
			PageID: normalizePageID(pageID),
		},
	}
}

func textRichText(content string) richText {
	return richText{Type: "text", Text: richTextText{Content: truncate(content, 1900)}}
}

func linkRichText(content string, url string) richText {
	text := richText{Type: "text", Text: richTextText{Content: truncate(content, 1900)}}
	if strings.TrimSpace(url) != "" {
		text.Text.Link = &link{URL: strings.TrimSpace(url)}
	}
	return text
}

func markdownInlineRichText(content string) []richText {
	if !strings.Contains(content, "](") {
		return []richText{textRichText(content)}
	}
	out := make([]richText, 0, 3)
	remaining := content
	for {
		open := strings.Index(remaining, "[")
		if open == -1 {
			break
		}
		close := strings.Index(remaining[open:], "](")
		if close == -1 {
			break
		}
		close += open
		urlStart := close + len("](")
		urlEndRel := strings.Index(remaining[urlStart:], ")")
		if urlEndRel == -1 {
			break
		}
		urlEnd := urlStart + urlEndRel
		if open > 0 {
			out = append(out, textRichText(remaining[:open]))
		}
		label := remaining[open+1 : close]
		url := remaining[urlStart:urlEnd]
		if strings.TrimSpace(label) == "" || strings.TrimSpace(url) == "" {
			out = append(out, textRichText(remaining[:urlEnd+1]))
		} else {
			out = append(out, linkRichText(label, normalizeMarkdownLinkURL(url)))
		}
		remaining = remaining[urlEnd+1:]
	}
	if remaining != "" {
		out = append(out, textRichText(remaining))
	}
	if len(out) == 0 {
		return []richText{textRichText(content)}
	}
	return out
}

func normalizeMarkdownLinkURL(raw string) string {
	url := strings.TrimSpace(raw)
	if strings.HasPrefix(url, "/") && !strings.HasPrefix(url, "//") {
		return "https://createos.sh" + url
	}
	return url
}

func titleWord(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	return strings.ToUpper(raw[:1]) + strings.ToLower(raw[1:])
}

func truncate(value string, maxLen int) string {
	if len(value) <= maxLen {
		return value
	}
	if maxLen <= 3 {
		return value[:maxLen]
	}
	return value[:maxLen-3] + "..."
}

func limitBlocks(blocks []block, limit int) []block {
	if len(blocks) <= limit {
		return blocks
	}
	return blocks[:limit]
}

func normalizePageID(raw string) string {
	cleaned := strings.TrimSpace(raw)
	cleaned = strings.Trim(cleaned, "/")
	if strings.Contains(cleaned, "?") {
		cleaned = strings.Split(cleaned, "?")[0]
	}
	if strings.Contains(cleaned, "-") {
		parts := strings.Split(cleaned, "-")
		last := parts[len(parts)-1]
		if len(last) == 32 {
			cleaned = last
		}
	}
	cleaned = strings.ReplaceAll(cleaned, "-", "")
	if len(cleaned) != 32 {
		return strings.TrimSpace(raw)
	}
	return fmt.Sprintf("%s-%s-%s-%s-%s", cleaned[:8], cleaned[8:12], cleaned[12:16], cleaned[16:20], cleaned[20:])
}
