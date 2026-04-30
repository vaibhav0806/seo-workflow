# FEATURE.md

> An SEO agent for vibe-coded sites. It diagnoses why your pages aren't ranking, opens PRs that fix what's mechanically broken, and learns from competitors who are doing it right.

## The problem

You shipped a site on Lovable, v0, Bolt, Next.js, or Astro. Google indexes some of it. A lot of it just sits there — "Crawled, currently not indexed," "Discovered, not indexed," soft 404s, canonical conflicts you didn't know existed. Search Console tells you something's broken but not what to do about it. Existing SEO tools (Ahrefs, Screaming Frog, Semrush) are built for marketers, not devs — they show you status, not fixes, and they don't open PRs.

Meanwhile, AI-native sites like Vercel, Replit, and the bigger dev platforms are quietly evolving how they get discovered — `llms.txt`, `AGENTS.md`, structured sitemaps, prerendering — and most builders don't even know these patterns exist.

The gap: a tool that's developer-native, agent-driven, and opinionated about what to fix.

## The product in one sentence

A weekly SEO scan that pulls Google Search Console data, diagnoses indexing problems against the user's repo, opens PRs for the mechanical fixes, and tracks competitor SEO patterns to suggest emerging best practices.

## Who it's for

Solo developers and small teams shipping sites where SEO is an afterthought but matters for growth. Specifically:

- Devs deploying Next.js / Vite / Astro sites who don't want to learn SEO
- Builders shipping on AI codegen platforms (Lovable, v0, Bolt) whose default outputs have known indexing issues
- Indie hackers and bootstrapped founders who can't justify an SEO consultant but lose traffic to silent indexing failures

Not for: enterprise SEO teams, agencies managing 100+ client sites, content-first publishers (their problems are editorial, not technical).

## Why now

Three things are true in 2026 that weren't true two years ago:

1. **AI-native discovery is real.** ChatGPT, Perplexity, and Claude refer meaningful traffic. New surfaces (`llms.txt`, `AGENTS.md`, markdown sitemaps) are emerging quarterly. Most sites haven't adopted them.
2. **Agent UX is the expectation.** Devs are used to background agents (Cursor, Devin, Claude Code) opening PRs while they work on other things. A traditional dashboard feels dated.
3. **AI codegen produces SPAs that don't index.** The volume of sites built on Lovable, v0, Bolt is growing fast, and most have rendering issues that block Google from seeing the content. There's a sharp, growing audience with a specific pain point.

## Core principles

1. **PRs over dashboards.** The primary deliverable is a pull request. Dashboards exist to triage PRs, not as the main surface.
2. **Mechanical first, agentic when needed.** Don't dress up deterministic fixes as "AI." Use agency only where the path isn't predetermined.
3. **Never auto-merge.** SEO failures are silent and slow. Always require human review.
4. **Refuse what we can't do well.** If we can't safely PR a fix for a given framework or bucket, we flag and explain. We don't generate bad code.
5. **Honest about limits.** Google's data lags. Some buckets are guesses. We say so.

## What v1 does

### Indexing diagnosis (primary)

- Connect Google Search Console via OAuth
- Connect a GitHub repo via GitHub App
- Weekly scan: pull URLs from sitemap + Search Analytics, inspect each via URL Inspection API
- Classify every URL into one of ~14 buckets based on `coverageState`, `pageFetchState`, and canonical signals
- For four mechanical buckets (sitemap drift, canonical issues, robots misconfig, accidental noindex), generate a fix and open a PR
- For diagnostic buckets ("Crawled, not indexed," "Soft 404," "Discovered, not indexed"), surface a notification with examples and recommended next steps
- Slack/email digest after each scan

### Competitor SEO watcher (secondary)

- User picks competitors from a curated list (Vercel, Lovable, v0, Bolt, Replit) or adds their own
- Weekly fetch of their public SEO files: `robots.txt`, `sitemap.xml`, `llms.txt`, `llms-full.txt`, `AGENTS.md`, `sitemap.md`, `.well-known/agents.md`
- Detect material changes (new files, structural reorganization, new patterns)
- Surface as suggestions: "Vercel just added a markdown sitemap. Want one for your site?" → user clicks, we generate based on their URL list, open a PR.

### Investigation chat (stub in v1)

For diagnostic buckets where the path isn't deterministic, the notification has an "Investigate" button. v1 opens a chat thread with read-only access to scan data — agent can answer questions about the findings but cannot yet take actions or open PRs from chat. v2 expands this surface.

## What v1 does NOT do

- LLM-driven page rewrites for "Crawled, not indexed" bucket
- Citation monitoring across ChatGPT / Perplexity / Claude
- Computer-use automation of GSC's UI-only reports
- Auto-merge of any PR
- Frameworks beyond Next.js (App + Pages Router), Vite + React, and Astro
- Multi-property aggregation for sites > 2k URLs (we just span scans across multiple days)
- Bing / Yandex / IndexNow integration

