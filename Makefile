# Makefile — task runner for the workspace. Run `make help` for the menu.
#
# Make recipes MUST be tab-indented (not spaces); that's a Make rule, not ours.

INFRA := docker-compose.infra.yml

# Auto-discover every Go module in the repo (each dir containing a go.mod),
# skipping hidden dirs. Lets build/vet/test work across all modules without
# editing this file every time we add a service.
MODULE_DIRS := $(shell find . -name go.mod -not -path '*/.*' -exec dirname {} \;)

# Use the go-installed golang-migrate by absolute path: the system has a DIFFERENT
# `migrate` (python sqlalchemy-migrate) earlier on PATH that would otherwise shadow it.
MIGRATE := $(shell go env GOPATH)/bin/migrate
SQLC    := $(shell go env GOPATH)/bin/sqlc

# golangci-lint v2. Override if it's not on PATH (e.g. GOLANGCI=$(go env GOPATH)/bin/golangci-lint).
GOLANGCI ?= golangci-lint

# Per-service DB URLs. Host port 5433 maps to the postgres container's 5432.
PRODUCT_DB_URL     ?= postgres://ecommerce:ecommerce@localhost:5433/productdb?sslmode=disable
PRODUCT_MIGRATIONS := services/product/migrations
USER_DB_URL        ?= postgres://ecommerce:ecommerce@localhost:5433/userdb?sslmode=disable
USER_MIGRATIONS    := services/user/migrations
PAYMENT_DB_URL     ?= postgres://ecommerce:ecommerce@localhost:5433/paymentdb?sslmode=disable
PAYMENT_MIGRATIONS := services/payment/migrations
ORDER_DB_URL       ?= postgres://ecommerce:ecommerce@localhost:5433/orderdb?sslmode=disable
ORDER_MIGRATIONS   := services/order/migrations
NOTIFICATION_DB_URL     ?= postgres://ecommerce:ecommerce@localhost:5433/notificationdb?sslmode=disable
NOTIFICATION_MIGRATIONS := services/notification/migrations

.PHONY: help infra-up infra-down infra-logs infra-ps up down down-v build vet test tidy lint \
	product-migrate-up product-migrate-down product-migrate-create product-sqlc \
	user-migrate-up user-migrate-down user-migrate-create user-make-admin user-sqlc \
	payment-migrate-up payment-migrate-down payment-sqlc \
	order-migrate-up order-migrate-down order-sqlc \
	notification-migrate-up notification-migrate-down notification-sqlc

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

## ---- Infrastructure ----
infra-up: ## Start infra (postgres, redis, nats, jaeger, prometheus, minio), wait for healthy
	docker compose -f $(INFRA) up -d --wait
	# Bucket setup runs after MinIO is healthy. It's a one-shot (behind the "init"
	# compose profile) so it isn't part of the --wait set above, which would fail
	# on its clean exit.
	docker compose -f $(INFRA) run --rm createbuckets

infra-down: ## Stop infra, keep data volumes
	docker compose -f $(INFRA) down

down-v: ## Stop infra AND delete data volumes (wipes dbs; re-runs initdb next up)
	docker compose -f $(INFRA) down -v

infra-logs: ## Tail infra logs
	docker compose -f $(INFRA) logs -f

infra-ps: ## Show infra container status
	docker compose -f $(INFRA) ps

# Bring up the whole stack: infra + apps. The order is guaranteed by depends_on.
up: ## Bring everything up (infra + apps), wait for healthy
	docker compose -f $(INFRA) -f docker-compose.yml up -d --wait
	# Create the MinIO product-images bucket. It's a one-shot behind the "init"
	# compose profile (so it isn't part of the --wait set above, which would fail
	# on its clean exit) — run it explicitly, same as infra-up. Idempotent.
	docker compose -f $(INFRA) -f docker-compose.yml run --rm createbuckets

down: ## Bring everything down (keep volumes)
	docker compose -f $(INFRA) -f docker-compose.yml down

## ---- Go workspace ----
build: ## Build every module in the workspace
	@for d in $(MODULE_DIRS); do echo "build $$d"; (cd $$d && go build ./...) || exit 1; done

