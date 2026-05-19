package contentrepo

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/nodeops/seo-workflow/internal/competitor"
)

const DestinationCreateOS = "createos"

var nonSlugChars = regexp.MustCompile(`[^a-z0-9]+`)

type BlogPost struct {
	Title        string
	Slug         string
	Description  string
	Author       string
	ReadTime     string
	Tags         []string
	Cover        string
	PublishedAt  time.Time
	Destination  string
	BodyMarkdown string
}

func BuildBlogPost(recommendation competitor.ContentRecommendation, generatedAt time.Time, author string, coverURL string) (BlogPost, error) {
	if recommendation.Draft == nil {
		return BlogPost{}, fmt.Errorf("content recommendation has no draft")
	}
	draft := *recommendation.Draft
	title := strings.TrimSpace(draft.Title)
	if title == "" {
		title = strings.TrimSpace(recommendation.SuggestedTitle)
	}
	body := strings.TrimSpace(draft.BodyMarkdown)
	if title == "" || body == "" {
		return BlogPost{}, fmt.Errorf("draft requires title and body")
	}

	slug := slugFromRoute(draft.Route)
	if slug == "" {
		slug = slugify(title)
	}
	if slug == "" {
		return BlogPost{}, fmt.Errorf("draft requires route or slugifiable title")
	}

	description := strings.TrimSpace(draft.MetaDescription)
	if description == "" {
		description = strings.TrimSpace(recommendation.ContentAngle)
	}
	description = trimForMeta(description, 160)
	if description == "" {
		return BlogPost{}, fmt.Errorf("draft requires meta description")
	}

	author = strings.TrimSpace(author)
	if author == "" {
		author = "CreateOS"
	}
	coverURL = strings.TrimSpace(coverURL)
	if coverURL == "" {
		return BlogPost{}, fmt.Errorf("cover URL is required")
	}
	tags := uniqueTags([]string{
		"createos",
		recommendation.Theme,
		recommendation.Pillar,
		recommendation.PageType,
	})

	return BlogPost{
		Title:        title,
		Slug:         slug,
		Description:  description,
		Author:       author,
		ReadTime:     estimatedReadTime(body),
		Tags:         tags,
		Cover:        coverURL,
		PublishedAt:  generatedAt.UTC(),
		Destination:  DestinationCreateOS,
		BodyMarkdown: body,
	}, nil
}

func (post BlogPost) FilePath() string {
	return "blogs/" + post.Slug + ".md"
}

func (post BlogPost) Markdown() string {
	lines := []string{
		"---",
		"title: " + yamlQuote(post.Title),
		"slug: " + post.Slug,
		"description: " + yamlQuote(post.Description),
		"author: " + yamlQuote(post.Author),
		"read_time: " + yamlQuote(post.ReadTime),
		"tags:",
	}
	for _, tag := range post.Tags {
		lines = append(lines, "  - "+yamlQuote(tag))
	}
	lines = append(lines,
		"cover: "+yamlQuote(post.Cover),
		"published_at: "+yamlQuote(post.PublishedAt.UTC().Format("2006-01-02T15:04:05.000Z")),
		"destination: "+post.Destination,
		"---",
		"",
		strings.TrimSpace(post.BodyMarkdown),
		"",
	)
	return strings.Join(lines, "\n")
}

func slugFromRoute(route string) string {
	route = strings.TrimSpace(route)
	if route == "" {
		return ""
	}
	route = strings.Trim(route, "/")
	parts := strings.Split(route, "/")
	if len(parts) == 0 {
		return ""
	}
	return slugify(parts[len(parts)-1])
}

func slugify(input string) string {
	input = strings.ToLower(strings.TrimSpace(input))
	input = nonSlugChars.ReplaceAllString(input, "-")
	input = strings.Trim(input, "-")
	for strings.Contains(input, "--") {
		input = strings.ReplaceAll(input, "--", "-")
	}
	return input
}

func yamlQuote(value string) string {
	escaped := strings.ReplaceAll(value, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return `"` + escaped + `"`
}

func trimForMeta(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	if len(value) <= limit {
		return value
	}
	trimmed := strings.TrimSpace(value[:limit-1])
	lastSpace := strings.LastIndex(trimmed, " ")
	if lastSpace > 80 {
		trimmed = trimmed[:lastSpace]
	}
	return strings.TrimSpace(trimmed) + "..."
}

func estimatedReadTime(markdown string) string {
	words := strings.Fields(markdown)
	minutes := len(words) / 200
	if len(words)%200 != 0 {
		minutes++
	}
	if minutes < 3 {
		minutes = 3
	}
	return fmt.Sprintf("%d min", minutes)
}

func uniqueTags(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	if len(out) == 0 {
		return []string{"createos"}
	}
	return out
}
