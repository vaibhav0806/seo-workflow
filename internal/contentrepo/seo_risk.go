package contentrepo

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

type SEORiskSeverity string

const (
	SEORiskLow      SEORiskSeverity = "LOW"
	SEORiskMedium   SEORiskSeverity = "MEDIUM"
	SEORiskHigh     SEORiskSeverity = "HIGH"
	SEORiskCritical SEORiskSeverity = "CRITICAL"
)

type SEORiskFinding struct {
	Code          string
	Risk          string
	Severity      SEORiskSeverity
	AffectedField string
	Evidence      string
	SuggestedFix  string
	GooglePolicy  string
}

type SEORiskReport struct {
	Severity SEORiskSeverity
	Risky    bool
	Summary  string
	Findings []SEORiskFinding
}

var wordPattern = regexp.MustCompile(`[a-z0-9]+`)
var markdownLinkPattern = regexp.MustCompile(`\[[^\]]+\]\((https?://[^)]+|/[^)]+)\)`)

func AssessSEORisk(post BlogPost) SEORiskReport {
	findings := make([]SEORiskFinding, 0)
	title := strings.TrimSpace(post.Title)
	body := strings.TrimSpace(post.BodyMarkdown)
	wordCount := len(wordPattern.FindAllString(strings.ToLower(body), -1))

	if title != "" && wordCount > 0 {
		phrase := strings.ToLower(strings.TrimSpace(title))
		count := strings.Count(strings.ToLower(body), phrase)
		if count >= 3 {
			findings = append(findings, SEORiskFinding{
				Code:          "keyword-repetition",
				Risk:          "Keyword stuffing",
				Severity:      SEORiskMedium,
				AffectedField: "body",
				Evidence:      fmt.Sprintf("`%s` appears %d times in %d words.", title, count, wordCount),
				SuggestedFix:  "Reduce exact-match title repetition and use natural semantic variants.",
				GooglePolicy:  "Keyword stuffing",
			})
		}

		corePhrase := coreTitlePhrase(title)
		if corePhrase != "" && corePhrase != phrase {
			coreCount := strings.Count(strings.ToLower(body), corePhrase)
			if coreCount >= 8 {
				findings = append(findings, SEORiskFinding{
					Code:          "keyword-repetition",
					Risk:          "Keyword stuffing",
					Severity:      SEORiskMedium,
					AffectedField: "body",
					Evidence:      fmt.Sprintf("`%s` appears %d times in %d words.", corePhrase, coreCount, wordCount),
					SuggestedFix:  "Reduce exact-match repetition and use natural alternatives.",
					GooglePolicy:  "Keyword stuffing",
				})
			}
		}
	}

	if isTopXTitle(title) && !containsAnyFold(body, "how we evaluated", "methodology", "evaluation criteria", "ranking criteria") {
		findings = append(findings, SEORiskFinding{
			Code:          "missing-listicle-methodology",
			Risk:          "Low-quality review/listicle",
			Severity:      SEORiskHigh,
			AffectedField: "body",
			Evidence:      "Title uses a Top/Best listicle pattern but no methodology or evaluation criteria section was found.",
			SuggestedFix:  "Add clear evaluation criteria, best-fit guidance, tradeoffs, and review limitations.",
			GooglePolicy:  "High quality reviews and helpful content",
		})
	}

	if wordCount > 0 && wordCount < 700 {
		findings = append(findings, SEORiskFinding{
			Code:          "thin-content",
			Risk:          "Thin or unhelpful content",
			Severity:      SEORiskMedium,
			AffectedField: "body",
			Evidence:      fmt.Sprintf("Draft has %d words; competitive SEO posts usually need deeper standalone coverage.", wordCount),
			SuggestedFix:  "Expand with concrete examples, criteria, tradeoffs, and CreateOS-specific analysis.",
			GooglePolicy:  "Helpful, reliable, people-first content",
		})
	}

	internalLinks, externalLinks := countEditorialLinks(body)
	if internalLinks < 2 {
		findings = append(findings, SEORiskFinding{
			Code:          "insufficient-internal-links",
			Risk:          "Internal linking",
			Severity:      SEORiskLow,
			AffectedField: "body",
			Evidence:      fmt.Sprintf("Draft contains %d CreateOS internal link(s).", internalLinks),
			SuggestedFix:  "Add at least two contextual links to existing CreateOS product or blog pages.",
			GooglePolicy:  "Crawlability and helpful navigation",
		})
	}
	if externalLinks < 2 {
		findings = append(findings, SEORiskFinding{
			Code:          "insufficient-external-sources",
			Risk:          "External sourcing",
			Severity:      SEORiskLow,
			AffectedField: "body",
			Evidence:      fmt.Sprintf("Draft contains %d external source link(s).", externalLinks),
			SuggestedFix:  "Support material technical or market claims with at least two authoritative primary sources.",
			GooglePolicy:  "Trust and factual support",
		})
	}

	severity := maxSEORiskSeverity(findings)
	risky := severity == SEORiskHigh || severity == SEORiskCritical
	summary := fmt.Sprintf("%s risk: %d finding(s) require human review.", severity, len(findings))
	if len(findings) == 0 {
		summary = "LOW risk: no obvious automated SEO spam signals found."
	}
	return SEORiskReport{Severity: severity, Risky: risky, Summary: summary, Findings: dedupeSEORiskFindings(findings)}
}

