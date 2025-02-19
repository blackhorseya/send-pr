BIN := bin

.PHONY: help
help: ## Show help message.
	@printf "Usage:\n"
	@printf "  make <target>\n\n"
	@printf "Targets:\n"
	@perl -nle'print $& if m{^[a-zA-Z0-9_-]+:.*?## .*$$}' $(MAKEFILE_LIST) | \
		sort | \
		awk 'BEGIN {FS = ":.*?## "}; \
		{printf "  %-18s %s\n", $$1, $$2}'

.PHONY: install-tools
install-tools: ## Install tools
	awk -F'"' '/_/ {print $$2}' tools.go | xargs -tI % go install %