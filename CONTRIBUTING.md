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