vet: ## go vet every module
	@for d in $(MODULE_DIRS); do echo "vet $$d"; (cd $$d && go vet ./...) || exit 1; done

test: ## go test every module (CI passes GOTEST_FLAGS=-race)
	@for d in $(MODULE_DIRS); do echo "test $$d"; (cd $$d && go test $(GOTEST_FLAGS) ./...) || exit 1; done

tidy: ## go mod tidy every module
	@for d in $(MODULE_DIRS); do echo "tidy $$d"; (cd $$d && go mod tidy) || exit 1; done

lint: ## golangci-lint every module (uses the root .golangci.yml)
	@for d in $(MODULE_DIRS); do echo "lint $$d"; (cd $$d && $(GOLANGCI) run ./...) || exit 1; done

## ---- Database migrations ----
product-migrate-up: ## Apply all productdb migrations
	$(MIGRATE) -path $(PRODUCT_MIGRATIONS) -database "$(PRODUCT_DB_URL)" up

product-migrate-down: ## Roll back the last productdb migration
	$(MIGRATE) -path $(PRODUCT_MIGRATIONS) -database "$(PRODUCT_DB_URL)" down 1

product-migrate-create: ## Create a new productdb migration: make product-migrate-create NAME=add_x
	$(MIGRATE) create -ext sql -dir $(PRODUCT_MIGRATIONS) -seq $(NAME)

user-migrate-up: ## Apply all userdb migrations
	$(MIGRATE) -path $(USER_MIGRATIONS) -database "$(USER_DB_URL)" up

user-migrate-down: ## Roll back the last userdb migration
	$(MIGRATE) -path $(USER_MIGRATIONS) -database "$(USER_DB_URL)" down 1

user-migrate-create: ## Create a new userdb migration: make user-migrate-create NAME=add_x
	$(MIGRATE) create -ext sql -dir $(USER_MIGRATIONS) -seq $(NAME)

user-make-admin: ## Promote a registered user to admin: make user-make-admin EMAIL=you@example.com
	@test -n "$(EMAIL)" || { echo "EMAIL is required: make user-make-admin EMAIL=you@example.com"; exit 1; }
	psql "$(USER_DB_URL)" -v email="$(EMAIL)" \
		-c "UPDATE users SET role='admin', updated_at=now() WHERE email = :'email';"

## ---- Code generation ----
product-sqlc: ## Regenerate product sqlc code from queries.sql
	cd services/product && $(SQLC) generate

user-sqlc: ## Regenerate user sqlc code from queries.sql
	cd services/user && $(SQLC) generate

payment-migrate-up: ## Apply all paymentdb migrations
	$(MIGRATE) -path $(PAYMENT_MIGRATIONS) -database "$(PAYMENT_DB_URL)" up
payment-migrate-down: ## Roll back the last paymentdb migration
	$(MIGRATE) -path $(PAYMENT_MIGRATIONS) -database "$(PAYMENT_DB_URL)" down 1
payment-sqlc: ## Regenerate payment sqlc code
	cd services/payment && $(SQLC) generate

order-migrate-up: ## Apply all orderdb migrations
	$(MIGRATE) -path $(ORDER_MIGRATIONS) -database "$(ORDER_DB_URL)" up
order-migrate-down: ## Roll back the last orderdb migration
	$(MIGRATE) -path $(ORDER_MIGRATIONS) -database "$(ORDER_DB_URL)" down 1
order-sqlc: ## Regenerate order sqlc code
	cd services/order && $(SQLC) generate

notification-migrate-up: ## Apply all notificationdb migrations
	$(MIGRATE) -path $(NOTIFICATION_MIGRATIONS) -database "$(NOTIFICATION_DB_URL)" up
notification-migrate-down: ## Roll back the last notificationdb migration
	$(MIGRATE) -path $(NOTIFICATION_MIGRATIONS) -database "$(NOTIFICATION_DB_URL)" down 1
notification-sqlc: ## Regenerate notification sqlc code
	cd services/notification && $(SQLC) generate
