# Contributing to eirctl

Thank you for helping improve eirctl. This guide explains how to provide feedback and submit changes.

## Feedback, bugs, and enhancements

Use [GitHub Issues](https://github.com/Ensono/eirctl/issues) to report bugs or suggest enhancements. Include enough context for others to understand and reproduce the problem, such as the eirctl version, operating system, configuration, expected behaviour, and actual behaviour.

Do **not** report suspected security vulnerabilities in a public issue. Follow the private reporting process in [SECURITY.md](SECURITY.md) instead.

## Before you start

Read the project [README](README.md) and the canonical documentation entry point at [docs/index.adoc](docs/index.adoc). Local builds and validation use Go 1.26.6; the documentation build also requires Docker Engine or a Docker-API-compatible OCI runtime, as described in [docs/building.adoc](docs/building.adoc).

## Submit a pull request

1. Create a focused change that addresses an issue or clearly explains why an issue is not needed.
2. Use a [Conventional Commits](https://www.conventionalcommits.org/) style title for commits and the pull request.
3. Add or update tests when the change affects behaviour, or explain in the pull request why tests are not applicable.
4. Update documentation, examples, schemas, or release notes when applicable.
5. Complete the pull request template, including the change rationale, implementation details, evidence, and instructions for testing.

The pull request template records the repository's acceptance checklist. It requires contributors to run relevant checks, avoid exposing secrets or weakening security controls, and confirm that the change meets the project's coding standards.

## Validate your change

Run the checks relevant to the files you changed from the repository root. The pull-request workflow uses the following commands:

```shell
# Run the standard Go test suite
go test ./...

# Run the CI-equivalent lint and test pipelines
go run -race cmd/main.go run lints
go run cmd/main.go run pipeline gha:unit:test --verbose

# Build and validate documentation when documentation changes
go run cmd/main.go run build:docs --verbose
```

The test pipeline generates coverage and JUnit report files under `.coverage/`. The documentation build writes generated output under `.eirctl/outputs/docs/`; do not commit generated documentation output.

## Community standards

All project participation is governed by the [Code of Conduct](CODE_OF_CONDUCT.md). Report conduct concerns using the contact details in that document.
