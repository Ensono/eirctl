# Implementation Evidence

## Local validation — 2026-08-19

### Passed

- `go test ./scripts/check-workflow-policy`
- `bash scripts/check-workflow-security.sh`
- `bash scripts/check-immutable-ci-dependencies.sh`
- `bash scripts/check-immutable-ci-dependencies_test.sh`
- `bash scripts/check-go-version.sh`
- `bash scripts/check-release-versioning_test.sh`
- `go test ./internal/schema`
- `actionlint .github/workflows/debug-build.yml .github/workflows/debug-build-request.yml .github/workflows/publish-debug-release.yml`
- Updated archive local-link check: 0 missing links across the two archives and current CI security documentation.
- `node --test scripts/validate-docs-output.test.mjs`: 6 tests passed.
- `DOCKER_HOST=unix:///run/user/1000/podman/podman.sock go run cmd/main.go run build:docs --verbose`: HTML, PDF, and rendered-output validation completed successfully.
- `go test ./...`
- `go vet ./...`
- `openspec validate make-debug-build-cache-safe --type change --strict`: change is valid.
- `pre-commit run --all-files`: Gitleaks, Go vet, Go build, and Go test hooks passed.

### Unrelated pre-existing findings retained

These broad checks were run and were not weakened or bypassed:

- Full-repository `actionlint` reports the existing unknown `ubuntu-26.04` labels in `.github/workflows/pr.yml` and the existing SC2086 finding in `.github/workflows/release.yml`. The three changed debug workflows pass Actionlint.
- `staticcheck ./...` reports 15 existing unused-test-helper/redundant-return findings in unchanged test files.
- `gosec ./...` reports 21 existing findings in unchanged runtime/materializer code, plus the existing false-positive G101 matches on the literal policy field names `persist-credentials` and `${{ secrets.SONAR_TOKEN }}` in `scripts/check-workflow-policy/main.go`. No newly added schema field or cache-authority logic is identified.
- `govulncheck ./...` reports GO-2026-4887 and GO-2026-4883 through the existing `github.com/docker/docker@v28.5.2+incompatible` dependency. The advisory reports no fixed version. Four additional required-module vulnerabilities are not called by this code.

## Live GitHub acceptance

### Pull request and static analysis — 2026-08-19

- Pushed commit: `a091d74` on non-default branch `fix/vulns-2026-08-19`.
- Pull request: https://github.com/Ensono/eirctl/pull/150
- CodeQL run: https://github.com/Ensono/eirctl/actions/runs/32244947620
- `Analyze (actions)` passed for the pushed change, along with the Go and JavaScript/TypeScript analyses. The code-scanning check completed successfully without a replacement Actions alert on the pull-request revision.
- Existing alert 23 remains open against `refs/heads/main` at commit `a767ca4c1e847457d5d7f6155c4023d96886726a`; its most recent instance reports CodeQL `2.26.3`, `.github/workflows/debug-build.yml`, and the predecessor `workflow_dispatch` path. It has not been dismissed. GitHub will not close that default-branch alert until a fixing revision reaches the default branch.
- The protected candidate-policy run failed because the trusted checker executes from `main` and still expects the predecessor `request` job with dispatch authority. This proves the policy and target topology cannot be migrated atomically in one PR under the current protected-base enforcement.

### Stacked transition preparation

- Installed official `github/gh-stack` extension with GitHub CLI 2.97.0.
- Created bottom branch `fix/debug-build-policy-transition` from `origin/main`.
- The bottom branch changes no active file under `.github/workflows`; target workflows exist only as policy test fixtures.
- Transition policy accepts the legacy request, builder, and publisher only when all three raw workflow SHA-256 digests match the reviewed `origin/main` files and the builder has no additional effective caller. Any changed legacy file falls back to normal fail-closed cache-authority analysis.
- The target topology requires exact trigger sets and exactly one effective `issue_comment` caller from `.github/workflows/debug-build-request.yml`.
- Simulated the authoritative boundary by running the unmodified `origin/main` checker from detached worktree `/tmp/eirctl-origin-main` against bottom-branch candidate workflow/configuration data; it passed.
- Bottom-branch focused tests, full Go tests, Go vet, pre-commit, workflow security, and strict OpenSpec validation passed locally.
- Submitted native GitHub stack #152 with the official `github/gh-stack` extension:
  - bottom policy transition: PR 151, https://github.com/Ensono/eirctl/pull/151, `fix/debug-build-policy-transition` → `main`, initial commit `18daa14`;
  - top strict workflow cutover: PR 150, https://github.com/Ensono/eirctl/pull/150, `fix/vulns-2026-08-19` → `fix/debug-build-policy-transition`, tip `1152411` at initial stack submission.
- PR 151 GitHub Actions checks passed, including protected policy, CodeQL Actions, lint, tests, and documentation. SonarCloud reported a failed external quality gate caused by 13 critical maintainability findings (code duplication and cognitive complexity), with 0 new bugs and 0 new vulnerabilities; those findings were fixed rather than bypassed.

### Live-flow blocker

`issue_comment` workflow definitions are loaded from the default branch. Before merge, an exact `/build-debug` comment on PR 150 would execute the old default-branch dispatch topology, not this branch's reusable workflow. The unsafe predecessor was therefore not triggered for testing. The protected publisher likewise cannot prove acceptance of the new request-run identity until the new definitions exist on the default branch.

Tasks 6.3–6.8 remain pending. The implementation needs a staged migration: first merge a policy-compatible transition that allows the reviewed target topology while the old workflows remain, then deliver the workflow cutover and strict rejection of the predecessor. After the cutover reaches `main`, record the authorized and negative request runs, caller/called job identity, permissions/runner, artifact IDs/names, publisher validation, cleanup, final CodeQL alert state, and repository prerequisites here.
