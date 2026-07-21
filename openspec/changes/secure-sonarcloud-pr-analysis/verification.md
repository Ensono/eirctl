# Verification Notes

## Baseline — 2026-07-21

- The active `main-is-main` ruleset is [repository ruleset 5842755](https://github.com/Ensono/eirctl/rules/5842755). It requires one approving review and the `Lint` and `Test (Linux)` checks (GitHub Actions integration `15368`), but `require_code_owner_review` is `false`.
- GitHub Actions is configured with `allowed_actions: selected`, `sha_pinning_required: true`, and a selected-action allow list. The list includes `actions/*@*` and `sonarsource/sonarqube-scan-action@*`; action names are matched case-insensitively by GitHub's selected-action policy.
- The repository exposes a `SONAR_TOKEN` Actions-secret name. Its value was neither retrieved nor recorded.
- The latest observed failed trusted-main `Lint and Test` run is [29817153665](https://github.com/Ensono/eirctl/actions/runs/29817153665), for revision `f2526393f439bdb99df14aa6deae105465c89ed1`. Its `SonarCloud analysis` job invoked the container-backed `sonar:scanner:cli` task and failed with `java.nio.file.AccessDeniedException: /eirctl/.scannerwork` while the scanner attempted to create its work directory.
- Pull-request runs do not schedule the `sonarcloud` job: its condition is `github.event_name == 'push' && github.ref == 'refs/heads/main'`.

## Live PR CodeQL Gate — 2026-07-21

- Pull request [#114](https://github.com/Ensono/eirctl/pull/114) at revision `90d745cf74d5e06132535a786ec2fc7fcb992f5a` has open CodeQL alert [#21](https://github.com/Ensono/eirctl/security/code-scanning/21): `actions/untrusted-checkout/high` (`high` severity), reported against `.github/workflows/trusted-sonarcloud-pr.yml`. The alert is open and has not been dismissed.
- The active [`main-is-main` ruleset](https://github.com/Ensono/eirctl/rules/5842755) enforces CodeQL `high_or_higher` security alerts with no bypass actors. The same live ruleset now requires code-owner review, one approving review, last-push approval, and resolved review threads.
- The rejected design checks out pull-request-controlled source in a privileged `workflow_run` job, even though the ref is a full immutable SHA. The replacement must remove privileged pull-request checkout entirely and materialize bounded passive source through the protected Git Data API helper.
- Acceptance requires resolving the alert through the no-checkout implementation. Dismissal, suppression, lowering the threshold, or bypassing the ruleset is not permitted.

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

## Git Data API Source Bounds — 2026-07-21

GitHub's recursive Git tree API was measured at pull-request revision `90d745cf74d5e06132535a786ec2fc7fcb992f5a` (root tree `2e3e93d25d374df458494eb0bc47bdd731773840`). The response was complete (`truncated: false`).

| Measure | Observed baseline | Protected bound | Headroom |
| --- | ---: | ---: | ---: |
| Recursive tree entries | 273 | 384 | 40.7% |
| Selected regular `.go` files | 102 | 160 | 56.9% |
| Longest repository path | 101 bytes | 160 bytes | 58.4% |
| Largest selected `.go` blob | 50,208 bytes | 131,072 bytes (128 KiB) | 161.0% |
| Aggregate selected `.go` bytes | 626,111 bytes | 1,048,576 bytes (1 MiB) | 67.5% |

These are protected constants, not workflow inputs. They are the smallest practical rounded bounds above the current repository baseline while leaving explicit growth headroom. Increasing one requires CODEOWNERS review, updated measurements, hostile-boundary tests, and a corresponding policy update.

## Local No-Checkout Validation — 2026-07-21

The protected Git Data API helper and policy were validated with:

- `go test ./scripts/materialize-sonar-source` (hostile tree, mode, path, bound, blob, write, and superseded-head fixtures);
- `go test ./scripts/check-workflow-policy` (same-repository/fork topology and bypass fixtures);
- `bash scripts/check-immutable-ci-dependencies_test.sh` and `bash scripts/check-immutable-ci-dependencies.sh`;
- `bash scripts/check-codeowners.sh` and `bash scripts/check-workflow-security.sh`;
- `actionlint`;
- `go test ./...`;
- `openspec validate secure-sonarcloud-pr-analysis`; and
- `git diff --check`.

All passed. An independent security review found and prompted fixes for revision-specific concurrency (which could not cancel stale runs) and report download by name rather than the verified artifact ID. The workflow now uses one concurrency group per PR and downloads through `steps.provenance.outputs.artifact-id`; regression fixtures reject both former forms.
