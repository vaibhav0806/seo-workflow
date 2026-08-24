---
name: createos-blog-opportunities
description: "Find, rank, approve, generate, and review CreateOS blog opportunities about enterprise agentic AI, AI agents, production workflows, and secure sandboxes. Use when asked for blog ideas, competitor content gaps, SEO title prioritization, approval queues, CreateOS content PRs, or internal/external link review."
---

# CreateOS Blog Opportunities

Use the SEO workflow repository as the source of truth. Do not invent a competitor list or publish commercial landing pages. CreateOS publishes blog content through `NodeOps-app/createos-content`.

## Find and rank titles

1. Inspect `internal/competitor/run.go` for the active competitor list and `docs/createos-context.md` for product positioning.
2. Use the configured CreateOS content inventory, open content PR inventory, and search-performance state when available.
3. Run `make discover-blog-opportunities` to create the approval queue.
4. Rank only topics with a defensible connection to:
   - enterprise agentic AI architecture, governance, reliability, or adoption;
   - AI agent building, orchestration, evaluation, deployment, or operations;
   - production AI workflows and the path from prototype to production;
   - secure sandboxes, isolated code execution, permissions, or runtime controls.
5. Prefer competitor-backed gaps with primary evidence, measurable search signals, or a strong CreateOS product angle. Reject generic AI news, trend summaries, vague thought leadership, and unrelated consumer topics.
6. Preserve the inventory decision. Never turn a `refresh` or `consolidate` recommendation into a new duplicate blog.

Each title must expose the primary keyword, search intent, evidence URLs, CreateOS angle, decision, existing route when applicable, and contextual internal-link targets. Titles should promise a concrete reader outcome and remain query-aligned; do not use competitor names merely for novelty.

## Require human approval

Discovery must stop after writing `CONTENT_APPROVAL_PATH`. Do not generate article prose, covers, GitHub branches, or pull requests while an item is `pending`.

A human approves a title by changing its `approvalStatus` to `approved`. Treat `rejected` as final unless the human explicitly revisits it. Discovery reruns preserve both decisions by stable approval ID.

## Generate approved blogs

Run `make publish-approved-blogs` only after explicit approval. The workflow must:

- accept only queue items marked `approved`;
- reload the latest content inventory and draft only items still classified `create`;
- lock the approved title and `/blogs/...` route;
- follow the CreateOS context and Naman/CreateOS writing guidance in the repo;
- use only existing CreateOS routes for internal links;
- cite only external URLs present in the opportunity evidence and never invent citations;
- open the PR in `NodeOps-app/createos-content`, where merge remains a separate human decision.

## Review before recommending merge

Read the rendered post, frontmatter, and PR diff. Report concrete findings before general commentary. Check:

- search intent, title/H1 alignment, canonical slug, meta description, headings, and useful standalone depth;
- duplicate/cannibalization risk against every existing CreateOS blog;
- at least two contextual internal links to existing CreateOS routes;
- at least two authoritative external primary sources for material factual claims;
- broken, invented, redirected, or weak links;
- unsupported claims, keyword repetition, filler, generic AI prose, fake quotations, and listicles without methodology;
- CreateOS relevance, honest tradeoffs, production detail, author/frontmatter consistency, cover availability, and CI.

Do not call internal links “backlinks.” Distinguish internal links, outbound citations, and genuine inbound backlinks. Never claim CI, link validity, or merge readiness without fresh verification.
