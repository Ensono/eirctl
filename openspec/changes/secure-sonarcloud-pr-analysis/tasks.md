## 1. Capture Current Security and Dependency Baselines

- [x] 1.1 Record the current `main-is-main` ruleset, selected-action allow list, SHA-pinning requirement, `SONAR_TOKEN` secret presence, and latest failing/skipped SonarCloud run URLs without exposing secret values.
- [x] 1.2 Query the latest stable releases and immutable commit SHAs for every action introduced or touched by this change, resolve annotated tags to commits, and record release/tag/SHA evidence for review.
- [x] 1.3 Confirm that each selected action is already allowed; if any is not allowed, stop and obtain allow-list approval before adding it.
- [x] 1.4 Add a focused verification fixture or documented probe that reproduces the current container `.scannerwork` permission failure and proves the replacement path no longer uses that failing container invocation in GitHub Actions.
- [x] 1.5 Record the live PR's `actions/untrusted-checkout/high` CodeQL finding, active blocking ruleset threshold, rejected checkout design, and requirement to resolve the alert without dismissal, suppression, or ruleset bypass.

## 2. Protect Security-Sensitive Configuration

- [x] 2.1 Create `.github/CODEOWNERS` entries for `/.github/CODEOWNERS`, `/.github/workflows/**`, and `/sonar-project.properties`, each owned by `@Ensono/digital-tools-maintainers`.
- [x] 2.2 Add automated validation that the required CODEOWNERS paths and owner remain present and that executable workflow and Sonar configuration changes cannot silently lose ownership coverage.
- [x] 2.3 Update CI security documentation to state that CODEOWNERS is merge governance and does not replace runtime isolation, provenance validation, or secret scoping.

## 3. Produce Bounded Untrusted Sonar Reports

- [x] 3.1 Update the Linux pull-request test path to upload a dedicated Sonar reports artifact containing only the expected Go coverage and JUnit report files, with a bounded retention period, deterministic name, and documented root-level downloaded layout after `upload-artifact` strips their common `.coverage` parent.
- [x] 3.2 Ensure the pull-request workflow retains read-only permissions, never references `SONAR_TOKEN`, and does not gain a protected environment or other privileged credential.
- [x] 3.3 Define and test the downloaded artifact contract for root-level `out` and `report-junit.xml` paths, regular-file types, maximum sizes, missing coverage, and rejection of symlinks, special files, traversal paths, directories, or unexpected content.

## 4. Enforce the No-Checkout Trusted Analyzer Policy

- [x] 4.1 Extend the structural workflow model to represent the trusted SonarCloud `workflow_run` topology, protected Git Data API source materializer, tree/blob provenance, source bounds, scanner runtime inputs, secret scope, caches, and execution after passive source is introduced.
- [x] 4.2 Require the expected upstream workflow/event, base and head repositories, current immutable PR identity, exact run/attempt/artifact, complete Git tree, selected blob identities, read-only permissions, no caches, no local actions, and `SONAR_TOKEN` only on the approved pinned scanner step.
- [x] 4.3 Reject every privileged pull-request checkout mechanism, generic untrusted source archive extraction, non-Go materialization, shell/build/package/container execution after materialization, alternate scanner, mutable scanner runtime, disabled signature verification, or cache operation.
- [x] 4.4 Add table-driven policy tests for valid same-repository and fork analyzer structures plus bypasses using checkout aliases, mutable or derived refs, forged head repositories, truncated trees, wrong blob identities, unsafe paths, missing bounds, job/workflow-scoped secrets, untrusted scanner settings, post-materialization commands, caches, alternate actions, and equivalent YAML syntax.
- [x] 4.5 Keep trusted candidate materialization and policy workflow fixtures inspecting the new workflow and trusted Sonar configuration only as data under protected base-branch checker code.

## 5. Implement Bounded API-Based Pull-Request SonarCloud Analysis

- [x] 5.1 Add a default-branch-controlled workflow triggered on completion of `Lint and Test` pull-request runs, with explicit least-privilege permissions and per-pull-request/revision concurrency.
- [x] 5.2 Revise trusted provenance resolution to verify workflow identity, event, base and head repository identity, base branch, pull-request number, full current head SHA, run ID, run attempt, report artifact, commit tree, and selected source-blob identities before any secret-bearing step.
- [x] 5.3 Download only the dedicated report artifact from the verified upstream run/attempt, validate it against the bounded passive-report contract, and fail closed before scanning on any mismatch.
- [x] 5.4 Measure the current repository's recursive tree entries, Go-file count, path lengths, largest Go file, and aggregate Go bytes; record the baseline and select the smallest practical protected bounds with documented headroom.
- [x] 5.5 Implement a protected standard-library Git Data API helper that requires a complete tree, validates canonical paths and entry modes, rejects symlinks/submodules/special entries, retrieves only regular `.go` blobs by SHA, verifies identity and size, enforces every bound, writes exclusive `0644` files under `analysis/source`, and rechecks the current PR head before success.
- [x] 5.6 Replace pull-request `actions/checkout` with the protected source helper, create trusted scanner configuration outside the source root before materialization, materialize no Git metadata or non-Go repository content, and invoke no post-materialization command or action other than the approved scanner.
- [x] 5.7 Force trusted endpoint, organization, project, source, test, report, PR, revision, and quality-gate settings; explicitly pin the scanner CLI version and approved binaries URL, keep signature verification enabled, and scope `SONAR_TOKEN` to the pinned scanner step only.
- [x] 5.8 Preserve the explicit failed-preparation behavior for missing coverage so SonarCloud analysis is never silently skipped.
- [x] 5.9 Add hostile helper tests for truncated trees, symlinks, submodules, unknown modes/types, absolute/traversal/backslash/control-character paths, duplicate normalized paths, excessive path/file/count/aggregate bounds, missing or wrong blobs, short writes, existing destinations, and a superseded PR head.
- [x] 5.10 Add workflow-level assertions proving fork source, scripts, workflows, local actions, dependencies, scanner configuration, containers, Git metadata, and binaries are never materialized or executed and that no privileged cache is restored or saved.

