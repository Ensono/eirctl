## 1. Structural Workflow Policy

- [x] 1.1 Refactor `scripts/check-workflow-policy` into error-returning validation functions plus a thin CLI, and add an explicit candidate-root option so trusted code can validate another tree.
- [x] 1.2 Replace raw-text trigger, checkout-ref, permission, environment, secret, and execution checks with a `yaml.v3` structural workflow model.
- [x] 1.3 Implement fail-closed privileged-flow analysis for dynamic checkout refs, local actions, shell steps, and external actions that consume checked-out repository content.
- [x] 1.4 Rework the broker, builder, publisher, release, Scorecard, and permission topology checks to use the structural model rather than substring assertions.
- [x] 1.5 Add table-driven policy tests for flow-style triggers, alternate indentation, `${{ inputs.ref }}`, `workflow_run.head_sha`, step-output refs, local actions, pinned Docker build actions, missing default-branch guards, and each existing permission/topology invariant.

## 2. Trusted Policy Enforcement

- [x] 2.1 Add a trusted script that fetches the pull-request Git object and materializes only the candidate configuration tree into a temporary data directory without sourcing or executing candidate files.
- [x] 2.2 Add a pinned, read-only `pull_request_target` workflow that checks out the protected base SHA, runs the base-branch checker against the candidate data root, and never invokes pull-request-controlled actions or scripts.
- [x] 2.3 Retain or clearly label the pull-request-local policy check as advisory feedback, and document the trusted policy workflow's stable required-check name for branch protection.
- [x] 2.4 Add policy fixtures proving that a pull request cannot bypass the authoritative check by modifying its copy of the checker, validation script, or workflow.

## 3. Reliable Debug-Build Dispatch

- [x] 3.1 Replace label removal/re-addition in `debug-build-request.yml` with a workflow-dispatch API call after exact-comment and collaborator authorization, passing the current PR number and immutable head SHA.
- [x] 3.2 Reduce the broker permissions to the minimum pull-request read and workflow-dispatch scopes, preserve per-PR concurrency, and assert that the broker never checks out code.
- [x] 3.3 Convert `debug-build.yml` to typed `workflow_dispatch` inputs and add a pinned pre-checkout API step that validates repository identity, PR number, and current head SHA before checking out untrusted code.
- [x] 3.4 Preserve read-only permissions, no environment or secret references, disabled caches, immutable-SHA checkout, and provenance containing the dispatch event, PR, source SHA, run ID, and run attempt.
- [x] 3.5 Update `publish-debug-release.yml` to authenticate the dispatched builder's workflow identity, event, repository, PR, run attempt, artifact ID, current SHA, and provenance before the write-capable job starts.
- [x] 3.6 Add static fixtures and tests that reject the old `GITHUB_TOKEN`-generated label topology and accept only the supported isolated dispatch topology.

## 4. Default-Branch Publication Boundary

- [x] 4.1 Add fail-closed `refs/heads/main` conditions to both debug publication jobs while preserving the existing `debug-release` environment and job-scoped permissions.
- [x] 4.2 Extend structural policy validation and tests to require the main-ref guards, the `debug-release` environment, read-only validation, and isolated `contents: write` publication.
- [x] 4.3 Confirm that the already-created `debug-release` environment permits deployments only from protected `main`, record the verification in the implementation summary, and do not create or replace the environment.

## 5. Immutable CI Images and Tools

- [x] 5.1 Resolve reviewed manifest digests for every maintained task-runner image used by lint, test, or release paths and record the registry/tag/digest evidence.
- [x] 5.2 Update maintained context image references, including the `go1x` release builder, to readable `name:tag@sha256:digest` form.
- [x] 5.3 Select and configure an exact reviewed GitVersion release in every workflow instead of `5.x`, and pin `govulncheck` to an exact reviewed module version instead of `@latest`.
- [x] 5.4 Extend automated version/security validation to reject tag-only maintained context images, floating GitVersion selectors, and `@latest` CI security-tool installs.
- [x] 5.5 Add negative fixtures for each mutable selector and verify that the current pinned repository configuration passes.

