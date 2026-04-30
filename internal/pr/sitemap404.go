package pr

import (
	"encoding/xml"
	"fmt"
	"strings"
)

type FileEdit struct {
	Path    string
	Content string
}

type PullRequestPlan struct {
	Branch string
	Title  string
	Body   string
	Files  []FileEdit
}

type sitemapURLSet struct {
	XMLName xml.Name     `xml:"urlset"`
	Attrs   []xml.Attr   `xml:",any,attr"`
	URLs    []sitemapURL `xml:"url"`
}

type sitemapURL struct {
	Loc string `xml:"loc"`
}

func BuildSitemap404PR(notFoundURLs []string, currentSitemap string) (PullRequestPlan, error) {
	targets := make(map[string]struct{}, len(notFoundURLs))
	for _, url := range notFoundURLs {
		trimmed := strings.TrimSpace(url)
		if trimmed == "" {
			continue
		}
		targets[trimmed] = struct{}{}
	}

	var sitemap sitemapURLSet
	if err := xml.Unmarshal([]byte(currentSitemap), &sitemap); err != nil {
		return PullRequestPlan{}, fmt.Errorf("parse sitemap xml: %w", err)
	}

	next := currentSitemap
	removed := []string{}

	if len(targets) > 0 {
		kept := make([]sitemapURL, 0, len(sitemap.URLs))
		removedSeen := make(map[string]struct{})

		for _, entry := range sitemap.URLs {
			loc := strings.TrimSpace(entry.Loc)
			if _, shouldRemove := targets[loc]; shouldRemove {
				if _, alreadySeen := removedSeen[loc]; !alreadySeen {
					removed = append(removed, loc)
					removedSeen[loc] = struct{}{}
				}
				continue
			}
			kept = append(kept, entry)
		}

		if len(removed) > 0 {
			sitemap.URLs = kept
			encoded, err := xml.MarshalIndent(sitemap, "", "  ")
			if err != nil {
				return PullRequestPlan{}, fmt.Errorf("encode sitemap xml: %w", err)
			}
			next = string(encoded)
		}
	}

	body := buildSitemap404Body(removed)

	return PullRequestPlan{
		Branch: "seo/fix-sitemap-404",
		Title:  "fix(seo): remove 404 URLs from sitemap",
		Body:   body,
		Files: []FileEdit{
			{
				Path:    "public/sitemap.xml",
				Content: next,
			},
		},
	}, nil
}

func buildSitemap404Body(removed []string) string {
	if len(removed) == 0 {
		return "No matching URLs reported as Not found (404) by GSC were found in sitemap.xml. No URLs were removed."
	}

	lines := []string{
		"These URLs were reported as Not found (404) by GSC and were removed from sitemap.xml:",
	}
	for _, url := range removed {
		lines = append(lines, "- "+url)
	}
	return strings.Join(lines, "\n")
}