These are intentional v2 candidates, not gaps.

## Architecture overview

```
                ┌────────────────────────────────┐
                │  Cloud Scheduler (weekly cron) │
                └────────────────┬───────────────┘
                                 ▼
        ┌────────────────────────────────────────────┐
        │  Worker pool (Go)                          │
        │                                            │
        │  ┌──────────────┐  ┌──────────────────┐    │
        │  │ url-discover │→ │ url-inspect      │    │
        │  └──────────────┘  └──────────────────┘    │
        │                            │               │
        │                            ▼               │
        │  ┌────────────────────────────────────┐    │
        │  │ classifier (deterministic)         │    │
        │  └────────────┬───────────────────────┘    │
        │               │                            │
        │       ┌───────┴────────┐                   │
        │       ▼                ▼                   │
        │  ┌─────────┐     ┌──────────┐              │
        │  │ pr-     │     │ notify   │              │
        │  │ builder │     │ user     │              │
        │  └─────────┘     └──────────┘              │
        └─────────────────────────────────────────────┘
                  │                │
                  ▼                ▼
            ┌────────────┐   ┌────────────┐
            │ GitHub App │   │ Slack /    │
            │            │   │ Email      │
            └────────────┘   └────────────┘

            ┌────────────────────────────┐
            │  Competitor watcher (cron) │
            │  Independent, no auth      │
            └────────────────────────────┘
```

### Stack

- **Backend:** Go (workers, API)
- **Frontend:** Next.js (dashboard, OAuth callbacks)
- **Database:** Postgres (state), Redis (queue + rate limiting)
- **Storage:** S3 / R2 (snapshots of competitor files)
- **Hosting:** Fly.io or Railway
- **Auth:** Google OAuth 2.0 (GSC), GitHub App (PRs)

### External APIs

| API | Purpose | Auth | Limits |
|---|---|---|---|
| Google Search Console — `sites` | List user's verified properties | OAuth | trivial |
| Google Search Console — `sitemaps` | URL discovery from submitted sitemaps | OAuth | trivial |
| Google Search Console — `searchanalytics` | URL discovery + impression-based prioritization | OAuth | 1,200 QPM/site |
| Google Search Console — `urlInspection` | Per-URL indexing diagnosis (the workhorse) | OAuth | **2,000 QPD + 600 QPM per property** |
| GitHub API | Branch creation, file edits, PR opening | GitHub App (JWT → installation token) | 5k/hr per installation |
| Public HTTP | Fetch competitor SEO files | None | self-imposed politeness |

## The classifier

This is the brain. Pure switch-case, no LLM, deterministic and debuggable.

| GSC State | Bucket | Auto-PR? |
|---|---|---|
| `Submitted and indexed` | healthy | – |
| `Indexed, not submitted in sitemap` | sitemap_missing_entry | ✅ |
| `Page with redirect` | sitemap_redirect | ✅ |
| `Not found (404)` (in sitemap) | sitemap_404 | ✅ |
| `Duplicate, Google chose different canonical` | canonical_mismatch | ✅ |
| `Duplicate without user-selected canonical` | canonical_missing | ✅ |
| `Excluded by 'noindex' tag` (and submitted) | noindex_misconfig | ⚠️ confirm first |
| `Blocked by robots.txt` (and submitted) | robots_misconfig | ⚠️ confirm first |
| `Crawled - currently not indexed` | quality_or_dup | flag only |
| `Discovered - currently not indexed` | crawl_budget | flag only |
| `Soft 404` | soft_404 | flag only |
| `pageFetchState = SOFT_404` + empty HTML | js_rendering | flag only |
| `pageFetchState = SERVER_ERROR (5xx)` | server_error | flag only |
| `pageFetchState = ACCESS_FORBIDDEN (403)` | bot_blocked | flag only |

v1 ships the four ✅ buckets. The two ⚠️ buckets ship as confirmation flows. Everything else surfaces as notifications.

## Onboarding flow

1. Sign in with GitHub
2. Connect Google Search Console (OAuth, scope: `webmasters.readonly`)
3. Pick a property (filtered to `permissionLevel = siteOwner`)
4. Install GitHub App on the relevant repo
5. Auto-detect framework (Next.js / Vite / Astro / unsupported)
6. First scan kicks off; results in 5–30 minutes depending on URL count

## Notifications

Weekly Slack/email digest:

```
📊 SEO scan for example.com
✅ 412 / 500 pages indexed
🔧 Auto-fixed 18 issues → PR #234
⚠️ 38 pages need investigation:
   • 23 "Crawled, not indexed" (likely quality)
   • 12 "Discovered, not indexed" (crawl budget)
   • 3 "Soft 404"

[Investigate →] [View report]
```

