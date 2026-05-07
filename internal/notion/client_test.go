package notion

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/nodeops/seo-workflow/internal/competitor"
	"github.com/stretchr/testify/require"
)

func TestCreateCompetitorReportPagePostsNotionPagePayload(t *testing.T) {
	var payloads []map[string]any
	var appendPayload map[string]any
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, "Bearer ntn_test", r.Header.Get("Authorization"))
		require.NotEmpty(t, r.Header.Get("Notion-Version"))
		if r.Method == http.MethodPatch {
			require.Equal(t, "/v1/blocks/page-id/children", r.URL.Path)
			require.NoError(t, json.NewDecoder(r.Body).Decode(&appendPayload))
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(bytes.NewBufferString(`{"object":"list","results":[]}`)),
			}, nil
		}
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/v1/pages", r.URL.Path)
		var payload map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		payloads = append(payloads, payload)
		if len(payloads) == 2 {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(bytes.NewBufferString(`{"id":"draft-page-id","url":"https://notion.so/draft"}`)),
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(bytes.NewBufferString(`{"id":"page-id","url":"https://notion.so/report"}`)),
		}, nil
	})

	client := NewClient("ntn_test", WithBaseURL("https://api.notion.test"), WithHTTPClient(&http.Client{Transport: transport}))
	report := competitor.ReportSummary{
		Title:             "Competitor SEO Report - 2026-05-04 22:15 UTC",
		WindowDays:        30,
		OurRecentURLCount: 23,
		CompetitorCount:   3,
		OpportunityCount:  2,
		TopOpportunities: []competitor.ReportOpportunity{
			{Priority: 1, Topic: "Rapid Prototyping & MVP", Score: 86, Competitor: "replit", Theme: "vibecoding", Why: "Speed-to-market demand.", WhatToDo: "Create a use-case page.", Evidence: []string{"https://replit.com/usecases/rapid-prototyping"}},
		},
		RecommendedContent: []competitor.ReportContentRecommendation{
			{Priority: 1, PageType: "use-case landing page", SuggestedSlug: "/use-cases/rapid-prototyping-mvp", SuggestedTitle: "Rapid Prototyping & MVP with CreateOS", TargetIntent: "persona/use-case evaluation", Pillar: "AI app-building use cases", Draft: &competitor.BlogDraft{Route: "/use-cases/rapid-prototyping-mvp", Title: "Rapid Prototyping & MVP with CreateOS", MetaDescription: "Build MVPs faster.", BodyMarkdown: "# Rapid Prototyping\n\nDraft body.", CTA: "Start building.", Status: "ai-generated-draft"}},
		},
	}

	result, err := client.CreateCompetitorReportPage(context.Background(), "1234567890abcdef1234567890abcdef", report)

	require.NoError(t, err)
	require.Equal(t, "https://notion.so/report", result.URL)
	require.Len(t, payloads, 2)
	parent := payloads[0]["parent"].(map[string]any)
	require.Equal(t, "page_id", parent["type"])
	require.Equal(t, "12345678-90ab-cdef-1234-567890abcdef", parent["page_id"])
	require.NotContains(t, payloads[0], "children")
	children := appendPayload["children"].([]any)
	require.NotEmpty(t, children)
	callout := children[2].(map[string]any)
	require.Equal(t, "callout", callout["type"])
	require.Contains(t, callout, "callout")
	draftParent := payloads[1]["parent"].(map[string]any)
	require.Equal(t, "12345678-90ab-cdef-1234-567890abcdef", draftParent["page_id"])
	draftChildren := payloads[1]["children"].([]any)
	require.Contains(t, childBlockTypes(draftChildren), "heading_2")
	require.Contains(t, childBlockTypes(draftChildren), "paragraph")
	require.NotContains(t, childBlockTypes(draftChildren), "code")
	require.Contains(t, childBlockTypes(children), "link_to_page")
	require.Contains(t, childBlockTexts(children), "Open Draft")
}

