# Contributing

## Development Setup

Run `make help` first to see the local contributor workflow.

You need Go installed locally, plus `golangci-lint` and `govulncheck`; `make test` installs `gotestsum` automatically through the existing `Makefile`.

```bash
make help
```

Use [README.md](README.md) for installation, examples, and architecture context.

## Local Checks

```bash
make build
make test
make integration-test
make lint
make lint-fix
make security-scan
make clean
```

## Pull Request Workflow

Create a branch from main, run local checks, and open PR with a short summary of the change and any relevant context for reviewers.

Keep [README.md](README.md) adopter-focused by linking contributors back there for installation, examples, and architecture details instead of duplicating that content in this guide.

## Security

For disclosure guidance, reporting channels, and expectations, see [SECURITY.md](SECURITY.md).
