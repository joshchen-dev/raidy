# Raidy

[![CI](https://github.com/joshchen-dev/raidy/actions/workflows/ci.yml/badge.svg)](https://github.com/joshchen-dev/raidy/actions/workflows/ci.yml)

Raidy is a raid scheduling project for Discord statics: the web console is where everyone plans, and Discord carries the announcements. One Go process runs the Discord bot, web API, scheduler, and production web assets, backed by PostgreSQL.

**Status:** `v0.0.13` is an early development snapshot.

```text
backend/     Go service, Discord interactions, web API, PostgreSQL migrations
web/         React, TypeScript, Vite, Tailwind CSS
e2e/         future cross-component tests
deployment/  local PostgreSQL and Kubernetes deployment manifests
```

## MVP behavior

- The web console, behind Discord OAuth2, is where teams, rosters of up to eight, and weekly or biweekly schedules are managed.
- Members mark the dates they can attend on the availability grid; omitted future dates become unavailable.
- Leaders confirm, cancel, or reopen dates. A confirmed date becomes `Attention required` when availability drops.
- Active polls follow roster additions and removals immediately; closed poll history stays unchanged.
- Polls become read-only history when their final occurrence starts.
- Discord is announcement-only: each period is posted with its dates in every reader's own timezone, live availability counts and status, and an **Open in Raidy** link. The post is edited in place as votes and decisions change.
- `/raidy` replies privately with a link to your team's page.
- The responsive console supports persisted light and dark themes.

## Run locally

Requirements: Go 1.27+, Node.js 24+, Docker with Compose, and a Discord application installed in a development server with the `bot` and `applications.commands` scopes.

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

In a second terminal:

```sh
cd web
npm install
npm run dev
```

Set `DISCORD_TOKEN`, `DISCORD_CLIENT_ID`, and `DISCORD_CLIENT_SECRET`, and set `APP_BASE_URL` (for example `http://localhost:5173`) so Discord posts link back to the console. Add `http://localhost:5173/api/auth/callback` as an OAuth2 redirect in the Discord developer portal and enable **Server Members Intent** for roster search. Set `DISCORD_GUILD_ID` during development so command changes appear immediately; leave it empty only for global commands.

The Vite development server opens at `http://localhost:5173` and proxies API calls to the Go process. For a production-style run, build `web/`, set `WEB_DIST_DIR` to its `dist` directory, and set `APP_BASE_URL` to the public origin. The Go process then serves both the API and static assets.

The Go process applies pinned, embedded Goose migrations before opening the application store. Existing local databases are upgraded in place.

## Run on local Kubernetes

The production image packages the Go service and built web console. The local Kustomize overlay runs it with a persistent PostgreSQL instance on k3d and exposes it through `kubectl port-forward`; see [deployment/README.md](deployment/README.md).

## Verify

```sh
cd backend
go test ./...
cd ..
docker compose -f deployment/local/compose.yaml exec db createdb -U raidy raidy_test
cd backend
TEST_DATABASE_URL='postgres://raidy:raidy@localhost:5432/raidy_test?sslmode=disable' go test ./internal/postgres -run TestSchedulingLifecycle
cd ../web
npm run build
```

The PostgreSQL lifecycle test truncates its target database, so always point it at a dedicated test database. It is skipped when `TEST_DATABASE_URL` is unset. Discord still requires manual checks in the development server: `/raidy`, automatic publication, live edits after web votes and date changes, the closed-history footer, and republishing after message deletion.

## Development workflow

`main` is protected and always releasable. Create a short-lived `feat/*`, `fix/*`, or `chore/*` branch, open a pull request, and wait for the required `test` check. Pull requests are squash-merged so `main` stays linear, and merged branches are deleted automatically.

Use a conventional PR title because it becomes the commit on `main`. During pre-1.0 development, every `feat:` or `fix:` pull request increments the patch version in `web/package.json`; the lockfile and README version must match. After the merged `main` build passes, push the matching `vX.Y.Z` tag to publish a GitHub Release with generated notes.

## Roadmap

1. Validate the container and Kubernetes manifests on k3d, then deploy this single replica to the RHEL/k3s homelab with Ansible, Argo CD, monitoring, off-device backups, and a documented restore test.
2. Add reminders and retryable notifications; add a transactional outbox and Kafka only when those asynchronous workloads exist.
3. Add Discord-to-FFXIV character verification, then Party Finder workflows.
4. Add cross-component tests and an on-demand AWS deployment path.

The PostgreSQL backup-verification operator belongs in a separate repository.
