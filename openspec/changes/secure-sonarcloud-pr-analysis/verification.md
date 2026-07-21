# Verification Notes

## Baseline — 2026-07-21

- The active `main-is-main` ruleset is [repository ruleset 5842755](https://github.com/Ensono/eirctl/rules/5842755). It requires one approving review and the `Lint` and `Test (Linux)` checks (GitHub Actions integration `15368`), but `require_code_owner_review` is `false`.
- GitHub Actions is configured with `allowed_actions: selected`, `sha_pinning_required: true`, and a selected-action allow list. The list includes `actions/*@*` and `sonarsource/sonarqube-scan-action@*`; action names are matched case-insensitively by GitHub's selected-action policy.
- The repository exposes a `SONAR_TOKEN` Actions-secret name. Its value was neither retrieved nor recorded.
- The latest observed failed trusted-main `Lint and Test` run is [29817153665](https://github.com/Ensono/eirctl/actions/runs/29817153665), for revision `f2526393f439bdb99df14aa6deae105465c89ed1`. Its `SonarCloud analysis` job invoked the container-backed `sonar:scanner:cli` task and failed with `java.nio.file.AccessDeniedException: /eirctl/.scannerwork` while the scanner attempted to create its work directory.
- Pull-request runs do not schedule the `sonarcloud` job: its condition is `github.event_name == 'push' && github.ref == 'refs/heads/main'`.

## Action Release and Pin Evidence — 2026-07-21

The selected-action policy permits every action below and requires full SHA pins. Release tags were resolved with GitHub's `releases/latest` endpoint and verified as immutable commit refs with `git ls-remote --tags` (none was an annotated tag requiring a second dereference).

| Action | Latest stable release | Resolved commit SHA |
| --- | --- | --- |
| `actions/checkout` | `v7.0.1` | `3d3c42e5aac5ba805825da76410c181273ba90b1` |
| `actions/setup-go` | `v7.0.0` | `b7ad1dad31e06c5925ef5d2fc7ad053ef454303e` |
| `actions/upload-artifact` | `v7.0.1` | `043fb46d1a93c77aae656e7c1c64a875d1fc6a0a` |
| `actions/download-artifact` | `v8.0.1` | `3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c` |
| `SonarSource/sonarqube-scan-action` | `v8.2.1` | `22918119ff8e1ca75a623e15c8296b6ea4fbe28f` |

The `actions/*@*` and `sonarsource/sonarqube-scan-action@*` selected-action patterns explicitly allow all of these selections. No allow-list change is required.

## Baseline Probe

The immutable CI dependency check now rejects every GitHub Actions `sonar:scanner:cli` invocation and requires the exact reviewed `SonarSource/sonarqube-scan-action` SHA in the trusted-main scan. The replacement was validated locally with `scripts/check-immutable-ci-dependencies.sh`; no GitHub Actions workflow now invokes the failing container task.
