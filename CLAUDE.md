# Project Instructions for AI Agents

This file provides instructions and context for AI coding agents working on this project.

<!-- BEGIN BEADS INTEGRATION v:1 profile:minimal hash:6cd5cc61 -->
## Beads Issue Tracker

This project uses **bd (beads)** for issue tracking. Run `bd prime` to see full workflow context and commands.

### Quick Reference

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --claim  # Claim work
bd close <id>         # Complete work
```

### Rules

- Use `bd` for ALL task tracking — do NOT use TodoWrite, TaskCreate, or markdown TODO lists
- Run `bd prime` for detailed command reference and session close protocol
- Use `bd remember` for persistent knowledge — do NOT use MEMORY.md files

**Architecture in one line:** issues live in a local Dolt DB; sync uses `refs/dolt/data` on your git remote; `.beads/issues.jsonl` is a passive export. See https://github.com/gastownhall/beads/blob/main/docs/SYNC_CONCEPTS.md for details and anti-patterns.

## Agent Context Profiles

The managed Beads block is task-tracking guidance, not permission to override repository, user, or orchestrator instructions.

- **Conservative (default)**: Use `bd` for task tracking. Do not run git commits, git pushes, or Dolt remote sync unless explicitly asked. At handoff, report changed files, validation, and suggested next commands.
- **Minimal**: Keep tool instruction files as pointers to `bd prime`; use the same conservative git policy unless active instructions say otherwise.
- **Team-maintainer**: Only when the repository explicitly opts in, agents may close beads, run quality gates, commit, and push as part of session close. A current "do not commit" or "do not push" instruction still wins.

## Session Completion

This protocol applies when ending a Beads implementation workflow. It is subordinate to explicit user, repository, and orchestrator instructions.

1. **File issues for remaining work** - Create beads for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **Handle git/sync by active profile**:
   ```bash
   # Conservative/minimal/default: report status and proposed commands; wait for approval.
   git status

   # Team-maintainer opt-in only, unless current instructions forbid it:
   git pull --rebase
   git push
   git status
   ```
5. **Hand off** - Summarize changes, validation, issue status, and any blocked sync/commit/push step

**Critical rules:**
- Explicit user or orchestrator instructions override this Beads block.
- Do not commit or push without clear authority from the active profile or the current user request.
- If a required sync or push is blocked, stop and report the exact command and error.
<!-- END BEADS INTEGRATION -->


## Build & Test

```bash
go build ./...
go vet ./...
go test ./... -v
golangci-lint run ./...
hadolint Dockerfile
kubeconform -strict kubernetes/*.yaml
goreleaser build --single-target --snapshot --clean -o busyhttp
docker build -t busyhttp:dev .
```

## Architecture Overview

busyhttp is a Go rewrite of [jpetazzo/busyhttp](https://github.com/jpetazzo/busyhttp):
an HTTP server that busy-spins the CPU for a configurable duration on every
request, used as a Kubernetes HPA/autoscaling demo tool in Platform Fix's
workshops (a controlled replacement for the upstream `registry.k8s.io/hpa-example`
image). Packaged to the same engineering standard as
[platformfix/colour](https://github.com/platformfix/colour), minus a Helm chart.

- `cmd/busyhttp/main.go`: wiring. Reads `PORT`/`BUSY_SECONDS` env vars, sets
  up the `http.ServeMux`, handles graceful shutdown.
- `internal/busyhttp/handler.go`: `NewHandler(duration time.Duration) http.HandlerFunc`
  (the busy-spin handler) and `Healthz` (liveness/readiness).
- `Dockerfile`: distroless nonroot base image.
- `kubernetes/`: raw `Deployment`/`Service` manifests (no Helm chart, deliberately).
- `.goreleaser.yaml` + `.github/workflows/release.yml`: tagged releases
  (`vX.Y.Z`) build multi-arch (amd64/arm64) cosign-signed images with SBOM
  and SLSA provenance, published to `ghcr.io/platformfix/busyhttp` under
  both the version tag and `:latest`. The two tags reference the identical
  multi-arch manifest, so a versioned release always produces a versioned
  image.

Full history: the original design and implementation plan (now superseded,
since the design spec was deleted once v0.1.0 shipped) are at
`docs/superpowers/plans/2026-09-07-busyhttp-build.md`.

## Conventions & Patterns

- **Never use `time.Sleep` in the handler.** The busy-spin (`for
  time.Now().Before(deadline) {}`) is the entire point of this tool: it has
  to show up as real CPU load an HPA can react to. `.github/workflows/e2e.yml`
  verifies this empirically by sampling `docker stats` CPU% during a request,
  not just checking elapsed wall-clock time.
- **`main` is branch-protected**: PRs required (enforced for admins too),
  squash-merge only, no force-pushes/deletions, signed commits required.
  Required status checks: `build, vet, test`, `golangci-lint`, `hadolint`,
  `kubeconform`, `pr-lint`, `commit-lint`, `govulncheck`, `DCO`.
- **Every commit needs `git commit -s`** (DCO sign-off), enforced by both
  the `commit-lint` job and the org's DCO GitHub App.
- **No Helm chart.** Deliberately excluded; `kubernetes/` ships raw manifests
  only.
- Repo-level security hardening is on: secret scanning, push protection,
  private vulnerability reporting, Dependabot security updates, and required
  SHA-pinning for GitHub Actions.