func TestReportBlocksUsesScannableNotionLayout(t *testing.T) {
	report := competitor.ReportSummary{
		Title:             "Competitor SEO Report - 2026-05-04 22:15 UTC",
		WindowDays:        30,
		OurRecentURLCount: 23,
		CompetitorCount:   3,
		OpportunityCount:  2,
		SkippedTopicCount: 1,
		TopOpportunities: []competitor.ReportOpportunity{
			{Priority: 1, Topic: "Rapid Prototyping & MVP", Score: 86, Competitor: "replit", Theme: "vibecoding", Why: "Speed-to-market demand.", WhatToDo: "Create a use-case page.", Evidence: []string{"https://replit.com/usecases/rapid-prototyping"}},
		},
		RecommendedContent: []competitor.ReportContentRecommendation{
			{Priority: 1, PageType: "use-case landing page", SuggestedSlug: "/use-cases/rapid-prototyping-mvp", SuggestedTitle: "Rapid Prototyping & MVP with CreateOS", TargetIntent: "persona/use-case evaluation", Pillar: "AI app-building use cases", Draft: &competitor.BlogDraft{Route: "/use-cases/rapid-prototyping-mvp", Title: "Rapid Prototyping & MVP with CreateOS", MetaDescription: "Build MVPs faster.", BodyMarkdown: "# Rapid Prototyping\n\nDraft body.", CTA: "Start building.", Status: "ai-generated-draft"}},
		},
		SkippedTopics: []competitor.ReportSkippedTopic{
			{Competitor: "vercel", Topic: "AI Infrastructure & Gateway", Theme: "ai", Reason: "covered-by-createos", PageCount: 3, EvidenceCount: 3},
		},
	}

	blocks := reportBlocks(report, []draftPageLink{{Title: "Draft: Rapid Prototyping & MVP with CreateOS", PageID: "draft-page-id", URL: "https://notion.so/draft"}})

	require.Equal(t, "heading_2", blocks[0].Type)
	require.Equal(t, "Dashboard", blockText(blocks[0]))
	require.Equal(t, "callout", blocks[2].Type)
	require.Contains(t, blockText(blocks[2]), "Best next action")
	require.Contains(t, blockTexts(blocks), "Top opportunity")
	require.Contains(t, blockTexts(blocks), "Drafts generated")
	require.Contains(t, blockTexts(blocks), "Competitors scanned")
	require.Contains(t, blockTypes(blocks), "table")
	require.Contains(t, blockTypes(blocks), "toggle")
	require.Less(t, firstBlockTextIndex(blocks, "Draft Hub"), firstBlockTextIndex(blocks, "Priority 1: Rapid Prototyping & MVP"))
	require.Less(t, firstBlockTextIndex(blocks, "Next 7 Days"), firstBlockTextIndex(blocks, "Priority 1: Rapid Prototyping & MVP"))
	require.Contains(t, blockTexts(blocks), "Priority 1: Rapid Prototyping & MVP")
	require.Contains(t, blockTexts(blocks), "Action: Create a use-case page.")
	require.Contains(t, blockTexts(blocks), "`/use-cases/rapid-prototyping-mvp`")
	require.Contains(t, blockTexts(blocks), "Draft Hub")
	require.Contains(t, blockTexts(blocks), "Open Draft")
	require.Contains(t, blockTypes(blocks), "link_to_page")
	require.NotContains(t, blockTexts(blocks), "Draft: Rapid Prototyping & MVP with CreateOS")
	require.NotContains(t, blockTypes(blocks), "code")
	require.Contains(t, blockTexts(blocks), "Skipped Topics Worth Reviewing")
}

func TestDraftPageBlocksRenderMarkdownAsNotionBlocks(t *testing.T) {
	blocks := draftPageBlocks(competitor.BlogDraft{
		Route:               "/use-cases/persona-based-ai-use-cases",
		Title:               "AI Workflows Should Match the Person Doing the Work",
		TitleOptions:        []string{"AI Workflows Should Match the Person Doing the Work", "Persona-Based AI Use Cases with CreateOS"},
		SelectedTitleReason: "The selected title creates a clearer reader hook while preserving intent.",
		MetaDescription:     "Create persona-specific AI workflows.",
		BodyMarkdown:        "# Persona-Based AI Use Cases with CreateOS\n\nModern teams need AI workflows that fit how people work.\n\n## Software Engineers\n\nEngineers need precise context.\n\n- Define role-specific prompts\n- Keep implementation consistent",
		InternalLinks: []competitor.SEOLinkSuggestion{
			{AnchorText: "unified execution layer", TargetPath: "/", Placement: "intro", Reason: "Connects to product positioning.", Status: "existing"},
		},
		CTA:    "Start building.",
		Status: "ai-generated-draft",
	})

	types := blockTypes(blocks)
	texts := blockTexts(blocks)
	require.Contains(t, types, "heading_2")
	require.Contains(t, types, "heading_3")
	require.Contains(t, types, "paragraph")
	require.Contains(t, types, "bulleted_list_item")
	require.NotContains(t, types, "code")
	require.Contains(t, texts, "AI Workflows Should Match the Person Doing the Work")
	require.Contains(t, texts, "Title options")
	require.Contains(t, texts, "Persona-Based AI Use Cases with CreateOS")
	require.Contains(t, texts, "Why selected: The selected title creates a clearer reader hook while preserving intent.")
	require.Contains(t, texts, "CreateOS internal link plan")
	require.Contains(t, texts, "CreateOS Internal Links")
	require.Contains(t, texts, "unified execution layer -> `/` | status: existing | placement: intro | reason: Connects to product positioning.")
	require.Contains(t, texts, "Modern teams need AI workflows that fit how people work.")
	require.Contains(t, texts, "Define role-specific prompts")
}

