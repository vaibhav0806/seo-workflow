package pr

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildSitemap404PR_PrettyPrintedRemoval(t *testing.T) {
	sitemap := `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url>
    <loc>https://example.com/a</loc>
  </url>
  <url>
    <loc>https://example.com/b</loc>
  </url>
</urlset>`

	plan, err := BuildSitemap404PR(
		[]string{"https://example.com/b"},
		sitemap,
	)

	require.NoError(t, err)
	require.Equal(t, "seo/fix-sitemap-404", plan.Branch)
	require.Equal(t, "fix(seo): remove 404 URLs from sitemap", plan.Title)
	require.Len(t, plan.Files, 1)
	require.Equal(t, "public/sitemap.xml", plan.Files[0].Path)
	require.NotContains(t, plan.Files[0].Content, "https://example.com/b")
	require.Contains(t, plan.Files[0].Content, "https://example.com/a")
	require.Contains(t, plan.Files[0].Content, "<urlset")
	require.Contains(t, plan.Body, "Not found (404) by GSC")
	require.Contains(t, plan.Body, "- https://example.com/b")
	require.NotContains(t, plan.Body, "- https://example.com/a")
}

func TestBuildSitemap404PR_MultipleRemovals(t *testing.T) {
	sitemap := `<urlset><url><loc>https://example.com/a</loc></url><url><loc>https://example.com/b</loc></url><url><loc>https://example.com/c</loc></url></urlset>`

	plan, err := BuildSitemap404PR(
		[]string{"https://example.com/a", "https://example.com/c"},
		sitemap,
	)

	require.NoError(t, err)
	require.Len(t, plan.Files, 1)
	require.NotContains(t, plan.Files[0].Content, "https://example.com/a")
	require.NotContains(t, plan.Files[0].Content, "https://example.com/c")
	require.Contains(t, plan.Files[0].Content, "https://example.com/b")
	require.Contains(t, plan.Body, "- https://example.com/a")
	require.Contains(t, plan.Body, "- https://example.com/c")
	require.NotContains(t, plan.Body, "- https://example.com/b")
}

func TestBuildSitemap404PR_URLNotPresent_AccurateBody(t *testing.T) {
	sitemap := `<urlset><url><loc>https://example.com/a</loc></url></urlset>`

	plan, err := BuildSitemap404PR(
		[]string{"https://example.com/missing"},
		sitemap,
	)

	require.NoError(t, err)
	require.Len(t, plan.Files, 1)
	require.Equal(t, sitemap, plan.Files[0].Content)
	require.Contains(t, plan.Files[0].Content, "https://example.com/a")
	require.NotContains(t, plan.Files[0].Content, "https://example.com/missing")
	require.Contains(t, plan.Body, "No matching URLs")
	require.Contains(t, plan.Body, "No URLs were removed")
}

func TestBuildSitemap404PR_EmptyNotFoundList(t *testing.T) {
	sitemap := `<urlset><url><loc>https://example.com/a</loc></url></urlset>`

	plan, err := BuildSitemap404PR(nil, sitemap)

	require.NoError(t, err)
	require.Len(t, plan.Files, 1)
	require.Equal(t, sitemap, plan.Files[0].Content)
	require.Contains(t, plan.Body, "No matching URLs")
	require.Contains(t, plan.Body, "No URLs were removed")
}

func TestBuildSitemap404PR_MalformedSitemapReturnsError(t *testing.T) {
	_, err := BuildSitemap404PR(
		[]string{"https://example.com/a"},
		`<urlset><url><loc>https://example.com/a</loc></urlset>`,
	)

	require.Error(t, err)
	require.ErrorContains(t, err, "parse sitemap xml")
}
