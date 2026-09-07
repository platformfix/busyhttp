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
