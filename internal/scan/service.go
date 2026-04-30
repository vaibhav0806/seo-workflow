package scan

import (
	"context"
	"errors"
	"fmt"

	"github.com/nodeops/seo-workflow/internal/classifier"
	"github.com/nodeops/seo-workflow/internal/gsc"
	"github.com/nodeops/seo-workflow/internal/pr"
)

// DiscoveryClient discovers URLs from sitemap and analytics sources.
type DiscoveryClient interface {
	Discover(ctx context.Context, property string) ([]string, []gsc.URLMetric, error)
}

// Inspector fetches normalized inspection signals for a URL under a property.
type Inspector interface {
	InspectURL(ctx context.Context, property string, url string) (classifier.InspectionSignal, error)
}

// PropertyWaiter blocks until a property-specific rate limit token is available.
type PropertyWaiter interface {
	Wait(ctx context.Context, property string) error
}

// SitemapLoader loads the current sitemap content for a property.
type SitemapLoader interface {
	Load(ctx context.Context, property string) (string, error)
}

// PROpener opens a pull request for a generated plan.
type PROpener interface {
	Open(ctx context.Context, property string, plan pr.PullRequestPlan) (string, error)
}

// Service orchestrates discovery, inspection, and optional remediation PR creation.
type Service struct {
	discovery DiscoveryClient
	inspector Inspector
	waiter    PropertyWaiter
	loader    SitemapLoader
	opener    PROpener
}

// Summary is the result of a property scan run.
type Summary struct {
	TotalURLs      int
	BucketCounts   map[string]int
	PullRequestURL string
	Findings       []Finding
}

// Finding captures the per-URL diagnostic result for reporting.
type Finding struct {
	URL            string
	Bucket         string
	CoverageState  string
	PageFetchState string
	InSitemap      bool
}

// NewService constructs a scan service with required dependencies.
func NewService(
	discovery DiscoveryClient,
	inspector Inspector,
	waiter PropertyWaiter,
	loader SitemapLoader,
	opener PROpener,
) *Service {
	return &Service{
		discovery: discovery,
		inspector: inspector,
		waiter:    waiter,
		loader:    loader,
		opener:    opener,
	}
}

// RunPropertyScan runs the vertical slice for one property.
func (s *Service) RunPropertyScan(ctx context.Context, property string) (Summary, error) {
	if s == nil {
		return Summary{}, errors.New("scan service precondition failed: service is nil")
	}
	if s.discovery == nil {
		return Summary{}, errors.New("scan service precondition failed: discovery client is nil")
	}
	if s.inspector == nil {
		return Summary{}, errors.New("scan service precondition failed: inspector is nil")
	}
	if s.waiter == nil {
		return Summary{}, errors.New("scan service precondition failed: property waiter is nil")
	}
	if s.loader == nil {
		return Summary{}, errors.New("scan service precondition failed: sitemap loader is nil")
	}
	if s.opener == nil {
		return Summary{}, errors.New("scan service precondition failed: pr opener is nil")
	}

	sitemapURLs, analyticsURLs, err := s.discovery.Discover(ctx, property)
	if err != nil {
		return Summary{}, fmt.Errorf("scan service discover failed for property %q: %w", property, err)
	}

	urls := gsc.MergeDiscoveries(sitemapURLs, analyticsURLs)
	summary := Summary{
		TotalURLs:    len(urls),
		BucketCounts: make(map[string]int),
		Findings:     make([]Finding, 0, len(urls)),
	}

	sitemap404URLs := make([]string, 0)
	for _, url := range urls {
		if waitErr := s.waiter.Wait(ctx, property); waitErr != nil {
			return Summary{}, fmt.Errorf(
				"scan service wait failed for property %q url %q: %w",
				property,
				url,
				waitErr,
			)
		}

		signal, inspectErr := s.inspector.InspectURL(ctx, property, url)
		if inspectErr != nil {
			return Summary{}, fmt.Errorf(
				"scan service inspect failed for property %q url %q: %w",
				property,
				url,
				inspectErr,
			)
		}

		bucket := classifier.Classify(signal)
		summary.BucketCounts[string(bucket)]++
		summary.Findings = append(summary.Findings, Finding{
			URL:            url,
			Bucket:         string(bucket),
			CoverageState:  signal.CoverageState,
			PageFetchState: signal.PageFetchState,
			InSitemap:      signal.InSitemap,
		})

		if bucket == classifier.Sitemap404 {
			sitemap404URLs = append(sitemap404URLs, url)
		}
	}

	if len(sitemap404URLs) == 0 {
		return summary, nil
	}

	currentSitemap, err := s.loader.Load(ctx, property)
	if err != nil {
		return Summary{}, fmt.Errorf("scan service sitemap load failed for property %q: %w", property, err)
	}

	plan, err := pr.BuildSitemap404PR(sitemap404URLs, currentSitemap)
	if err != nil {
		return Summary{}, fmt.Errorf("scan service pr plan build failed for property %q: %w", property, err)
	}

	if len(plan.Files) == 0 {
		return summary, nil
	}

	sitemapChanged := false
	for _, file := range plan.Files {
		if file.Path == "public/sitemap.xml" && file.Content != currentSitemap {
			sitemapChanged = true
			break
		}
	}
	if !sitemapChanged {
		return summary, nil
	}

	prURL, err := s.opener.Open(ctx, property, plan)
	if err != nil {
		return Summary{}, fmt.Errorf("scan service pr open failed for property %q: %w", property, err)
	}
	summary.PullRequestURL = prURL

	return summary, nil
}
