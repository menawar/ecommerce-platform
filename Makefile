# Makefile — task runner for the workspace. Run `make help` for the menu.
#
# Make recipes MUST be tab-indented (not spaces); that's a Make rule, not ours.

INFRA := docker-compose.infra.yml

# Auto-discover every Go module in the repo (each dir containing a go.mod),
# skipping hidden dirs. Lets build/vet/test work across all modules without
# editing this file every time we add a service.
MODULE_DIRS := $(shell find . -name go.mod -not -path '*/.*' -exec dirname {} \;)

.PHONY: help infra-up infra-down infra-logs infra-ps up down down-v build vet test tidy

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

## ---- Infrastructure ----
infra-up: ## Start infra (postgres, redis, nats, jaeger, prometheus), wait for healthy
	docker compose -f $(INFRA) up -d --wait

infra-down: ## Stop infra, keep data volumes
	docker compose -f $(INFRA) down

down-v: ## Stop infra AND delete data volumes (wipes dbs; re-runs initdb next up)
	docker compose -f $(INFRA) down -v

infra-logs: ## Tail infra logs
	docker compose -f $(INFRA) logs -f

infra-ps: ## Show infra container status
	docker compose -f $(INFRA) ps

# `up`/`down` are aliases for infra today; they'll also bring up app services
# (docker-compose.yml) once those exist.
up: infra-up   ## Bring everything up (infra for now)
down: infra-down ## Bring everything down (keep volumes)

## ---- Go workspace ----
build: ## Build every module in the workspace
	@for d in $(MODULE_DIRS); do echo "build $$d"; (cd $$d && go build ./...) || exit 1; done

vet: ## go vet every module
	@for d in $(MODULE_DIRS); do echo "vet $$d"; (cd $$d && go vet ./...) || exit 1; done

test: ## go test every module
	@for d in $(MODULE_DIRS); do echo "test $$d"; (cd $$d && go test ./...) || exit 1; done

tidy: ## go mod tidy every module
	@for d in $(MODULE_DIRS); do echo "tidy $$d"; (cd $$d && go mod tidy) || exit 1; done
