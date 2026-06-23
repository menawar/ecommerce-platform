---
name: db-migrate
description: Apply (or roll back) database migrations for the e-commerce services. Use when the user wants to migrate the databases up to date or revert a migration.
---

Apply all service migrations against the local Postgres (host :5433).

Up (all services):
```
make infra-up
for d in user product payment order notification; do make ${d}-migrate-up; done
```

Single service: `make <service>-migrate-up` / `make <service>-migrate-down` (down rolls back ONE step — confirm before running, it's destructive).
New migration: `make <service>-migrate-create NAME=<desc>` (also `MIGRATE`/`SQLC` are called by absolute `$(GOPATH)/bin` path because a system `migrate` shadows golang-migrate).

After migrating, report each service's applied version (golang-migrate prints `N/u <name>`), or note "no change".