func countEditorialLinks(body string) (int, int) {
	internal := 0
	external := 0
	for _, match := range markdownLinkPattern.FindAllStringSubmatch(body, -1) {
		target := strings.ToLower(strings.TrimSpace(match[1]))
		if strings.HasPrefix(target, "/") || strings.Contains(target, "createos.sh/") {
			internal++
			continue
		}
		external++
	}
	return internal, external
}

func (report SEORiskReport) FindingCodes() []string {
	codes := make([]string, 0, len(report.Findings))
	for _, finding := range report.Findings {
		codes = append(codes, finding.Code)
	}
	sort.Strings(codes)
	return codes
}

func (report SEORiskReport) Markdown() string {
	severity := report.Severity
	if severity == "" {
		severity = SEORiskLow
	}
	lines := []string{
		"## SEO Risk Review: " + string(severity),
		"",
		"This is an automated risk review, not a merge decision. Humans should use this evidence to decide whether to merge, edit, or use a mitigated PR.",
		"",
		report.Summary,
		"",
	}
	if len(report.Findings) == 0 {
		return strings.Join(append(lines, "No automated SEO spam or helpful-content risks were detected."), "\n")
	}
	lines = append(lines,
		"| Risk | Severity | Field | Evidence | Suggested Fix | Google Policy Area |",
		"|---|---:|---|---|---|---|",
	)
	for _, finding := range report.Findings {
		lines = append(lines, fmt.Sprintf("| %s | %s | %s | %s | %s | %s |",
			escapeMarkdownTable(finding.Risk),
			titleSeverity(finding.Severity),
			escapeMarkdownTable(finding.AffectedField),
			escapeMarkdownTable(finding.Evidence),
			escapeMarkdownTable(finding.SuggestedFix),
			escapeMarkdownTable(finding.GooglePolicy),
		))
	}
	return strings.Join(lines, "\n")
}

func MitigateSEORisk(post BlogPost, report SEORiskReport) BlogPost {
	mitigated := post
	if !strings.HasSuffix(mitigated.Slug, "-mitigated") {
		mitigated.Slug += "-mitigated"
	}
	body := strings.TrimSpace(mitigated.BodyMarkdown)
	if body == "" {
		body = "# " + mitigated.Title
	}
	if isTopXTitle(mitigated.Title) && !containsAnyFold(body, "how we evaluated", "methodology", "evaluation criteria", "ranking criteria") {
		body += "\n\n## How We Evaluated These Platforms\n\nThis comparison should be reviewed against production readiness criteria: deployment control, governance, security posture, integration depth, orchestration support, reliability, ownership model, and how much work remains after the first prototype. The goal is not to crown a universal winner. The goal is to help enterprise teams identify which platform fits their operating constraints."
	}
	body += "\n\n## Editorial Risk Mitigation Notes\n\nThis mitigated draft was generated because the original PR triggered automated SEO risk checks. Before merge, human review should confirm the methodology is specific, the claims are supportable, the wording is not repetitive, and the article gives readers enough original CreateOS perspective to be useful without returning to search results."
	if len(report.Findings) > 0 {
		body += "\n\nAutomated findings addressed in this version: " + strings.Join(report.FindingCodes(), ", ") + "."
	}
	mitigated.BodyMarkdown = body
	mitigated.ReadTime = estimatedReadTime(body)
	return mitigated
}

func isTopXTitle(title string) bool {
	lower := strings.ToLower(strings.TrimSpace(title))
	return strings.HasPrefix(lower, "top ") || strings.HasPrefix(lower, "best ")
}

func containsAnyFold(value string, needles ...string) bool {
	lower := strings.ToLower(value)
	for _, needle := range needles {
		if strings.Contains(lower, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func coreTitlePhrase(title string) string {
	lower := strings.ToLower(title)
	rawTokens := wordPattern.FindAllString(lower, -1)
	stop := map[string]struct{}{
		"top": {}, "best": {}, "for": {}, "the": {}, "in": {}, "with": {}, "and": {}, "to": {}, "of": {},
		"teams": {}, "team": {}, "enterprise": {}, "2026": {},
	}
	tokens := make([]string, 0, 4)
	for _, token := range rawTokens {
		if _, skip := stop[token]; skip {
			continue
		}
		tokens = append(tokens, token)
		if len(tokens) == 3 {
			break
		}
	}
	return strings.Join(tokens, " ")
}

func maxSEORiskSeverity(findings []SEORiskFinding) SEORiskSeverity {
	severity := SEORiskLow
	for _, finding := range findings {
		if severityRank(finding.Severity) > severityRank(severity) {
			severity = finding.Severity
		}
	}
	return severity
}

func severityRank(severity SEORiskSeverity) int {
	switch severity {
	case SEORiskCritical:
		return 4
	case SEORiskHigh:
		return 3
	case SEORiskMedium:
		return 2
	default:
		return 1
	}
}

func dedupeSEORiskFindings(findings []SEORiskFinding) []SEORiskFinding {
	out := make([]SEORiskFinding, 0, len(findings))
	seen := map[string]struct{}{}
	for _, finding := range findings {
		key := finding.Code + "|" + finding.Evidence
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, finding)
	}
	return out
}

func titleSeverity(severity SEORiskSeverity) string {
	switch severity {
	case SEORiskCritical:
		return "Critical"
	case SEORiskHigh:
		return "High"
	case SEORiskMedium:
		return "Medium"
	default:
		return "Low"
	}
}

func escapeMarkdownTable(value string) string {
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "|", "\\|")
	return strings.TrimSpace(value)
}
