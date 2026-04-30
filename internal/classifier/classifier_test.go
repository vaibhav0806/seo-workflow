package classifier

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClassify(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		signal InspectionSignal
		want   Bucket
	}{
		{
			name: "healthy when submitted and indexed",
			signal: InspectionSignal{
				CoverageState: "Submitted and indexed",
			},
			want: Healthy,
		},
		{
			name: "sitemap missing entry when indexed but not in sitemap",
			signal: InspectionSignal{
				CoverageState: "Indexed, not submitted in sitemap",
			},
			want: SitemapMissing,
		},
		{
			name: "sitemap redirect",
			signal: InspectionSignal{
				CoverageState: "Page with redirect",
			},
			want: SitemapRedirect,
		},
		{
			name: "sitemap 404 when page is in sitemap",
			signal: InspectionSignal{
				CoverageState: "Not found (404)",
				InSitemap:     true,
			},
			want: Sitemap404,
		},
		{
			name: "not found without sitemap flag falls back to unknown",
			signal: InspectionSignal{
				CoverageState: "Not found (404)",
				InSitemap:     false,
			},
			want: Unknown,
		},
		{
			name: "canonical mismatch",
			signal: InspectionSignal{
				CoverageState: "Duplicate, Google chose different canonical",
			},
			want: CanonicalMismatch,
		},
		{
			name: "canonical missing",
			signal: InspectionSignal{
				CoverageState: "Duplicate without user-selected canonical",
			},
			want: CanonicalMissing,
		},
		{
			name: "noindex misconfig when page is in sitemap",
			signal: InspectionSignal{
				CoverageState: "Excluded by 'noindex' tag",
				InSitemap:     true,
			},
			want: NoindexMisconfig,
		},
		{
			name: "noindex without sitemap flag falls back to unknown",
			signal: InspectionSignal{
				CoverageState: "Excluded by 'noindex' tag",
				InSitemap:     false,
			},
			want: Unknown,
		},
		{
			name: "robots misconfig when page is in sitemap",
			signal: InspectionSignal{
				CoverageState: "Blocked by robots.txt",
				InSitemap:     true,
			},
			want: RobotsMisconfig,
		},
		{
			name: "robots blocked without sitemap flag falls back to unknown",
			signal: InspectionSignal{
				CoverageState: "Blocked by robots.txt",
				InSitemap:     false,
			},
			want: Unknown,
		},
		{
			name: "quality bucket",
			signal: InspectionSignal{
				CoverageState: "Crawled - currently not indexed",
			},
			want: QualityOrDup,
		},
		{
			name: "crawl budget",
			signal: InspectionSignal{
				CoverageState: "Discovered - currently not indexed",
			},
			want: CrawlBudget,
		},
		{
			name: "soft 404 by coverage state",
			signal: InspectionSignal{
				CoverageState: "Soft 404",
			},
			want: Soft404,
		},
		{
			name: "js rendering by page fetch state soft 404",
			signal: InspectionSignal{
				PageFetchState: "SOFT_404",
			},
			want: JSRendering,
		},
		{
			name: "server error by page fetch state",
			signal: InspectionSignal{
				PageFetchState: "SERVER_ERROR",
			},
			want: ServerError,
		},
		{
			name: "bot blocked by page fetch state",
			signal: InspectionSignal{
				PageFetchState: "ACCESS_FORBIDDEN",
			},
			want: BotBlocked,
		},
		{
			name: "unknown fallback",
			signal: InspectionSignal{
				CoverageState: "Unmapped state",
			},
			want: Unknown,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := Classify(tt.signal)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestClassifyPrecedence(t *testing.T) {
	t.Parallel()

	signal := InspectionSignal{
		CoverageState:  "Soft 404",
		PageFetchState: "SERVER_ERROR",
	}

	got := Classify(signal)
	require.Equal(t, Soft404, got)
}
