## 1. Split the Debug-Build Trust Domains

- [x] 1.1 Refactor the `issue_comment` path into a command broker that accepts only exact `/build-debug` requests on pull requests, verifies `write`/`maintain`/`admin` repository permission, serializes requests per PR, and performs no checkout or PR-controlled execution.
- [x] 1.2 Have the broker retrigger a dedicated debug-build label with only the narrow PR/issue write permission needed for that signal.
- [x] 1.3 Convert the debug builder to a `pull_request` `labeled` workflow gated on the dedicated label, with `contents: read`, no environment or secret references, and checkout of the event's immutable PR head SHA.
- [x] 1.4 Disable automatic dependency caching in the debug builder as defense in depth and retain artifact provenance for repository, PR, head SHA, workflow run, run attempt, and version.

## 2. Preserve Trusted Publication

- [x] 2.1 Update debug-publication validation to accept only successful runs of the exact debug-build workflow under the `pull_request` event and to verify association with the requested PR.
- [x] 2.2 Retain fail-closed checks for repository, current PR head SHA, run attempt, artifact identity, and provenance before the job receives `contents: write`.
- [x] 2.3 Verify the publisher does not check out PR code or execute, source, install, or interpret downloaded artifacts, and keep publication gated by the protected `debug-release` environment.

## 3. Enforce the Boundary Locally

- [x] 3.1 Extend workflow-policy parsing to classify privileged/default-branch triggers and reject checkout plus execution of PR-controlled content in those workflows.
- [x] 3.2 Add positive fixtures for the brokered `issue_comment` → `pull_request/labeled` → `workflow_dispatch` topology and negative fixtures covering privileged checkout/execution and cache-poisoning variants.
- [x] 3.3 Add repository-specific checks for the broker's no-checkout rule, builder event/label/SHA/permissions/environment rules, and publisher no-checkout rule.
- [x] 3.4 Update `docs/ci-security.md` with the three-domain flow, repeated-request behavior, label lifecycle, required `debug-release` protections, and operator instructions.

## 4. Validate and Re-enumerate Security Findings

- [x] 4.1 Run workflow policy/security checks, workflow linting, Go tests, `go vet`, `staticcheck`, `gosec`, Go version checks, vulnerability scanning, and `openspec validate fix-debug-build-cache-poisoning --type change --strict`; require scoped checks to pass and record unrelated pre-existing analyzer findings as a separate baseline.

### Validation Baseline

- `go vet`: existing unkeyed `container.InspectResponse` test literal in `runner/executor_container_test.go`.
- `staticcheck`: existing unused test helpers/fields, a redundant test `return`, and an unused error assignment outside the debug-build workflow changes.
- `gosec`: existing findings, including `G115` exit-status conversions in `runner/executor_container.go`.
- `govulncheck`: existing `GO-2026-5746` call paths through `github.com/docker/docker`.

These findings are tracked separately; this change must not worsen them.
- [x] 4.2 Confirm the branch still selects `golang.org/x/crypto v0.52.0` and `golang.org/x/net v0.55.0` and introduces no dependency regression.
- [x] 4.3 Push the change and use `gh` to verify PR CodeQL alert 10 closes without dismissal, the CodeQL check passes, and no replacement PR code-scanning alert appears.
- [ ] 4.4 After merge, use `gh` to confirm the 13 `x/crypto` and one `x/net` Dependabot alerts close on the default branch and document any residual repository-wide findings separately.
