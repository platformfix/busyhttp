# busyhttp Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `platformfix/busyhttp`, a Go HTTP server that busy-spins the CPU for a configurable duration per request, packaged to `platformfix/colour`'s full engineering standard minus a Helm chart, and cut its first tagged release.

**Architecture:** A tiny `net/http` server (`cmd/busyhttp/main.go` wiring, `internal/busyhttp` handler package) wrapped in the same CI/release scaffolding as `colour`: golangci-lint + hadolint, a Go-test CI job, a Docker-level e2e job that proves the CPU-burn behavior, kubeconform validation of raw K8s manifests, and a goreleaser pipeline that builds multi-arch cosign-signed images with SBOM/SLSA provenance to GHCR.

**Tech Stack:** Go 1.27 (`net/http`, `log/slog`), Docker (distroless nonroot base), GoReleaser v2, GitHub Actions, kubeconform.

**Spec:** `docs/superpowers/specs/2026-09-07-busyhttp-design.md` (already committed at `6432afa`).

## Global Constraints

- Module path: `github.com/platformfix/busyhttp`; `go.mod` declares `go 1.27` (matches the locally installed `go1.27.1` and `colour`'s own `go.mod`).
- `BUSY_SECONDS` env var (float seconds, default `1`) controls the spin duration; unparseable value is a fatal startup error (log + exit 1).
- `PORT` env var, default `8080`.
- The spin is a real busy-wait (`for time.Now().Before(deadline) {}`), never `time.Sleep` — this is the entire point of the tool.
- `/healthz` always returns `200 ok`.
- No Helm chart, no `kubernetes/chart/` directory, no chart-publish step anywhere in CI.
- Every file this plan creates that has a direct analog in `platformfix/colour` follows that analog's exact shape/style unless a step below says otherwise.
- Every commit uses `git commit -s` (DCO sign-off is mandatory in every repo per user's global git rules); signing itself is automatic from global git config.
- Working directory for every task: `/private/tmp/claude-501/-Users-steve-wade--agenc-missions-97446ce4-5135-42c0-b21b-75322bda6001-agent/875c1525-566d-42e7-af49-45d318801e2d/scratchpad/busyhttp` (a clone of `platformfix/busyhttp`, already initialized with `.beads/`, and already containing the committed design spec). Tracking bead: `busyhttp-4jh` (epic, `in_progress`); its child `busyhttp-4jh.1` tracks deleting the design spec after the first release (Task 9 closes it).

---

### Task 1: Go module + handler package (TDD)

**Files:**
- Create: `go.mod`
- Create: `internal/busyhttp/handler.go`
- Test: `internal/busyhttp/handler_test.go`

**Interfaces:**
- Produces: `busyhttp.NewHandler(duration time.Duration) http.HandlerFunc` and `busyhttp.Healthz(w http.ResponseWriter, r *http.Request)` — both consumed by Task 2's `main.go`.

- [ ] **Step 1: Create `go.mod`**

```
module github.com/platformfix/busyhttp

go 1.27
```

- [ ] **Step 2: Write the failing tests**

`internal/busyhttp/handler_test.go`:

```go
package busyhttp

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewHandler_BusySpinsForDuration(t *testing.T) {
	duration := 20 * time.Millisecond
	handler := NewHandler(duration)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	start := time.Now()
	handler(rec, req)
	elapsed := time.Since(start)

	if elapsed < duration {
		t.Fatalf("handler returned after %s, want at least %s", elapsed, duration)
	}

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}

	want := "I've been busy for 0.02s.\n"
	if got := rec.Body.String(); got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}

func TestNewHandler_WholeSecondFormatsWithoutDecimal(t *testing.T) {
	handler := NewHandler(time.Second)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	want := "I've been busy for 1s.\n"
	if got := rec.Body.String(); got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}

func TestHealthz(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	Healthz(rec, req)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if got, want := rec.Body.String(), "ok"; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./... -v`
Expected: FAIL — `internal/busyhttp` package doesn't exist yet (`no Go files` or `NewHandler undefined`).

- [ ] **Step 4: Write the implementation**

`internal/busyhttp/handler.go`:

```go
// Package busyhttp serves an HTTP handler that burns CPU for a configurable
// duration on every request — a demo load generator for Kubernetes
// HPA/autoscaling exercises.
package busyhttp

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// NewHandler returns a handler that busy-spins the CPU for duration before
// responding. The spin is a literal deadline-polling loop, not time.Sleep:
// the whole point of this tool is that a request shows up as real CPU load,
// which an HPA can react to and time.Sleep would not produce.
func NewHandler(duration time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		deadline := time.Now().Add(duration)
		for time.Now().Before(deadline) {
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintf(w, "I've been busy for %ss.\n", formatSeconds(duration))

		slog.Info("request", "remote_addr", r.RemoteAddr, "method", r.Method, "path", r.URL.String(), "busy_seconds", duration.Seconds())
	}
}

// formatSeconds renders a duration as a plain seconds value, trimming a
// trailing ".0" so "1s" prints as "1" rather than "1.0".
func formatSeconds(d time.Duration) string {
	seconds := d.Seconds()
	if seconds == float64(int64(seconds)) {
		return fmt.Sprintf("%d", int64(seconds))
	}
	return fmt.Sprintf("%g", seconds)
}

// Healthz reports liveness/readiness for Kubernetes probes.
func Healthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./... -v`
Expected: PASS (all three tests)

- [ ] **Step 6: Commit**

```bash
git add go.mod internal/busyhttp/handler.go internal/busyhttp/handler_test.go
git commit -s -m "feat: add busyhttp handler with configurable CPU-burn duration"
git push
```

---

### Task 2: `cmd/busyhttp/main.go` wiring

**Files:**
- Create: `cmd/busyhttp/main.go`

**Interfaces:**
- Consumes: `busyhttp.NewHandler(time.Duration) http.HandlerFunc`, `busyhttp.Healthz` from Task 1.

- [ ] **Step 1: Write `cmd/busyhttp/main.go`**

```go
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/platformfix/busyhttp/internal/busyhttp"
)

// busySeconds reads BUSY_SECONDS as a float number of seconds, defaulting
// to 1s. An unparseable value is a fatal startup error rather than a
// silent fallback: it's an external input worth validating at the
// boundary, and a demo tool that silently ignores its own configuration
// knob is worse than one that refuses to start.
func busySeconds() time.Duration {
	raw := os.Getenv("BUSY_SECONDS")
	if raw == "" {
		return time.Second
	}
	seconds, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		slog.Error("invalid BUSY_SECONDS", "value", raw, "error", err)
		os.Exit(1)
	}
	return time.Duration(seconds * float64(time.Second))
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	duration := busySeconds()

	mux := http.NewServeMux()
	mux.HandleFunc("/", busyhttp.NewHandler(duration))
	mux.HandleFunc("/healthz", busyhttp.Healthz)

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("starting server", "port", port, "busy_seconds", duration.Seconds())
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 2: Build and smoke-test manually**

Run: `go build -o /tmp/busyhttp-smoketest ./cmd/busyhttp && BUSY_SECONDS=0.2 PORT=8091 /tmp/busyhttp-smoketest &`
Then: `sleep 1 && time curl -sf http://localhost:8091/ && curl -sf http://localhost:8091/healthz && kill %1`
Expected: the `curl /` call takes ≳0.2s and prints `I've been busy for 0.2s.`; `/healthz` prints `ok`.

- [ ] **Step 3: Run `go vet`**

Run: `go vet ./...`
Expected: no output (clean).

- [ ] **Step 4: Commit**

```bash
git add cmd/busyhttp/main.go
git commit -s -m "feat: wire up busyhttp server entrypoint"
git push
```

---

### Task 3: Dockerfile + repo hygiene configs

**Files:**
- Create: `Dockerfile`
- Create: `.gitignore`
- Create: `.golangci.yml`

- [ ] **Step 1: Write `Dockerfile`**

```dockerfile
# syntax=docker/dockerfile:1
FROM gcr.io/distroless/static-debian12:nonroot
COPY busyhttp /busyhttp
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/busyhttp"]
```

- [ ] **Step 2: Write `.gitignore`**

```
/busyhttp
/dist/
*.test
.DS_Store
```

- [ ] **Step 3: Write `.golangci.yml`**

```yaml
# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Platform Fix
#
# golangci-lint v2 config. Default linter set (errcheck, govet, ineffassign,
# staticcheck, unused) with the house idioms excluded — deferred Close and
# best-effort Fprint to std streams are intentionally unchecked.
version: "2"

linters:
  settings:
    errcheck:
      exclude-functions:
        - (io.Closer).Close
        - (*os.File).Close
        - fmt.Fprint
        - fmt.Fprintf
        - fmt.Fprintln

  exclusions:
    generated: lax
    presets:
      - std-error-handling
    rules:
      - path: _test\.go
        linters:
          - errcheck
          - staticcheck
          - govet
```

- [ ] **Step 4: Build the binary and the image, verify it runs**

Run:
```bash
CGO_ENABLED=0 go build -o busyhttp ./cmd/busyhttp
docker build -t busyhttp:dev .
docker run -d --rm -p 8092:8080 -e BUSY_SECONDS=0.2 --name busyhttp-dev busyhttp:dev
sleep 1
curl -sf http://localhost:8092/
curl -sf http://localhost:8092/healthz
docker stop busyhttp-dev
rm busyhttp
```
Expected: the `/` response is `I've been busy for 0.2s.`, `/healthz` is `ok`, container stops cleanly.

- [ ] **Step 5: Run golangci-lint and hadolint**

Run: `golangci-lint run ./...`
Expected: no issues.

Run: `hadolint Dockerfile`
Expected: no issues.

- [ ] **Step 6: Commit**

```bash
git add Dockerfile .gitignore .golangci.yml
git commit -s -m "build: add Dockerfile and lint/gitignore configs"
git push
```

---

### Task 4: Kubernetes manifests

**Files:**
- Create: `kubernetes/deployment.yaml`
- Create: `kubernetes/service.yaml`

- [ ] **Step 1: Write `kubernetes/deployment.yaml`**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: busyhttp
spec:
  replicas: 1
  selector:
    matchLabels:
      app: busyhttp
  template:
    metadata:
      labels:
        app: busyhttp
    spec:
      containers:
        - name: busyhttp
          image: ghcr.io/platformfix/busyhttp:latest
          ports:
            - containerPort: 8080
          resources:
            requests:
              cpu: 200m
            limits:
              cpu: 500m
          livenessProbe:
            httpGet:
              path: /healthz
              port: 8080
          readinessProbe:
            httpGet:
              path: /healthz
              port: 8080
```

- [ ] **Step 2: Write `kubernetes/service.yaml`**

```yaml
apiVersion: v1
kind: Service
metadata:
  name: busyhttp
spec:
  selector:
    app: busyhttp
  ports:
    - port: 80
      targetPort: 8080
```

- [ ] **Step 3: Validate with kubeconform**

Run: `kubeconform -strict kubernetes/*.yaml`
Expected: no output, exit code 0.

- [ ] **Step 4: Commit**

```bash
git add kubernetes/deployment.yaml kubernetes/service.yaml
git commit -s -m "feat: add Kubernetes deployment and service manifests"
git push
```

---

### Task 5: GoReleaser config + local snapshot build

**Files:**
- Create: `.goreleaser.yaml`

- [ ] **Step 1: Write `.goreleaser.yaml`**

```yaml
# SPDX-License-Identifier: MIT
# Copyright (c) 2026 Platform Fix
#
# GoReleaser config (v2). Builds the busyhttp binary, builds and pushes a
# multi-arch (amd64/arm64) image to GHCR, and cosign keyless-signs the
# pushed manifest digest — GitHub's own OIDC identity is the signing
# identity (Fulcio/Rekor), no key material anywhere. release.yml attaches
# the image SBOM and SLSA provenance as separate steps after this runs, and
# holds the GitHub Release as a draft until those land, then publishes it.
version: 2

project_name: busyhttp

before:
  hooks:
    - go mod download

builds:
  - id: busyhttp
    main: ./cmd/busyhttp
    binary: busyhttp
    env:
      - CGO_ENABLED=0
    flags:
      - -trimpath
    ldflags:
      - -s -w
    goos: [linux]
    goarch: [amd64, arm64]

archives:
  - id: busyhttp
    formats: [tar.gz]
    name_template: "busyhttp_{{ .Version }}_{{ .Os }}_{{ .Arch }}"

checksum:
  name_template: checksums.txt
  algorithm: sha256

signs:
  - cmd: cosign
    signature: "${artifact}.sigstore.json"
    args:
      - sign-blob
      - "--bundle=${signature}"
      - "${artifact}"
      - --yes
    artifacts: checksum
    output: true

sboms:
  - id: archive
    artifacts: archive
    cmd: syft
    args: ["$artifact", "--output", "spdx-json=$document"]
    documents:
      - "${artifact}.sbom.spdx.json"

dockers:
  - id: busyhttp-amd64
    goos: linux
    goarch: amd64
    dockerfile: Dockerfile
    image_templates:
      - "ghcr.io/platformfix/busyhttp:{{ .Version }}-amd64"
    use: buildx
    build_flag_templates:
      - "--platform=linux/amd64"
  - id: busyhttp-arm64
    goos: linux
    goarch: arm64
    dockerfile: Dockerfile
    image_templates:
      - "ghcr.io/platformfix/busyhttp:{{ .Version }}-arm64"
    use: buildx
    build_flag_templates:
      - "--platform=linux/arm64"

docker_manifests:
  - name_template: "ghcr.io/platformfix/busyhttp:{{ .Version }}"
    image_templates:
      - "ghcr.io/platformfix/busyhttp:{{ .Version }}-amd64"
      - "ghcr.io/platformfix/busyhttp:{{ .Version }}-arm64"
  - name_template: "ghcr.io/platformfix/busyhttp:latest"
    image_templates:
      - "ghcr.io/platformfix/busyhttp:{{ .Version }}-amd64"
      - "ghcr.io/platformfix/busyhttp:{{ .Version }}-arm64"

docker_signs:
  - cmd: cosign
    artifacts: manifests
    args:
      - "sign"
      - "${artifact}"
      - "--yes"

changelog:
  sort: asc
  groups:
    - title: Features
      regexp: '^.*?feat(\(.+\))??!?:.+$'
      order: 0
    - title: Bug fixes
      regexp: '^.*?fix(\(.+\))??!?:.+$'
      order: 1
    - title: Documentation
      regexp: '^.*?docs(\(.+\))??!?:.+$'
      order: 2
    - title: Other changes
      order: 999
  filters:
    exclude:
      - '^test(\(.+\))?:'
      - '^ci(\(.+\))?:'
      - '^chore(\(.+\))?:'
      - '^style(\(.+\))?:'
      - '^build(\(.+\))?:'

snapshot:
  version_template: "{{ incpatch .Version }}-snapshot-{{ .ShortCommit }}"

release:
  github:
    owner: platformfix
    name: busyhttp
  draft: true
```

- [ ] **Step 2: Run a local snapshot build (build-only, no docker/sign/publish)**

Run: `goreleaser build --single-target --snapshot --clean -o busyhttp`
Expected: exits 0, produces a `./busyhttp` binary. Run `./busyhttp &` with `PORT=8093 BUSY_SECONDS=0.1`, curl it, then kill it and `rm busyhttp` to clean up.

- [ ] **Step 3: Commit**

```bash
git add .goreleaser.yaml
git commit -s -m "build: add goreleaser config for multi-arch signed releases"
git push
```

---

### Task 6: Core CI workflows

**Files:**
- Create: `.github/workflows/ci.yml`
- Create: `.github/workflows/lint.yml`
- Create: `.github/workflows/k8s-validate.yml`
- Create: `.github/workflows/commit-lint.yaml`
- Create: `.github/workflows/pr-lint.yml`
- Create: `.github/workflows/scorecard.yml`
- Create: `.github/dependabot.yml`
- Create: `commitlint.config.cjs`

- [ ] **Step 1: Write `.github/workflows/ci.yml`**

```yaml
name: ci

on:
  pull_request:
    branches:
      - main
  push:
    branches:
      - main

concurrency:
  group: ci-${{ github.ref }}
  cancel-in-progress: true

permissions:
  contents: read

jobs:
  test:
    name: build, vet, test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
      - uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
        with:
          go-version-file: go.mod
      - run: go build ./...
      - run: go vet ./...
      - run: go test ./... -v
```

- [ ] **Step 2: Write `.github/workflows/lint.yml`**

```yaml
name: lint

on:
  pull_request:
    branches:
      - main
  push:
    branches:
      - main

concurrency:
  group: lint-${{ github.ref }}
  cancel-in-progress: true

permissions:
  contents: read

jobs:
  hadolint:
    name: hadolint
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1
      - uses: hadolint/hadolint-action@4b5806eb9c6bee4954fc0e0cc3ad6175fc9782c1
        with:
          dockerfile: Dockerfile
          failure-threshold: error
  golangci-lint:
    name: golangci-lint
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1
      - uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
        with:
          go-version-file: go.mod
      - uses: golangci/golangci-lint-action@ba0d7d2ec06a0ea1cb5fa41b2e4a3ab91d21278a # v9.3.0
        with:
          version: v2.13.1
```

- [ ] **Step 3: Write `.github/workflows/k8s-validate.yml`**

```yaml
name: k8s-validate

on:
  pull_request:
    branches:
      - main
  push:
    branches:
      - main

concurrency:
  group: k8s-validate-${{ github.ref }}
  cancel-in-progress: true

permissions:
  contents: read

jobs:
  kubeconform:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
      - name: Install kubeconform
        run: |
          curl -sSL https://github.com/yannh/kubeconform/releases/latest/download/kubeconform-linux-amd64.tar.gz | tar xz kubeconform
          sudo mv kubeconform /usr/local/bin/
      - run: kubeconform -strict kubernetes/*.yaml
```

- [ ] **Step 4: Write `.github/workflows/commit-lint.yaml`**

```yaml
name: commit-lint

on:
  pull_request:
    types:
      - opened
      - edited
      - synchronize

permissions:
  contents: read
  pull-requests: read

jobs:
  commit-lint:
    name: commit-lint
    runs-on: ubuntu-latest
    if: github.event.pull_request.user.login != 'dependabot[bot]'
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v4
        with:
          fetch-depth: 0
      - uses: wagoid/commitlint-github-action@b948419dd99f3fd78a6548d48f94e3df7f6bf3ed # v6
```

- [ ] **Step 5: Write `.github/workflows/pr-lint.yml`**

```yaml
name: pr-lint

on:
  pull_request:
    types:
      - opened
      - edited
      - synchronize

permissions:
  contents: read
  pull-requests: read

jobs:
  lint:
    name: pr-lint
    runs-on: ubuntu-latest
    steps:
      - uses: amannn/action-semantic-pull-request@48f256284bd46cdaab1048c3721360e808335d50 # v5
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

- [ ] **Step 6: Write `.github/workflows/scorecard.yml`**

```yaml
name: Scorecard analysis workflow
on:
  push:
    # Only the default branch is supported.
    branches:
    - main
  schedule:
    # Weekly on Saturdays.
    - cron:  '30 1 * * 6'

permissions: read-all

jobs:
  analysis:
    name: Scorecard analysis
    runs-on: ubuntu-latest
    permissions:
      # Needed for Code scanning upload
      security-events: write
      # Needed for GitHub OIDC token if publish_results is true
      id-token: write

    steps:
      - name: "Checkout code"
        uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v4
        with:
          persist-credentials: false

      - name: "Run analysis"
        uses: ossf/scorecard-action@2d1146689b8cda280b9bc96326124645441f03bc # v2.4.4
        with:
          results_file: results.sarif
          results_format: sarif
          publish_results: true

      - name: "Upload artifact"
        uses: actions/upload-artifact@043fb46d1a93c77aae656e7c1c64a875d1fc6a0a # v7.0.1
        with:
          name: SARIF file
          path: results.sarif
          retention-days: 5

      - name: "Upload to code-scanning"
        uses: github/codeql-action/upload-sarif@ff2f1c621b7f889edc0d3c761ac2e6a3f8cdb0dd # v4.37.7
        with:
          sarif_file: results.sarif
```

- [ ] **Step 7: Write `.github/dependabot.yml`**

```yaml
version: 2
updates:
  - package-ecosystem: "github-actions"
    directory: "/"
    schedule:
      interval: "weekly"
    commit-message:
      prefix: "ci"
  - package-ecosystem: "gomod"
    directory: "/"
    schedule:
      interval: "weekly"
    commit-message:
      prefix: "chore"
```

- [ ] **Step 8: Write `commitlint.config.cjs`**

```js
module.exports = {
  extends: ['@commitlint/config-conventional'],
};
```

- [ ] **Step 9: Commit**

```bash
git add .github/workflows/ci.yml .github/workflows/lint.yml .github/workflows/k8s-validate.yml \
  .github/workflows/commit-lint.yaml .github/workflows/pr-lint.yml .github/workflows/scorecard.yml \
  .github/dependabot.yml commitlint.config.cjs
git commit -s -m "ci: add build, lint, k8s-validate, commit-lint, pr-lint, scorecard workflows"
git push
```

---

### Task 7: e2e and release workflows

**Files:**
- Create: `.github/workflows/e2e.yml`
- Create: `.github/workflows/release.yml`

- [ ] **Step 1: Write `.github/workflows/e2e.yml`**

```yaml
name: e2e

on:
  pull_request:
    branches:
      - main
  push:
    branches:
      - main

concurrency:
  group: e2e-${{ github.ref }}
  cancel-in-progress: true

permissions:
  contents: read

jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1

      - uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
        with:
          go-version-file: go.mod

      - uses: goreleaser/goreleaser-action@f06c13b6b1a9625abc9e6e439d9c05a8f2190e94 # v7.2.3
        with:
          distribution: goreleaser
          version: "~> v2"
          args: build --single-target --snapshot --clean -o busyhttp

      - name: Build the image
        run: docker build -t busyhttp:ci .

      - name: Run the container
        run: |
          docker run -d --rm -p 8081:8080 -e BUSY_SECONDS=0.3 --name busyhttp-e2e busyhttp:ci
          sleep 2

      - name: Verify the request actually burns CPU for ~0.3s
        run: |
          start_ns=$(date +%s%N)
          body=$(curl -sf http://localhost:8081/)
          end_ns=$(date +%s%N)
          elapsed_ns=$((end_ns - start_ns))
          echo "$body"
          echo "elapsed: ${elapsed_ns}ns"
          if [ "$elapsed_ns" -lt 300000000 ]; then
            echo "request returned in ${elapsed_ns}ns, expected at least 300000000ns (0.3s)"
            exit 1
          fi
          echo "$body" | grep -q "0.3" || {
            echo "response did not mention the configured duration"
            exit 1
          }

      - name: Verify /healthz
        run: |
          code=$(curl -s -o /dev/null -w '%{http_code}' http://localhost:8081/healthz)
          [ "$code" = "200" ] || { echo "expected 200 from /healthz, got $code"; exit 1; }

      - name: Show logs on failure
        if: failure()
        run: docker logs busyhttp-e2e || true

      - name: Clean up
        if: always()
        run: docker stop busyhttp-e2e || true
```

- [ ] **Step 2: Write `.github/workflows/release.yml`**

```yaml
name: release

on:
  pull_request:
    branches:
      - main
  push:
    tags:
      - "v*"

permissions:
  contents: read

jobs:
  goreleaser:
    name: goreleaser
    runs-on: ubuntu-latest
    permissions:
      contents: write
      packages: write
      id-token: write
      attestations: write
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          fetch-depth: 0

      - uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
        with:
          go-version-file: go.mod

      - uses: sigstore/cosign-installer@6f9f17788090df1f26f669e9d70d6ae9567deba6 # v4.1.2

      - uses: anchore/sbom-action/download-syft@3ad7283483fc7af8ff2b4ea19663c2d5ca935e26 # v0.24.2

      - uses: docker/setup-qemu-action@96fe6ef7f33517b61c61be40b68a1882f3264fb8 # v4.2.0

      - uses: docker/setup-buildx-action@37fe631027851001ddb9b187196cc803df7f5f0e # v4

      - name: Log in to GHCR
        if: startsWith(github.ref, 'refs/tags/')
        uses: docker/login-action@dbcb813823bdd20940b903addbd779551569679f # v4.6.0
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Validate config on every PR (no publish)
        if: github.event_name == 'pull_request'
        uses: goreleaser/goreleaser-action@f06c13b6b1a9625abc9e6e439d9c05a8f2190e94 # v7.2.3
        with:
          distribution: goreleaser
          version: "~> v2"
          args: release --snapshot --clean --skip=publish,sign,announce

      - name: Release (tags only)
        if: startsWith(github.ref, 'refs/tags/')
        uses: goreleaser/goreleaser-action@f06c13b6b1a9625abc9e6e439d9c05a8f2190e94 # v7.2.3
        with:
          distribution: goreleaser
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}

      - name: Get the published image digest (tags only)
        if: startsWith(github.ref, 'refs/tags/')
        id: digest
        run: |
          VERSION="${GITHUB_REF_NAME#v}"
          DIGEST=$(docker buildx imagetools inspect "ghcr.io/platformfix/busyhttp:${VERSION}" --format '{{json .Manifest}}' | jq -r .digest)
          echo "digest=${DIGEST}" >> "$GITHUB_OUTPUT"

      - name: Attach an SBOM to the image itself (tags only)
        if: startsWith(github.ref, 'refs/tags/')
        run: |
          IMAGE="ghcr.io/platformfix/busyhttp@${{ steps.digest.outputs.digest }}"
          syft "$IMAGE" -o spdx-json > image-sbom.spdx.json
          cosign attest --yes --predicate image-sbom.spdx.json --type spdxjson "$IMAGE"

      - name: Attest build provenance (tags only)
        if: startsWith(github.ref, 'refs/tags/')
        uses: actions/attest-build-provenance@4d101475d8b20a2381f78447822ac1eab6504dd8 # v4.2.2
        with:
          subject-name: ghcr.io/platformfix/busyhttp
          subject-digest: ${{ steps.digest.outputs.digest }}
          push-to-registry: true

      - name: Publish the draft release (tags only)
        if: startsWith(github.ref, 'refs/tags/')
        run: gh release edit "${GITHUB_REF_NAME}" --draft=false --repo platformfix/busyhttp
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

Note: no `azure/setup-helm` step and no "Package and push the Helm chart" step — this repo ships no Helm chart.

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/e2e.yml .github/workflows/release.yml
git commit -s -m "ci: add e2e CPU-burn verification and release workflows"
git push
```

---

### Task 8: Repo docs and final push verification

**Files:**
- Create: `README.md`
- Create: `CONTRIBUTING.md`
- Create: `SECURITY.md`
- Create: `LICENSE`

- [ ] **Step 1: Write `LICENSE`**

```
MIT License

Copyright (c) 2026 Platform Fix

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

- [ ] **Step 2: Write `SECURITY.md`**

```md
# Security Policy

## Supported versions

busyhttp ships one rolling `latest` release; there's no long-term-support branch to track. Security fixes land on `main` and are published as the next image tag.

## Reporting a vulnerability

Please report security issues privately rather than opening a public GitHub issue: use [GitHub's private vulnerability reporting](https://github.com/platformfix/busyhttp/security/advisories/new) for this repository (Security tab → Report a vulnerability).

Include what you'd include in any good bug report: the affected version or commit, what you found, and how to reproduce it. We'll acknowledge new reports within 5 business days and aim to have a fix or mitigation plan within 30 days, depending on severity.
```

- [ ] **Step 3: Write `CONTRIBUTING.md`**

```md
# Contributing

Thanks for considering a contribution to busyhttp.

## Before you start

Open an issue for anything beyond a small fix, so we can agree on the approach before you put time into it.

## Commits and pull requests

- Commit messages must follow [Conventional Commits](https://www.conventionalcommits.org/). This is enforced by CI (`commit-lint`).
- Pull request titles must also follow Conventional Commits. CI (`pr-lint`) checks this too, since a squash merge takes its message from the PR title.
- Keep commits small and focused.

## Testing changes locally

```bash
go test ./...
golangci-lint run ./...
goreleaser build --single-target --snapshot --clean -o busyhttp
docker build -t busyhttp:dev .
kubeconform -strict kubernetes/*.yaml
```

## Reporting issues

Open an issue on GitHub with what you expected, what happened instead, and how to reproduce it.
```

- [ ] **Step 4: Write `README.md`**

```md
# busyhttp

[![ci](https://github.com/platformfix/busyhttp/actions/workflows/ci.yml/badge.svg)](https://github.com/platformfix/busyhttp/actions/workflows/ci.yml)
[![e2e](https://github.com/platformfix/busyhttp/actions/workflows/e2e.yml/badge.svg)](https://github.com/platformfix/busyhttp/actions/workflows/e2e.yml)
[![lint](https://github.com/platformfix/busyhttp/actions/workflows/lint.yml/badge.svg)](https://github.com/platformfix/busyhttp/actions/workflows/lint.yml)
[![k8s-validate](https://github.com/platformfix/busyhttp/actions/workflows/k8s-validate.yml/badge.svg)](https://github.com/platformfix/busyhttp/actions/workflows/k8s-validate.yml)
[![commit-lint](https://github.com/platformfix/busyhttp/actions/workflows/commit-lint.yaml/badge.svg)](https://github.com/platformfix/busyhttp/actions/workflows/commit-lint.yaml)
[![pr-lint](https://github.com/platformfix/busyhttp/actions/workflows/pr-lint.yml/badge.svg)](https://github.com/platformfix/busyhttp/actions/workflows/pr-lint.yml)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/platformfix/busyhttp/badge)](https://scorecard.dev/viewer/?uri=github.com/platformfix/busyhttp)
[![Latest Release](https://img.shields.io/github/v/release/platformfix/busyhttp)](https://github.com/platformfix/busyhttp/releases)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

A trivial HTTP server that burns CPU on every request: a demo load
generator for Kubernetes HPA/autoscaling exercises.

Inspired by [jpetazzo/busyhttp](https://github.com/jpetazzo/busyhttp),
rebuilt in Go for Platform Fix's own Kubernetes workshops — replacing the
upstream `registry.k8s.io/hpa-example` (php-apache) image so every image
used in training is one Platform Fix controls.

## Quickstart

Run the raw demo:

```bash
kubectl apply -f kubernetes/
kubectl port-forward svc/busyhttp 8080:80 &
curl http://localhost:8080/
```

Or just run the container directly:

```bash
docker run -p 8080:8080 ghcr.io/platformfix/busyhttp:latest
curl http://localhost:8080/
```

## How it works

Every request to `/` busy-spins the CPU for `BUSY_SECONDS` (default `1`)
before responding — a literal deadline-polling loop, not `time.Sleep`, so
it shows up as real CPU load an HPA can react to.

- `BUSY_SECONDS`: how long to spin, in seconds (accepts fractions, e.g. `0.5`). Default `1`.
- `PORT`: listen port. Default `8080`.
- `GET /healthz`: liveness/readiness endpoint, always `200 ok`.

## Local development

```bash
go test ./...
golangci-lint run ./...
goreleaser build --single-target --snapshot --clean -o busyhttp
docker build -t busyhttp:dev .
```

## Releases

Tagged releases (`vX.Y.Z`) are built and published by
[goreleaser](.goreleaser.yaml): a multi-arch image pushed to
`ghcr.io/platformfix/busyhttp` under both the version tag and `latest`,
cosign-signed (keyless, via GitHub's OIDC identity), with an SBOM and SLSA
build provenance attached.

## License

[MIT](LICENSE)
```

- [ ] **Step 5: Commit and push**

```bash
git add README.md CONTRIBUTING.md SECURITY.md LICENSE
git commit -s -m "docs: add README, CONTRIBUTING, SECURITY, LICENSE"
git push
```

- [ ] **Step 6: Verify CI is green on GitHub**

Run: `gh run list --repo platformfix/busyhttp --limit 10`
Then for any run still `in_progress`/`queued`: `gh run watch <run-id> --repo platformfix/busyhttp`
Expected: `ci`, `lint`, `e2e`, `k8s-validate`, `pr-lint`-N/A-on-push, `scorecard` all show `completed`/`success` for the latest commit on `main`. If anything fails, read the log (`gh run view <run-id> --repo platformfix/busyhttp --log-failed`), fix, commit, push, and re-check — do not proceed to Task 9 until `ci`, `lint`, `e2e`, and `k8s-validate` are green.

---

### Task 9: Cut and verify the first release, then remove the design spec

**Files:**
- Delete: `docs/superpowers/specs/2026-09-07-busyhttp-design.md`

- [ ] **Step 1: Tag and push the first release**

```bash
git tag -s v0.1.0 -m "v0.1.0"
git push origin v0.1.0
```

(`-s` signs the tag with the same key global git config uses for commit signing.)

- [ ] **Step 2: Watch the release workflow**

Run: `gh run list --repo platformfix/busyhttp --workflow=release.yml --limit 3`
Then: `gh run watch <run-id> --repo platformfix/busyhttp`
Expected: `completed`/`success`.

- [ ] **Step 3: Verify the published artifacts**

Run:
```bash
gh release view v0.1.0 --repo platformfix/busyhttp
docker buildx imagetools inspect ghcr.io/platformfix/busyhttp:0.1.0
docker buildx imagetools inspect ghcr.io/platformfix/busyhttp:latest
```
Expected: the GitHub release is published (not draft), and both the `0.1.0` and `latest` GHCR manifests exist and reference the same multi-arch image — confirms Steve's requirement that a versioned release also produces a versioned image tag.

- [ ] **Step 4: Delete the design spec now that the first release exists**

```bash
git rm docs/superpowers/specs/2026-09-07-busyhttp-design.md
git commit -s -m "chore: remove design spec now that v0.1.0 has shipped"
git push
```

- [ ] **Step 5: Close out tracking**

```bash
bd close busyhttp-4jh.1 --reason="v0.1.0 released; design spec removed"
bd close busyhttp-4jh --reason="platformfix/busyhttp built, CI green, v0.1.0 released"
```
