.PHONY: test
.PHONY: smoke-oneshot
.PHONY: smoke-competitor

test:
	go test ./... -count=1

smoke-oneshot:
	WORKER_MODE=oneshot go run ./cmd/worker

smoke-competitor:
	WORKER_MODE=oneshot-competitor go run ./cmd/worker
