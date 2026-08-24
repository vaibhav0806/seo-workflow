.PHONY: test
.PHONY: smoke-oneshot
.PHONY: smoke-competitor
.PHONY: discover-blog-opportunities
.PHONY: publish-approved-blogs
.PHONY: test-e2e

test:
	go test ./... -count=1

test-e2e:
	go test ./internal/competitor -run '^TestE2E' -count=1

smoke-oneshot:
	WORKER_MODE=oneshot go run ./cmd/worker

smoke-competitor:
	WORKER_MODE=oneshot-competitor go run ./cmd/worker

discover-blog-opportunities:
	WORKER_MODE=oneshot-competitor go run ./cmd/worker

publish-approved-blogs:
	WORKER_MODE=approved-content go run ./cmd/worker