## Pricing model (planned)

- **Free:** 1 site, ≤ 100 URLs, weekly scans, mechanical PRs only
- **Pro ($29/mo):** 1 site, up to 2k URLs, competitor watcher, investigation chat, priority support
- **Team ($99/mo):** 3 sites, up to 10k URLs (multi-day scans), Slack integration, custom competitors

Pricing is anchored to LovableSEO and similar dev-native tools. Adjust based on alpha feedback.

## Known constraints

### OAuth verification gating

The `webmasters.readonly` scope is classified sensitive. Public launch requires Google's OAuth verification, which takes 2–6 weeks and requires:

- Public homepage on a verified domain
- Privacy policy and terms of service
- YouTube demo video of the OAuth flow
- Scope justification

**Mitigation:** submit verification on day 1 of the build, in parallel with development. Until verified, app runs in production with a 100-user cap and an "unverified app" warning — usable for an alpha but not for a real launch.

### URL Inspection quota

2,000 queries per day per property is a hard cap. For sites > 2k URLs:

- v1: scan spans multiple days, prioritized by impressions
- v2: encourage users to set up multiple GSC properties (per subdirectory) for parallel quota

### Owner permission requirement

URL Inspection only works for properties where the connected account is a verified `siteOwner`. Users added with lower permission levels see a 403. Onboarding has to detect this and explain — common pitfall when an agency or dev was given partial access.

### Framework detection

PR generation requires recognizing the user's framework. v1 supports Next.js (both routers), Vite + React, Astro. Unsupported frameworks (Hugo, Jekyll, Webflow, custom) are rejected at onboarding rather than risking bad PRs.

### Indexing data lag

GSC data trails Google's actual crawl by 2–7 days. We do not promise real-time diagnosis. Cadence is weekly because that's what matches data freshness.

### LLM trust boundaries

The classifier is deterministic. LLM use is scoped to:

- Generating PR descriptions (low risk, human-reviewed)
- Classifying competitor file diffs (low risk, advisory only)
- Investigation chat (read-only in v1, no actions)

We do not let an LLM decide what to PR. That's a switch statement.

## Build phases

### Phase 0: Pre-build (this week)

- Buy domain
- Stub homepage describing the product
- Write privacy policy + terms of service
- Create GCP project, configure OAuth consent screen
- **Submit OAuth verification** (runs in parallel for 2–6 weeks)
- Create GitHub App, install on test repo

### Phase 1: Core scan loop (week 1–2)

- Postgres schema, Redis queue
- Google OAuth + token storage with rotation handling
- GSC client: `sites.list`, `sitemaps.list`, `searchanalytics.query`, `urlInspection.index.inspect`
- Per-property token bucket rate limiter
- URL discovery (sitemap + impressions, deduped, sorted by impressions)
- Classifier (the switch statement)
- One PR generator: `sitemap_404` (the easiest bucket)

### Phase 2: PRs and notifications (week 2–3)

- GitHub App auth, branch + commit + PR creation
- Remaining three mechanical PR generators (canonical, sitemap drift, missing entries)
- Framework detector (Next.js / Vite / Astro)
- Slack integration (optional) + email digest
- Dashboard: scan history, PR list, finding triage

### Phase 3: Competitor watcher (week 3)

- Crawler for the watched files
- Snapshot store (Postgres + R2 for large)
- LLM diff classifier
- Suggestion → PR flow for `llms.txt` / `sitemap.md` / `AGENTS.md`

### Phase 4: Alpha launch (end of week 3)

- Deploy
- Onboard 5 users (existing dev network)
- Iterate on PR quality based on what gets merged vs. closed

## Success metrics

- **Week 4:** 5 alpha users, ≥ 70% of mechanical PRs merged
- **Week 8:** 25 users, ≥ 1 user reporting measurable indexing improvement
- **Week 12:** 100 users, OAuth verification approved, ≥ 5 paying customers
- **Month 6:** 500 users, $5k MRR, decision point on raising vs. continuing solo

## Open questions

- Pricing tier boundaries — is 100 URLs free generous enough to convert, tight enough to push to Pro?
- Should the competitor watcher be public (anyone can see what Vercel changed) or per-user? Public has marketing upside, private feels less spammy.
- Is "investigation chat" the right v1 stub, or should we ship an LLM-powered "soft 404 detector" that's more concrete?
- GitHub App permissions: do we ask for `contents:write` upfront, or use a two-step where read-only is default and write is opt-in per repo?

## Out of scope (forever, not just v1)

- Becoming an Ahrefs / Semrush replacement. We are not a keyword research, backlink analysis, or rank tracking tool.
- Generic content writing assistance. We diagnose technical SEO; we don't write your blog posts.
- Off-platform SEO (link building, PR, outreach). Repo-bound only.

---

*Last updated: 2026-04-30*
