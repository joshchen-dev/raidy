# Raidy

[![CI](https://github.com/joshchen-dev/raidy/actions/workflows/ci.yml/badge.svg)](https://github.com/joshchen-dev/raidy/actions/workflows/ci.yml)

Raidy is a Discord-first raid scheduling project. The current MVP is one Go process backed by PostgreSQL; later components stay empty until the backend workflow is proven.

**Status:** `v0.0.1` is an early development snapshot awaiting live Discord validation.

```text
backend/     Go service, Discord interactions, PostgreSQL schema
web/         future Discord OAuth2 web UI
e2e/         future cross-component tests
deployment/  local PostgreSQL and future deployment manifests
```

## MVP behavior

- `/team setup` creates an eight-person-or-smaller static roster from Discord accounts.
- `/team manage` replaces that roster or deletes the team and its history.
- `/schedule setup` configures a weekly or biweekly timetable.
- `/schedule manage` publishes early, republishes a deleted message, pauses automation, or replaces the template.
- Poll membership is snapshotted, so later roster edits only affect future polls.
- Members explicitly mark available dates; omitted future dates become unavailable.
- Leaders confirm, cancel, or reopen dates. A confirmed date becomes `Attention required` when availability drops.
- Polls become read-only history when their final occurrence starts.

## Run locally

Requirements: Go 1.27+, Docker with Compose, and a Discord application installed in a development server with the `bot` and `applications.commands` scopes.

```sh
docker compose -f deployment/local/compose.yaml up -d
cp backend/.env.example backend/.env
set -a
source backend/.env
set +a
cd backend
go mod tidy
go run ./cmd/raidy
```

Set `DISCORD_TOKEN`. Set `DISCORD_GUILD_ID` during development so command changes appear immediately; leave it empty only for global commands. The process exposes liveness at `GET /healthz` and database readiness at `GET /readyz`.

The first migration is mounted into PostgreSQL's initialization directory. It only runs when the local volume is first created. Before adding a second migration or using persistent deployment, add and pin a migration runner.

## Verify

```sh
cd backend
go test ./...
TEST_DATABASE_URL='postgres://raidy:raidy@localhost:5432/raidy?sslmode=disable' go test ./internal/postgres -run TestSchedulingLifecycle
```

The PostgreSQL test is skipped when `TEST_DATABASE_URL` is unset. Discord interactions still require manual checks in the development server: both modals, user/date selectors, availability edits, automatic publication, expired controls, and republishing after message deletion.

## Roadmap

1. Complete the manual Discord checks, then deploy this single replica to the RHEL/k3s homelab with Ansible, Argo CD, monitoring, off-device backups, and a documented restore test.
2. Add reminders and retryable notifications; add a transactional outbox and Kafka only when those asynchronous workloads exist.
3. Add Discord-to-FFXIV character verification, then Party Finder workflows.
4. Add the web API/UI with Discord OAuth2, followed by cross-component tests and an on-demand AWS deployment path.

The PostgreSQL backup-verification operator belongs in a separate repository.
