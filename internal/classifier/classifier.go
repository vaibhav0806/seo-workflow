package classifier

// Bucket represents a deterministic issue classification bucket.
type Bucket string

const (
	Healthy           Bucket = "healthy"
	SitemapMissing    Bucket = "sitemap_missing_entry"
	SitemapRedirect   Bucket = "sitemap_redirect"
	Sitemap404        Bucket = "sitemap_404"
	CanonicalMismatch Bucket = "canonical_mismatch"
	CanonicalMissing  Bucket = "canonical_missing"
	NoindexMisconfig  Bucket = "noindex_misconfig"
	RobotsMisconfig   Bucket = "robots_misconfig"
	QualityOrDup      Bucket = "quality_or_dup"
	CrawlBudget       Bucket = "crawl_budget"
	Soft404           Bucket = "soft_404"
	JSRendering       Bucket = "js_rendering"
	ServerError       Bucket = "server_error"
	BotBlocked        Bucket = "bot_blocked"
	Unknown           Bucket = "unknown"
)

// InspectionSignal contains normalized signals used for deterministic bucketing.
type InspectionSignal struct {
	CoverageState  string
	PageFetchState string
	InSitemap      bool
}

// Classify maps inspection signals into a deterministic bucket.
func Classify(signal InspectionSignal) Bucket {
	switch {
	case signal.CoverageState == "Submitted and indexed":
		return Healthy
	case signal.CoverageState == "Indexed, not submitted in sitemap":
		return SitemapMissing
	case signal.CoverageState == "Page with redirect":
		return SitemapRedirect
	case signal.CoverageState == "Not found (404)" && signal.InSitemap:
		return Sitemap404
	case signal.CoverageState == "Duplicate, Google chose different canonical":
		return CanonicalMismatch
	case signal.CoverageState == "Duplicate without user-selected canonical":
		return CanonicalMissing
	case signal.CoverageState == "Excluded by 'noindex' tag" && signal.InSitemap:
		return NoindexMisconfig
	case signal.CoverageState == "Blocked by robots.txt" && signal.InSitemap:
		return RobotsMisconfig
	case signal.CoverageState == "Crawled - currently not indexed":
		return QualityOrDup
	case signal.CoverageState == "Discovered - currently not indexed":
		return CrawlBudget
	case signal.CoverageState == "Soft 404":
		return Soft404
	case signal.PageFetchState == "SOFT_404":
		return JSRendering
	case signal.PageFetchState == "SERVER_ERROR":
		return ServerError
	case signal.PageFetchState == "ACCESS_FORBIDDEN":
		return BotBlocked
	default:
		return Unknown
	}
}
