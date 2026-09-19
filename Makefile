.PHONY: setup gen check test test\:live dev

GOLANGCI_LINT := $(shell go env GOPATH)/bin/golangci-lint

setup:
	go mod download
	@if [ ! -x "$(GOLANGCI_LINT)" ]; then \
		echo "installing golangci-lint..."; \
		go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest; \
	fi
	npm install
	"$(MAKE)" gen

# Regenerates services/fleet/graph/generated/ from graph/schema.*.graphqls
# (S2). The guard clause is now dead for this repo but stays cheap insurance
# against a clean clone that hasn't run `go get -tool` yet.
gen:
	@if [ -f services/fleet/gqlgen.yml ]; then \
		cd services/fleet && go run github.com/99designs/gqlgen generate; \
	else \
		echo "gen: no gqlgen.yml yet (lands in S2) — skipping"; \
	fi

check:
	go build -o bin/ ./...
	go vet ./...
	"$(GOLANGCI_LINT)" run ./...
	npm run typecheck --workspaces --if-present

test:
	go test ./...
	npm run test --workspaces --if-present

# LLM-touching tests: extraction accuracy + promptfoo golden suite. Costs
# money. Requires ANTHROPIC_API_KEY and OPENROUTER_API_KEY.
test\:live:
	go test -tags=live ./...
	npm run eval --workspace evals --if-present

dev:
	@echo "dev: fleet service + agent server + web dev server (lands with S2/S7/S8)"
