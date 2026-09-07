# busyhttp design

## Provenance

- Requested by Steve Wade, 2026-09-07, in an AgenC mission session (mission
  `97446ce4-5135-42c0-b21b-75322bda6001`) running against
  `platformfix/second-brain`.
- Behavior reference: [jpetazzo/busyhttp](https://github.com/jpetazzo/busyhttp)
  — a Flask app whose `/` route busy-spins for exactly 1 second, then
  responds. `busyhttp.py` (10 lines) and its `Dockerfile` were read in full
  before this design was written.
- Engineering-standard reference: [platformfix/colour](https://github.com/platformfix/colour)
  — read in full (all Go source, all CI workflows, `.goreleaser.yaml`,
  Dockerfile, `CONTRIBUTING.md`, `SECURITY.md`, `LICENSE`, lint configs,
  Helm chart) to derive the standard this repo replicates.
- Design approved in chat, including the follow-up requirement that
  versioned releases produce versioned Docker image tags (already native to
  `colour`'s `.goreleaser.yaml` `docker_manifests` block — see below).

## Purpose

A CPU-burning HTTP demo server for Platform Fix's own Kubernetes workshops.
It replaces the upstream `registry.k8s.io/hpa-example` (php-apache) image
currently used in `k8s-workshop-labs` Lab 3 ("Scaling and Storage") for the
HPA/autoscaling exercise, so Steve controls every image used in his
training rather than depending on an external one. **Wiring the swap into
`k8s-workshop-labs` itself is out of scope for this repo** — that's a
separate follow-up in a different repo.

## Application behavior

- `GET /`: busy-spins the CPU for a configurable duration, then responds
  `"I've been busy for Ns.\n"` (N = the duration actually used, formatted
  as seconds). The spin is a literal `for time.Now().Before(deadline) {}`
  loop — not `time.Sleep` — matching the original's actual CPU-burning
  mechanic (this is the entire point of the tool: it must show up as real
  CPU load for an HPA to react to).
- `GET /healthz`: returns `200 ok` for liveness/readiness probes. Not
  present in the original or in php-apache; added because `colour` has the
  same convention and it costs nothing.
- Any other path/method: default `http.ServeMux` 404.
- Env vars:
  - `PORT` — listen port, default `8080` (matches `colour`).
  - `BUSY_SECONDS` — spin duration in seconds, parsed as `float64` (so
    `0.3` is valid, for fast CI runs and flexible demos), default `1`. If
    set but unparseable, the process logs and exits 1 at startup — an
    external input validated at the boundary, unlike `colour`'s `PORT`
    which is simply handed to `net.Listen` unvalidated (a bad `PORT` value
    fails just as loudly there, so the same "fail fast at the boundary"
    outcome holds even though `colour` doesn't explicitly parse it).
- Structured `slog` logging per request (method, path, remote addr,
  duration spun) and graceful shutdown on SIGTERM/interrupt — copied from
  `colour/cmd/colour/main.go`'s pattern verbatim (context + signal.NotifyContext
  + 10s shutdown timeout).

## Package layout

```
cmd/busyhttp/main.go          # wiring: env parsing, mux, server, shutdown
internal/busyhttp/handler.go  # NewHandler(d time.Duration) http.HandlerFunc, Healthz
internal/busyhttp/handler_test.go
go.mod                        # module github.com/platformfix/busyhttp, go 1.27
```

`NewHandler` takes the spin duration as a constructor argument (rather than
a package-level constant, which is `colour.Handler`'s style) because,
unlike `colour`, this handler has a runtime-configurable knob that tests
need to override with a short duration (a few ms) to stay fast.

## Kubernetes manifests

`kubernetes/deployment.yaml` + `kubernetes/service.yaml` — a single demo,
not `colour`'s blue/green split (busyhttp has no multi-variant need).

- `requests.cpu: 200m` / `limits.cpu: 500m` — copied from the current Lab 3
  php-apache deployment, so the existing
  `kubectl autoscale --cpu-percent=50 --min=1 --max=10` exercise keeps
  working unchanged once the image is swapped in.
- Label `app: busyhttp`, container port `8080`, Service `port: 80 →
  targetPort: 8080` (matches `colour`'s Service convention).
- Liveness/readiness probes on `/healthz`.
- **No Helm chart** — explicitly out of scope per Steve's answer ("everything
  in colour repo apart from a helm chart").

## CI / release pipeline

Full `colour` stack, minus everything Helm-shaped:

| File | Behavior |
|---|---|
| `.github/workflows/ci.yml` | `go build`, `go vet`, `go test -v` |
| `.github/workflows/lint.yml` | hadolint on `Dockerfile` + golangci-lint |
| `.github/workflows/k8s-validate.yml` | kubeconform only, against `kubernetes/*.yaml` (no helm-lint/helm-template job — there's no chart) |
| `.github/workflows/e2e.yml` | goreleaser snapshot build → `docker build` → run with `BUSY_SECONDS=0.3` → assert the request takes ≳0.3s (proves the CPU-burn mechanic actually works, not just that the server responds) → assert `/healthz` is 200 |
| `.github/workflows/commit-lint.yaml` | Conventional Commits on PR commits, Dependabot exempted |
| `.github/workflows/pr-lint.yml` | Conventional Commits on the PR title |
| `.github/workflows/scorecard.yml` | OpenSSF Scorecard, weekly + on push to main |
| `.github/workflows/release.yml` | goreleaser release on `v*` tags: multi-arch (amd64/arm64) build, cosign keyless-sign, SBOM (syft), SLSA build provenance attestation, GHCR image push — **with the Helm package/push step removed** |
| `.github/dependabot.yml` | `gomod` + `github-actions`, weekly |

`.goreleaser.yaml` is copied near-verbatim from `colour` (renamed to
`busyhttp`/`ghcr.io/platformfix/busyhttp`): its `docker_manifests` block
already produces **both** a `{{ .Version }}`-tagged manifest and a
`latest`-tagged one from the same per-arch images, so a versioned release
(`v0.1.0`) yields `ghcr.io/platformfix/busyhttp:0.1.0` alongside `:latest`
with no extra work — this satisfies Steve's requirement that versioned
releases produce versioned image tags.

Repo-standard files copied and adapted (name/description changed, content
otherwise unchanged in shape): `Dockerfile` (distroless nonroot, same
pattern), `README.md` (Helm-install section dropped), `CONTRIBUTING.md`
(helm-lint/template lines dropped from the local-testing snippet, kubeconform
line kept), `SECURITY.md`, `LICENSE` (MIT, Copyright (c) 2026 Platform Fix),
`.golangci.yml`, `.gitignore`, `commitlint.config.cjs`.

## Testing

- `handler_test.go`: constructs the handler with a few-millisecond duration,
  asserts elapsed time is at least that duration and the response body
  matches; a separate test covers `/healthz`.
- `go vet` and golangci-lint cover static checks.
- The e2e workflow (above) is the only check that exercises the real
  CPU-burn behavior end-to-end, at container level.

## Explicitly out of scope

- Swapping `k8s-workshop-labs` Lab 3 over to this image — separate repo,
  separate piece of work, to be picked up as its own follow-up once this
  repo has a tagged release to point at.
- A Helm chart — declined explicitly.
- Any HPA object itself — that belongs to the workshop lab exercise, not to
  the demo application's own repo (mirrors how `colour` ships no Deployment
  strategy/rollout objects beyond the plain demo Deployment either).
