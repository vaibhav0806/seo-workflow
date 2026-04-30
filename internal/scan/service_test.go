package scan

import (
	"context"
	"errors"
	"testing"

	"github.com/nodeops/seo-workflow/internal/classifier"
	"github.com/nodeops/seo-workflow/internal/gsc"
	"github.com/nodeops/seo-workflow/internal/pr"
	"github.com/stretchr/testify/require"
)

type fakeDiscoveryClient struct {
	sitemap   []string
	analytics []gsc.URLMetric
	err       error
	calls     int
	seenProps []string
}

func (f *fakeDiscoveryClient) Discover(_ context.Context, property string) ([]string, []gsc.URLMetric, error) {
	f.calls++
	f.seenProps = append(f.seenProps, property)
	if f.err != nil {
		return nil, nil, f.err
	}

	return f.sitemap, f.analytics, nil
}

type fakeInspector struct {
	signal    classifier.InspectionSignal
	err       error
	calls     int
	seenProps []string
	seenURLs  []string
}

func (f *fakeInspector) InspectURL(_ context.Context, property string, url string) (classifier.InspectionSignal, error) {
	f.calls++
	f.seenProps = append(f.seenProps, property)
	f.seenURLs = append(f.seenURLs, url)
	if f.err != nil {
		return classifier.InspectionSignal{}, f.err
	}

	return f.signal, nil
}

type fakePropertyWaiter struct {
	err       error
	calls     int
	seenProps []string
}

func (f *fakePropertyWaiter) Wait(_ context.Context, property string) error {
	f.calls++
	f.seenProps = append(f.seenProps, property)
	if f.err != nil {
		return f.err
	}

	return nil
}

type fakeSitemapLoader struct {
	content   string
	err       error
	calls     int
	seenProps []string
}

func (f *fakeSitemapLoader) Load(_ context.Context, property string) (string, error) {
	f.calls++
	f.seenProps = append(f.seenProps, property)
	if f.err != nil {
		return "", f.err
	}

	return f.content, nil
}

type fakePROpener struct {
	url       string
	err       error
	calls     int
	seenPlan  pr.PullRequestPlan
	seenProps []string
}

func (f *fakePROpener) Open(_ context.Context, property string, plan pr.PullRequestPlan) (string, error) {
	f.calls++
	f.seenPlan = plan
	f.seenProps = append(f.seenProps, property)
	if f.err != nil {
		return "", f.err
	}

	return f.url, nil
}