func TestMarkdownToBlocksRendersMarkdownLinksAsNotionLinks(t *testing.T) {
	blocks := markdownToBlocks("Read the [CreateOS blog](https://createos.sh/blogs) for related execution guides.")

	require.Len(t, blocks, 1)
	require.NotNil(t, blocks[0].Paragraph)
	require.Len(t, blocks[0].Paragraph.RichText, 3)
	require.Equal(t, "Read the ", blocks[0].Paragraph.RichText[0].Text.Content)
	require.Equal(t, "CreateOS blog", blocks[0].Paragraph.RichText[1].Text.Content)
	require.NotNil(t, blocks[0].Paragraph.RichText[1].Text.Link)
	require.Equal(t, "https://createos.sh/blogs", blocks[0].Paragraph.RichText[1].Text.Link.URL)
	require.Equal(t, " for related execution guides.", blocks[0].Paragraph.RichText[2].Text.Content)
}

func TestMarkdownToBlocksConvertsRelativeCreateOSLinksToAbsoluteNotionLinks(t *testing.T) {
	blocks := markdownToBlocks("Read the [CreateOS blog](/blogs/context-switching-cost-developer-productivity).")

	require.Len(t, blocks, 1)
	require.NotNil(t, blocks[0].Paragraph)
	require.Len(t, blocks[0].Paragraph.RichText, 3)
	require.Equal(t, "CreateOS blog", blocks[0].Paragraph.RichText[1].Text.Content)
	require.NotNil(t, blocks[0].Paragraph.RichText[1].Text.Link)
	require.Equal(t, "https://createos.sh/blogs/context-switching-cost-developer-productivity", blocks[0].Paragraph.RichText[1].Text.Link.URL)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func blockTexts(blocks []block) string {
	var out []string
	var walk func([]block)
	walk = func(items []block) {
		for _, block := range items {
			out = append(out, blockText(block))
			switch {
			case block.Table != nil:
				walk(block.Table.Children)
			case block.Toggle != nil:
				walk(block.Toggle.Children)
			}
		}
	}
	walk(blocks)
	return strings.Join(out, "\n")
}

func blockText(block block) string {
	var rich []richText
	switch {
	case block.Heading2 != nil:
		rich = block.Heading2.RichText
	case block.Heading3 != nil:
		rich = block.Heading3.RichText
	case block.Paragraph != nil:
		rich = block.Paragraph.RichText
	case block.BulletedListItem != nil:
		rich = block.BulletedListItem.RichText
	case block.NumberedListItem != nil:
		rich = block.NumberedListItem.RichText
	case block.Callout != nil:
		rich = block.Callout.RichText
	case block.Toggle != nil:
		rich = block.Toggle.RichText
	case block.Quote != nil:
		rich = block.Quote.RichText
	case block.TableRow != nil:
		for _, cell := range block.TableRow.Cells {
			for _, text := range cell {
				rich = append(rich, text)
			}
		}
	case block.Code != nil:
		rich = block.Code.RichText
	case block.LinkToPage != nil:
		rich = []richText{textRichText(block.LinkToPage.PageID)}
	}
	var parts []string
	for _, text := range rich {
		parts = append(parts, text.Text.Content)
	}
	return strings.Join(parts, "")
}

func blockTypes(blocks []block) string {
	var types []string
	var walk func([]block)
	walk = func(items []block) {
		for _, item := range items {
			types = append(types, item.Type)
			switch {
			case item.Table != nil:
				walk(item.Table.Children)
			case item.Toggle != nil:
				walk(item.Toggle.Children)
			}
		}
	}
	walk(blocks)
	return strings.Join(types, "\n")
}

func firstBlockTextIndex(blocks []block, content string) int {
	for idx, block := range blocks {
		if strings.Contains(blockText(block), content) {
			return idx
		}
	}
	return -1
}

func childBlockTypes(children []any) string {
	var types []string
	for _, child := range children {
		block := child.(map[string]any)
		types = append(types, block["type"].(string))
	}
	return strings.Join(types, "\n")
}

func childBlockTexts(children []any) string {
	var texts []string
	var walkRichText func([]any)
	walkRichText = func(items []any) {
		for _, item := range items {
			rich := item.(map[string]any)
			text, ok := rich["text"].(map[string]any)
			if !ok {
				continue
			}
			content, _ := text["content"].(string)
			texts = append(texts, content)
		}
	}
	for _, child := range children {
		block := child.(map[string]any)
		switch block["type"] {
		case "callout":
			body := block["callout"].(map[string]any)
			walkRichText(body["rich_text"].([]any))
		case "paragraph":
			body := block["paragraph"].(map[string]any)
			walkRichText(body["rich_text"].([]any))
		case "table":
			body := block["table"].(map[string]any)
			for _, row := range body["children"].([]any) {
				rowBody := row.(map[string]any)["table_row"].(map[string]any)
				for _, cell := range rowBody["cells"].([]any) {
					walkRichText(cell.([]any))
				}
			}
		}
	}
	return strings.Join(texts, "\n")
}
