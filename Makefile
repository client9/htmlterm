SHELL := sh

.PHONY: help
.DEFAULT_GOAL := help
help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}' 

build: ## build module
	go build ./...

test: ## run all unit tests
	go test ./...

race: ## run unit tests with the race detector
	go test -race ./...

version: ## print OS, Go, and golangci versions
	@echo $$0
	@uname -a
	@go version
	@golangci-lint --version

bench: ## run local benchmarks
	go test -benchmem -bench .

cover: ## generate code coverage report
	rm -f cover.out
	go test ./... -coverprofile=cover.out -coverpkg=./...
	go tool cover -func=cover.out

vuln: ## run Go vulnerability analysis
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

## NOTE: this downloads it's schema over the network
lintverify:
	golangci-lint config verify

fmt: ## reformat source code
	go mod tidy
	gofmt -w -s $$(find . -name '*.go' -not -path './.git/*')

lint: ## lint and verify repo is already formatted
	go mod tidy
	git diff --exit-code -- go.mod go.sum
	test -z "$$(gofmt -l $$(find . -name '*.go' -not -path './.git/*'))"
	golangci-lint run .

clean: ## remove any generated files
	rm -f *.out
