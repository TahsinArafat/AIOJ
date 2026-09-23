# Phase B — CI/CD & Dev Velocity

**Status:** Implemented (2026-06-13)  
**Goal:** Catch regressions before merge; cut deploy lead time from ~30 min manual SSH toward automated path.  
**Depends on:** Phase A (auth/config surface stable).

## Tasks

| # | Artifact | Purpose | Status |
|---|----------|---------|--------|
| 1 | `.github/workflows/backend-test.yml` | `go test ./...` + `go build` on PR/push to main | ✅ |
| 2 | `.github/workflows/frontend-build.yml` | `npm run build` + Vitest on PR/push to main | ✅ |
| 3 | `.github/workflows/lint.yml` | `golangci-lint` + `eslint .` on PR/push to main | ✅ |
| 4 | `.github/workflows/docker-image.yml` | Build & push image to GHCR on main | ✅ |
| 5 | `.github/workflows/deploy-staging.yml` | SSH deploy: `docker compose pull && up -d` | ✅ |
| 6 | `.github/dependabot.yml` | Auto-PRs for go.mod, package.json, Docker base, Actions | ✅ |
| 7 | `Makefile` | `make test`, `make lint`, `make build-image`, `make test-frontend` | ✅ |
| 8 | `lefthook.yml` (optional) | Pre-commit: `gofmt` check + `eslint` on staged web files | ✅ |

## Required GitHub repository secrets (deploy-staging)

| Secret | Description |
|--------|-------------|
| `STAGING_HOST` | Hostname or IP of staging box |
| `STAGING_USER` | SSH user (default pattern: `deploy`) |
| `STAGING_SSH_KEY` | Private key for SSH (ed25519 recommended) |
| `STAGING_PORT` | Optional SSH port (default 22) |
| `STAGING_DIR` | Path to compose project on server (e.g. `/opt/aioj`) |

`docker-image` uses `GITHUB_TOKEN` (no extra secrets). Package write permission is set in the workflow.

## Local parity

```bash
make test            # go test ./... -count=1
make test-frontend   # cd web && npm test
make lint            # go vet + gofmt -l + eslint
make build-image     # docker build -t aioj:local .
make fmt             # gofmt -w (and go run goimports if available)
```

## Verification (2026-06-13)

- Workflow YAML parsed (Python `yaml.safe_load` all files)
- `make test-frontend` and `make lint` run locally
- Dependabot covers `gomod`, `npm`, `docker`, `github-actions`
