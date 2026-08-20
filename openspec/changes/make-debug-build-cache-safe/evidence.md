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
- PR 151 GitHub Actions checks passed, including protected policy, CodeQL Actions, lint, tests, and documentation. SonarCloud initially reported 13 critical maintainability findings (code duplication and cognitive complexity), with 0 new bugs and 0 new vulnerabilities. After refactoring, the [SonarCloud quality gate](https://sonarcloud.io/dashboard?id=Ensono_eirctl&pullRequest=151) passed with 87.0% new coverage, 0.0% duplication, and zero new code-smell severity.
- A staging run exposed that every job calling the Pull Requests API also needs explicit `pull-requests: read`. The production stack now grants that read-only metadata permission to the reusable caller/builder, clean finalizer, and publisher validator; no write permission or secret was added.

### Staging end-to-end acceptance — 2026-08-19

Repository: https://github.com/Ensono-Staging/eirctl-test (internal). No production secret or credential was copied. Actions were disabled during default-branch bootstrap and re-enabled afterward. During the first acceptance pass, the staging organization allowlist excluded `gittools/actions`, so staging alone used a fixed `0.0.0-staging` output and a guarded release step. After the organization allowlist was updated, the exact unmodified production tree was synchronized and revalidated as recorded below.

The GitHub cache-policy sources were rechecked on 2026-08-19: the [June 26, 2026 changelog](https://github.blog/changelog/2026-06-26-read-only-actions-cache-for-untrusted-triggers/) and [cache access documentation](https://docs.github.com/en/actions/using-workflows/caching-dependencies-to-speed-up-workflows#cache-access-for-low-trust-workflow-triggers). The staging jobs used GitHub Actions runner `2.336.0`, `ubuntu-24.04`, image release `ubuntu24/20260810.271`.

- Test PR: https://github.com/Ensono-Staging/eirctl-test/pull/2. Initial immutable head: `0adce4c2c2bbe08b1d81fbf1fac9d7ab36713b71`.
- Non-exact comment: https://github.com/Ensono-Staging/eirctl-test/pull/2#issuecomment-5341963524. Run https://github.com/Ensono-Staging/eirctl-test/actions/runs/32251703952 concluded `skipped`; `authorize`, `build`, and `finalize` all had no executed steps.
- The first exact run, https://github.com/Ensono-Staging/eirctl-test/actions/runs/32251746660, proved the missing PR-metadata permission by failing `pulls.get` with `403 Resource not accessible by integration`; checkout and build were skipped. This produced the least-privilege correction above.
- Successful authorized run: https://github.com/Ensono-Staging/eirctl-test/actions/runs/32252378958, path `.github/workflows/debug-build-request.yml`, root event `issue_comment`, run attempt `1`, conclusion `success`. The API recorded the reusable reference `debug-build.yml@eb53b5b300ea45770e9e767f8119ba325797bf0a` from `refs/heads/main`.
- Jobs `authorize`, `build / build`, and `finalize` all succeeded on GitHub-hosted `ubuntu-24.04` runners. Token logs showed authorization with only `pull-requests: read`; builder with `contents: read` and `pull-requests: read`; finalizer with `actions: read`, `contents: read`, and `pull-requests: read`.
- Intermediate artifact ID `9365331278`, name `debug-build-intermediate-32252378958-1`, digest `sha256:9eb6742a8fed16c2d7156dffe88885049e9c8940e254bcfe8e8e78a8a3fabe74`.
- Final artifact ID `9365350515`, name `debug-build-32252378958`, digest `sha256:faaebd532b4bdc70e08724a1cfaf318039863eb5f77e991973eff34571629b9d`. It contained exactly seven non-empty expected binaries plus `debug-build-provenance.json`.
- Provenance bound repository `Ensono-Staging/eirctl-test`, event `issue_comment`, workflow path `.github/workflows/debug-build-request.yml`, PR `2`, commit `0adce4c2c2bbe08b1d81fbf1fac9d7ab36713b71`, run `32252378958`, attempt `1`, intermediate/final names, `finalized_by: finalize`, and staging SemVer `0.0.0-staging`.
- Preliminary correct publisher selection: https://github.com/Ensono-Staging/eirctl-test/actions/runs/32253650040. `validate-build` authenticated the run, artifact, current PR head, and provenance; `publish` stopped in `waiting` at protected environment approval. Cancelling this run correctly avoided a release but recorded deployment `5982555862` as `error`, so it was not treated as clean deployment acceptance and was later deleted.
- Malformed publisher selection: https://github.com/Ensono-Staging/eirctl-test/actions/runs/32253750305 used an all-zero SHA. Validation failed before download/provenance handling; `publish` was skipped.
- Stale-head run: https://github.com/Ensono-Staging/eirctl-test/actions/runs/32253904582. Authorization captured the old head, staging advanced the PR during a deterministic pre-validation window, and builder revalidation failed with `The pull request is not open at the authorized current commit SHA in this repository.` Checkout, setup, build, upload, and finalization were skipped.
- Clean deployment rerun: request https://github.com/Ensono-Staging/eirctl-test/actions/runs/32355050335 rebuilt and finalized head `f1a226e65c8ba403b27051a3fc49d946b2d07324`. Publisher https://github.com/Ensono-Staging/eirctl-test/actions/runs/32355955777 completed both `validate-build` and `publish`; the publish runner downloaded only the validated artifact and rechecked provenance. Staging alone guarded the final release-action step with unset repository variable `STAGING_ENABLE_RELEASE`, so it was skipped rather than creating a release. Deployment `5999855783` completed with status `success`, and the repository still had no release.
- Cleanup: all artifacts from both successful builds were deleted; the obsolete error deployment was deleted; no release existed; the test PR was closed and its branch deleted; the artificial stale-head delay was reverted; Actions were re-enabled.

### Exact production-tree staging revalidation — 2026-08-20

After Ensono-Staging added every referenced action identity to its organization allowlist, staging `main` was replaced with the exact production stack tree at `6cf7d9d6c0667303b7cc2fe0e03b8c8f83ed8674`. `git diff` confirmed zero tree differences and the action audit reported `NOT_ALLOWED_COUNT 0`; no GitVersion or publisher guard adaptation remained.

- Local validation against the exact tree passed: full pre-commit (including Go-version consistency, vet, build, and tests), Actionlint for the three debug workflows, workflow security policy, and strict OpenSpec validation.
- Test PR: https://github.com/Ensono-Staging/eirctl-test/pull/3, immutable head `4b5236316cb806de1be9193850c7aca26cf4e766`.
- Exact authorized request: https://github.com/Ensono-Staging/eirctl-test/actions/runs/32359186129, root event `issue_comment`, run attempt `1`, default-branch workflow SHA `6cf7d9d6c0667303b7cc2fe0e03b8c8f83ed8674`.
- `authorize`, reusable `build / build`, and `finalize` succeeded. The unmodified pinned GitVersion setup and execution actions both ran successfully, followed by exact-SHA checkout, build, intermediate upload, bounded finalization, provenance creation, and immutable final upload.
- Intermediate artifact ID `9403214039`, name `debug-build-intermediate-32359186129-1`, digest `sha256:cc0c259deb1925c29283bcd7a03dc4ab05fd270db80880305cf472fc949d7ad8`.
- Final artifact ID `9403224709`, name `debug-build-32359186129`, digest `sha256:2d92ee6efb351c26d391bbd9e09070c610465e64e7f8d97471b775c95f082795`.
- Exact publisher: https://github.com/Ensono-Staging/eirctl-test/actions/runs/32360353390. Both `validate-build` and `publish` succeeded; the publish runner downloaded only the validated artifact, rechecked provenance, and executed the unmodified pinned release action.
- Deployment `6000625955` completed with status `success`. Prerelease tag `debug-pr-3-4b5236316cb8` contained exactly the seven expected binaries.
- Cleanup: the prerelease and tag, both artifacts, the validation PR, and its branch were deleted. Staging Actions remained enabled, no run required approval, and no release remained. At validation completion, staging `main` matched the production workflow tree at `6cf7d9d`.

### Production live-flow blocker

`issue_comment` workflow definitions are loaded from the default branch. Before merge, an exact `/build-debug` comment on PR 150 would execute the old default-branch dispatch topology, not this branch's reusable workflow. The unsafe predecessor was therefore not triggered for testing. The protected publisher likewise cannot prove acceptance of the new request-run identity until the new definitions exist on the default branch.

Production tasks 6.3–6.5 remain pending. The implementation needs a staged migration: first merge a policy-compatible transition that allows the reviewed target topology while the old workflows remain, then deliver the workflow cutover and strict rejection of the predecessor. After the cutover reaches `main`, record the authorized and negative request runs, caller/called job identity, permissions/runner, artifact IDs/names, publisher validation, cleanup, final CodeQL alert state, and repository prerequisites here.