func TestRunPropertyScan(t *testing.T) {
	t.Parallel()

	t.Run("returns summary and pr url when sitemap 404 findings exist", func(t *testing.T) {
		t.Parallel()

		property := "sc-domain:example.com"
		discovery := &fakeDiscoveryClient{
			sitemap: []string{"https://example.com/a", "https://example.com/b"},
			analytics: []gsc.URLMetric{
				{URL: "https://example.com/b", Impressions: 10},
			},
		}
		inspector := &fakeInspector{
			signal: classifier.InspectionSignal{
				CoverageState: "Not found (404)",
				InSitemap:     true,
			},
		}
		waiter := &fakePropertyWaiter{}
		loader := &fakeSitemapLoader{
			content: `<urlset><url><loc>https://example.com/a</loc></url><url><loc>https://example.com/b</loc></url></urlset>`,
		}
		opener := &fakePROpener{
			url: "https://github.com/nodeops/seo-workflow/pull/123",
		}

		service := NewService(discovery, inspector, waiter, loader, opener)
		summary, err := service.RunPropertyScan(context.Background(), property)

		require.NoError(t, err)
		require.Equal(t, 2, summary.TotalURLs)
		require.Equal(t, 2, summary.BucketCounts["sitemap_404"])
		require.Equal(t, "https://github.com/nodeops/seo-workflow/pull/123", summary.PullRequestURL)
		require.Len(t, summary.Findings, 2)
		require.Equal(t, "https://example.com/b", summary.Findings[0].URL)
		require.Equal(t, "sitemap_404", summary.Findings[0].Bucket)
		require.Equal(t, 1, discovery.calls)
		require.Equal(t, 2, waiter.calls)
		require.Equal(t, 2, inspector.calls)
		require.Equal(t, 1, loader.calls)
		require.Equal(t, 1, opener.calls)
		require.Contains(t, opener.seenPlan.Body, "Not found (404)")
		require.Equal(t, []string{property}, discovery.seenProps)
		require.Equal(t, []string{property, property}, waiter.seenProps)
		require.Equal(t, []string{property, property}, inspector.seenProps)
		require.Equal(t, []string{"https://example.com/b", "https://example.com/a"}, inspector.seenURLs)
		require.Equal(t, []string{property}, loader.seenProps)
		require.Equal(t, []string{property}, opener.seenProps)
	})

	t.Run("returns error when waiter fails", func(t *testing.T) {
		t.Parallel()

		expectedErr := errors.New("wait failed")
		discovery := &fakeDiscoveryClient{
			sitemap: []string{"https://example.com/a", "https://example.com/b"},
			analytics: []gsc.URLMetric{
				{URL: "https://example.com/b", Impressions: 10},
			},
		}
		inspector := &fakeInspector{}
		waiter := &fakePropertyWaiter{err: expectedErr}
		loader := &fakeSitemapLoader{}
		opener := &fakePROpener{}

		service := NewService(discovery, inspector, waiter, loader, opener)
		summary, err := service.RunPropertyScan(context.Background(), "sc-domain:example.com")

		require.ErrorIs(t, err, expectedErr)
		require.ErrorContains(t, err, "wait failed")
		require.Equal(t, Summary{}, summary)
		require.Equal(t, 1, discovery.calls)
		require.Equal(t, 1, waiter.calls)
		require.Equal(t, 0, inspector.calls)
		require.Equal(t, 0, loader.calls)
		require.Equal(t, 0, opener.calls)
	})

	t.Run("returns error when pr opener fails", func(t *testing.T) {
		t.Parallel()

		expectedErr := errors.New("open pr failed")

		discovery := &fakeDiscoveryClient{
			sitemap: []string{"https://example.com/a", "https://example.com/b"},
			analytics: []gsc.URLMetric{
				{URL: "https://example.com/b", Impressions: 10},
			},
		}
		inspector := &fakeInspector{
			signal: classifier.InspectionSignal{
				CoverageState: "Not found (404)",
				InSitemap:     true,
			},
		}
		waiter := &fakePropertyWaiter{}
		loader := &fakeSitemapLoader{
			content: `<urlset><url><loc>https://example.com/a</loc></url><url><loc>https://example.com/b</loc></url></urlset>`,
		}
		opener := &fakePROpener{err: expectedErr}

		service := NewService(discovery, inspector, waiter, loader, opener)
		summary, err := service.RunPropertyScan(context.Background(), "sc-domain:example.com")

		require.ErrorIs(t, err, expectedErr)
		require.ErrorContains(t, err, "pr open failed")
		require.Equal(t, Summary{}, summary)
		require.Equal(t, 1, discovery.calls)
		require.Equal(t, 2, waiter.calls)
		require.Equal(t, 2, inspector.calls)
		require.Equal(t, 1, loader.calls)
		require.Equal(t, 1, opener.calls)
	})

	t.Run("returns error when discovery fails", func(t *testing.T) {
		t.Parallel()

		expectedErr := errors.New("discover failed")
		discovery := &fakeDiscoveryClient{err: expectedErr}
		inspector := &fakeInspector{}
		waiter := &fakePropertyWaiter{}
		loader := &fakeSitemapLoader{}
		opener := &fakePROpener{}

		service := NewService(discovery, inspector, waiter, loader, opener)
		summary, err := service.RunPropertyScan(context.Background(), "sc-domain:example.com")

		require.ErrorIs(t, err, expectedErr)
		require.ErrorContains(t, err, "discover failed")
		require.Equal(t, Summary{}, summary)
		require.Equal(t, 1, discovery.calls)
		require.Equal(t, 0, waiter.calls)
		require.Equal(t, 0, inspector.calls)
		require.Equal(t, 0, loader.calls)
		require.Equal(t, 0, opener.calls)
	})

	t.Run("returns error when inspector fails", func(t *testing.T) {
		t.Parallel()

		expectedErr := errors.New("inspect failed")
		discovery := &fakeDiscoveryClient{
			sitemap: []string{"https://example.com/a", "https://example.com/b"},
			analytics: []gsc.URLMetric{
				{URL: "https://example.com/b", Impressions: 10},
			},
		}
		inspector := &fakeInspector{err: expectedErr}
		waiter := &fakePropertyWaiter{}
		loader := &fakeSitemapLoader{}
		opener := &fakePROpener{}

		service := NewService(discovery, inspector, waiter, loader, opener)
		summary, err := service.RunPropertyScan(context.Background(), "sc-domain:example.com")

		require.ErrorIs(t, err, expectedErr)
		require.ErrorContains(t, err, "inspect failed")
		require.Equal(t, Summary{}, summary)
		require.Equal(t, 1, discovery.calls)
		require.Equal(t, 1, waiter.calls)
		require.Equal(t, 1, inspector.calls)
		require.Equal(t, 0, loader.calls)
		require.Equal(t, 0, opener.calls)
	})

	t.Run("returns error when sitemap loader fails for sitemap 404 flow", func(t *testing.T) {
		t.Parallel()

		expectedErr := errors.New("load sitemap failed")
		discovery := &fakeDiscoveryClient{
			sitemap: []string{"https://example.com/a", "https://example.com/b"},
			analytics: []gsc.URLMetric{
				{URL: "https://example.com/b", Impressions: 10},
			},
		}
		inspector := &fakeInspector{
			signal: classifier.InspectionSignal{
				CoverageState: "Not found (404)",
				InSitemap:     true,
			},
		}
		waiter := &fakePropertyWaiter{}
		loader := &fakeSitemapLoader{err: expectedErr}
		opener := &fakePROpener{}

		service := NewService(discovery, inspector, waiter, loader, opener)
		summary, err := service.RunPropertyScan(context.Background(), "sc-domain:example.com")

		require.ErrorIs(t, err, expectedErr)
		require.ErrorContains(t, err, "sitemap load failed")
		require.Equal(t, Summary{}, summary)
		require.Equal(t, 1, discovery.calls)
		require.Equal(t, 2, waiter.calls)
		require.Equal(t, 2, inspector.calls)
		require.Equal(t, 1, loader.calls)
		require.Equal(t, 0, opener.calls)
	})

	t.Run("returns error when sitemap is malformed for sitemap 404 flow", func(t *testing.T) {
		t.Parallel()

		discovery := &fakeDiscoveryClient{
			sitemap: []string{"https://example.com/a"},
		}
		inspector := &fakeInspector{
			signal: classifier.InspectionSignal{
				CoverageState: "Not found (404)",
				InSitemap:     true,
			},
		}
		waiter := &fakePropertyWaiter{}
		loader := &fakeSitemapLoader{content: `<urlset><url><loc>https://example.com/a</loc></urlset>`}
		opener := &fakePROpener{}

		service := NewService(discovery, inspector, waiter, loader, opener)
		summary, err := service.RunPropertyScan(context.Background(), "sc-domain:example.com")

		require.Error(t, err)
		require.ErrorContains(t, err, "pr plan build failed")
		require.Equal(t, Summary{}, summary)
		require.Equal(t, 1, discovery.calls)
		require.Equal(t, 1, waiter.calls)
		require.Equal(t, 1, inspector.calls)
		require.Equal(t, 1, loader.calls)
		require.Equal(t, 0, opener.calls)
	})

	t.Run("no sitemap 404 does not call loader or opener", func(t *testing.T) {
		t.Parallel()

		discovery := &fakeDiscoveryClient{
			sitemap: []string{"https://example.com/a", "https://example.com/b"},
			analytics: []gsc.URLMetric{
				{URL: "https://example.com/b", Impressions: 10},
			},
		}
		inspector := &fakeInspector{
			signal: classifier.InspectionSignal{
				CoverageState: "Submitted and indexed",
				InSitemap:     true,
			},
		}
		waiter := &fakePropertyWaiter{}
		loader := &fakeSitemapLoader{}
		opener := &fakePROpener{}

		service := NewService(discovery, inspector, waiter, loader, opener)
		summary, err := service.RunPropertyScan(context.Background(), "sc-domain:example.com")

		require.NoError(t, err)
		require.Equal(t, 2, summary.TotalURLs)
		require.Equal(t, 2, summary.BucketCounts["healthy"])
		require.Empty(t, summary.PullRequestURL)
		require.Equal(t, 1, discovery.calls)
		require.Equal(t, 2, waiter.calls)
		require.Equal(t, 2, inspector.calls)
		require.Equal(t, 0, loader.calls)
		require.Equal(t, 0, opener.calls)
	})

	t.Run("no-op sitemap plan does not call opener", func(t *testing.T) {
		t.Parallel()

		discovery := &fakeDiscoveryClient{
			sitemap: []string{"https://example.com/a", "https://example.com/b"},
			analytics: []gsc.URLMetric{
				{URL: "https://example.com/b", Impressions: 10},
			},
		}
		inspector := &fakeInspector{
			signal: classifier.InspectionSignal{
				CoverageState: "Not found (404)",
				InSitemap:     true,
			},
		}
		waiter := &fakePropertyWaiter{}
		loader := &fakeSitemapLoader{
			content: `<urlset><url><loc>https://example.com/c</loc></url></urlset>`,
		}
		opener := &fakePROpener{}

		service := NewService(discovery, inspector, waiter, loader, opener)
		summary, err := service.RunPropertyScan(context.Background(), "sc-domain:example.com")

		require.NoError(t, err)
		require.Equal(t, 2, summary.TotalURLs)
		require.Equal(t, 2, summary.BucketCounts["sitemap_404"])
		require.Empty(t, summary.PullRequestURL)
		require.Equal(t, 2, waiter.calls)
		require.Equal(t, 1, loader.calls)
		require.Equal(t, 0, opener.calls)
	})

	t.Run("returns precondition error for nil dependency", func(t *testing.T) {
		t.Parallel()

		service := NewService(nil, &fakeInspector{}, &fakePropertyWaiter{}, &fakeSitemapLoader{}, &fakePROpener{})
		summary, err := service.RunPropertyScan(context.Background(), "sc-domain:example.com")

		require.Error(t, err)
		require.ErrorContains(t, err, "discovery client is nil")
		require.Equal(t, Summary{}, summary)
	})

	t.Run("returns precondition error for nil waiter", func(t *testing.T) {
		t.Parallel()

		service := NewService(&fakeDiscoveryClient{}, &fakeInspector{}, nil, &fakeSitemapLoader{}, &fakePROpener{})
		summary, err := service.RunPropertyScan(context.Background(), "sc-domain:example.com")

		require.Error(t, err)
		require.ErrorContains(t, err, "property waiter is nil")
		require.Equal(t, Summary{}, summary)
	})
}