## 6. Repair Trusted Main Analysis and Align Action Versions

- [x] 6.1 Replace the `main` SonarCloud container-task invocation with the same latest stable immutable-SHA-pinned official scan action used by the trusted PR analyzer.
- [x] 6.2 Preserve trusted report generation, full revision metadata, trusted `sonar-project.properties`, `SONAR_TOKEN` step scope, and `sonar.qualitygate.wait=true` for `main` pushes.
- [x] 6.3 Remove GitHub Actions-only Docker scanner environment settings that are no longer required, while leaving unrelated local task-runner behavior out of scope.
- [x] 6.4 Update other occurrences of action families touched by this change where required for consistent latest stable versions, full SHA pins, readable comments, and avoidance of immediate grouped Dependabot churn.
- [x] 6.5 Run the immutable CI dependency check and verify every introduced or updated action remains inside the repository selected-action policy.

## 7. Document and Validate the Revised Trust Boundary

- [x] 7.1 Update `docs/ci-security.md` with the no-checkout Git Data API flow, source and report provenance invariants, materialization bounds, scanner parser risk, forced runtime/settings, secret scope, CodeQL acceptance rule, and rollback procedure.
- [x] 7.2 Document why ordinary secret-bearing PR jobs, privileged PR builds, privileged immutable checkout, untrusted or generic source archives, same-repository-only scanning, and automatic analysis were rejected.
- [x] 7.3 Run hostile source-helper tests, focused policy unit tests, immutable-dependency validation, workflow security/YAML validation, and the repository's relevant Go tests; resolve every failure.
- [x] 7.4 Push the revised workflow and confirm CodeQL reports no new untrusted-checkout or equivalent high-severity alert without dismissal, suppression, threshold reduction, or ruleset bypass.
- [x] 7.5 Run `openspec validate secure-sonarcloud-pr-analysis` and confirm the revised implementation satisfies every modified and added scenario.
- [x] 7.6 Reproduce the live `upload-artifact` path layout from PR run `29918022977`, align the trusted report validator, protected path normalization, exact structural-policy assertions, tests, and documentation to exactly two root-level downloaded report files while preserving the established scanner paths, and prove the corrected validator accepts the downloaded live artifact while retaining fail-closed negative fixtures.

## 8. Exercise Live Workflows and Enforce Repository Rules

- [x] 8.1 Record run `29914871551` diagnostics, the public `Ensono_eirctl` and `Ensono_taskctl` project metadata, and repository `SONAR_TOKEN` secret metadata in the change verification notes without recording any credential value.
- [x] 8.2 Record organization `ensono` and project key `Ensono_eirctl` as fixed, document that the current maintainer lacks SonarQube Cloud administration and token-generation rights, and record that access has been requested without changing project identity or creating a duplicate.

**Access dependency:** Tasks 8.3–8.8 and 8.10–8.11 remain blocked until an authorized maintainer can inspect the canonical project and replace the analysis credential; independently completed governance task 8.9 is not blocked.

- [ ] 8.3 Through authenticated SonarQube Cloud access, verify the fixed `Ensono_eirctl` project is bound to `Ensono/eirctl` with main branch `main`, determine the `ensono` organization plan, and repair or provision the binding only under the same fixed organization and project key if required.
- [ ] 8.4 Generate the plan-supported least-privilege analysis credential, replace the GitHub `SONAR_TOKEN` secret value without exposing it, and record its type, owner, and expiry in the team's secret-management process.
- [ ] 8.5 Exercise a trusted `main` push and confirm project settings load, reports upload, analysis submission, coverage ingestion, scanner completion, and quality-gate waiting; verify the absence of `.scannerwork`, `NOT_FOUND`, `NONEXISTENT`, and authorization failures, then revoke the superseded credential.
- [ ] 8.6 Exercise a same-repository PR and confirm the exact head SHA, coverage, SonarCloud PR decoration, quality-gate result, acceptable no-`.git` new-code behavior, and no secret exposure in the untrusted workflow.
- [ ] 8.7 Exercise a fork-originated PR with adversarial Git tree entries, `sonar-project.properties`, scripts, workflows, local actions, dependency hooks, container definitions, and inert executable-looking files; confirm forbidden entries fail closed or remain unmaterialized, trusted settings win, no fork content executes, and SonarCloud decorates the exact fork revision.
- [ ] 8.8 Use the same-repository and fork exercises to confirm and document the API-materialized source tree's SonarCloud SCM/new-code fidelity and passive-only boundary without `.git`, then rerun static policy and CodeQL validation for the selected form.
- [x] 8.9 Confirm `main-is-main` requires code-owner review and verify workflow, CODEOWNERS, and Sonar property changes request `@Ensono/digital-tools-maintainers` approval.
- [ ] 8.10 Observe the stable external SonarCloud check context and integration ID from a successful live PR analysis, add that exact check to `main-is-main`, and verify a missing or failing quality gate blocks merge while a passing gate satisfies the rule.
- [ ] 8.11 Record final workflow URLs, canonical project and binding state, credential type/owner/expiry without its value, ruleset state, action and scanner release/SHA/version evidence, source bounds, residual scanner-parser risk, and rollback information in the change verification notes.