## 6. OpenSSH-Compatible Known-Host Paths

- [x] 6.1 Represent effective user and system known-host selections as ordered path slices and preserve precedence through SSH config and `GIT_SSH_COMMAND` processing.
- [x] 6.2 Replace `strings.Fields` parsing with quote- and escape-aware tokenization that preserves paths containing spaces, multiple files, repeated directives, and included SSH configuration.
- [x] 6.3 Add an injectable platform-default helper that retains Unix system trust paths and discovers the standard Windows ProgramData OpenSSH known-host file.
- [x] 6.4 Add focused tests for quoted and escaped paths, multiple configured files, include/precedence behavior, Windows defaults, missing files, hashed hosts, aliases, non-default ports, and the explicit insecure opt-out.
- [x] 6.5 Run the SSH/config package under the race detector and confirm that failure messages remain fail-closed and free of credential material.

## 7. Documentation and Validation

- [x] 7.1 Update CI security documentation to describe direct debug workflow dispatch, trusted base-branch policy enforcement, default-branch publication, immutable execution pins, and the existing `debug-release` environment requirement without claiming the environment is absent.
- [x] 7.2 Update SSH and contributor documentation for quoted known-host paths, platform defaults, and immutable tool/image maintenance.
- [x] 7.3 Run formatting, module reproducibility checks, workflow lint, workflow policy, Go-version and immutable-pin validation, targeted race tests, the full Go test suite, `go vet`, `staticcheck`, and `gosec`; document any unchanged pre-existing analyzer baseline.
- [x] 7.4 Run `openspec validate fix-security-review-findings --type change --strict` and verify that proposal, design, delta specs, tasks, implementation, and repository status are coherent and clean.

## 8. Current-Branch Delivery and Pull Request

- [x] 8.1 Verify the current branch is named, is not `main`, matches the intended PR head, and has the expected remote; inspect status, ignored files, the full `main...HEAD` diff, and candidate staged paths for unrelated files or credentials.
- [ ] 8.2 Stage only intended implementation and OpenSpec files, review the staged diff, rerun required validation where practical, create a descriptive conventional commit, and record its SHA.
- [ ] 8.3 Push the current branch with upstream tracking when needed, never force-push, and verify that the remote branch and associated PR head resolve to the recorded commit SHA.
- [ ] 8.4 Update only the delimited `fix-security-review-findings` section of the existing PR description with the implementation summary, security decisions, validation evidence, residual risks, and post-merge manual test plan; add the same content as a comment if a non-destructive body update is unavailable.
- [ ] 8.5 Monitor all checks for the exact pushed head SHA until every required check succeeds and no check is failed, cancelled, timed out, stale, or awaiting action; accept skipped or neutral checks only when intentionally non-applicable.
- [ ] 8.6 If a check fails, collect and summarize its logs, make a new reviewed commit only when a code or configuration fix is required, push without rewriting history, refresh the PR summary, and repeat monitoring for the new head SHA.

## 9. Post-Merge Manual Test Plan Handoff

- [ ] 9.1 Reconcile the design's MT-01 through MT-19 cases with the final implementation so workflow names, inputs, permissions, provenance fields, image/tool pins, SSH paths, and expected evidence are exact.
- [ ] 9.2 Confirm the plan identifies entry criteria, required test accounts and platforms, release-blocking observations, sanitized evidence to retain, cleanup actions, and rollback steps without requiring creation of the existing `debug-release` environment.
- [ ] 9.3 Include the complete manual test plan or a stable repository link to it in the PR managed section or fallback comment, and identify the post-merge execution owner and evidence location.
- [ ] 9.4 Verify the plan uses a harmless dedicated test PR and prerelease, does not auto-approve production environments or merge the PR, and requires temporary branches, artifacts, releases, and tags to be cleaned up after evidence is captured.
