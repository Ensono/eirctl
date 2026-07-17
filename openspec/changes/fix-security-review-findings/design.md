## Context

The current debug flow uses an `issue_comment` broker to remove and re-add a label with the repository `GITHUB_TOKEN`, expecting the resulting `pull_request:labeled` event to start the builder. GitHub suppresses workflow runs for most events created by `GITHUB_TOKEN`, so the signal changes repository state but does not start the build. The publisher is manually dispatchable from repository refs and therefore needs an independently enforced default-branch boundary.

The repository also treats a checker executed from the pull-request checkout as an authoritative policy gate. That permits one change to alter both a workflow and the code intended to reject it. The checker additionally scans raw text with formatting-sensitive regular expressions. Release builds use task-runner images selected only by mutable tags, and downloaded CI tools use floating selectors. Git SSH verification is fail-closed, but its `strings.Fields` path handling loses valid quoted path boundaries and its system defaults are Unix-only.

The `debug-release` GitHub Environment already exists. This design uses that environment and does not create or provision it.

## Goals / Non-Goals

**Goals:**

- Make an authorized debug request reliably start exactly one isolated builder for the validated current pull-request head.
- Keep untrusted build code read-only, secretless, cache-isolated, and separate from publication authority.
- Ensure debug publication uses the protected default-branch workflow definition and the existing protected environment.
- Make the required workflow-policy check independent of pull-request modifications and structural rather than formatting-sensitive.
- Make maintained task-runner images and downloaded CI tools reproducible and immutable.
- Preserve OpenSSH-compatible known-host paths, including spaces, multiple files, and Windows defaults.
- Add regression coverage for each review finding and policy bypass.
- Deliver the verified implementation through the current branch, communicate the result on its existing pull request, and monitor the pushed head SHA until all required checks are green.
- Provide an executable post-merge manual test matrix with expected evidence, cleanup, and rollback criteria.

**Non-Goals:**

- Creating the `debug-release` environment or redefining its reviewer membership.
- Redesigning normal pull-request tests, production release semantics, or artifact formats beyond fields needed to authenticate the dispatched builder.
- Removing the explicit `StrictHostKeyChecking=no` compatibility escape hatch.
- Remediating unrelated pre-existing analyzer findings or pinning every developer-only tool in the repository.
- Creating or merging a pull request, force-pushing, bypassing branch protection, approving environments automatically, or retrying failed CI without diagnosis.

## Decisions

### 1. Dispatch the builder directly instead of signaling with a label

The broker will use the GitHub Actions workflow-dispatch API with the repository `GITHUB_TOKEN`, an event type GitHub explicitly allows to start another workflow. It will:

1. accept only the exact `/build-debug` comment on a pull request;
2. verify the commenter has `write`, `maintain`, or `admin` permission;
3. query the pull request and capture its current immutable head SHA and number;
4. dispatch the debug builder from the protected default-branch workflow definition with those values as inputs; and
5. serialize requests per pull request.

The broker will not check out code. Its job permissions will be limited to the API scopes needed to read pull-request identity and dispatch the workflow; the obsolete label mutation and `issues: write` permission will be removed unless a status comment remains necessary.

The debug builder will change from `pull_request:labeled` to `workflow_dispatch` inputs. Before checkout, a pinned trusted API step will re-read the pull request and verify that its repository, number, and current head SHA match the dispatch inputs. The checkout will use that SHA. The job will retain read-only contents access, no environment, no secret references, and disabled dependency caching. Provenance will record the dispatch event, builder run ID and attempt, pull request, and verified source SHA.

The publisher will update its run-identity checks from `pull_request` to the dedicated dispatched builder and will continue verifying the current PR SHA and downloaded provenance before the write-capable job starts.

**Alternatives considered:**

- Continue toggling the label with a GitHub App or PAT: preserves the `pull_request` event but introduces a long-lived external credential and additional repository setup.
- Use `repository_dispatch`: it works with `GITHUB_TOKEN` but requires broader contents authority and provides less workflow-specific intent than dispatching the named builder.
- Keep the existing label design: rejected because token-generated label events do not start the required workflow.

### 2. Enforce the publisher's default-branch boundary in code and environment policy

Both publisher jobs will require `github.ref == 'refs/heads/main'`, and the structural workflow policy will require the same guard, `environment: debug-release`, and isolated job-scoped write permissions. The existing `debug-release` environment should restrict deployments to the protected default branch so a branch-modified workflow cannot remove the in-file guard and receive approval. No environment creation task is included.

**Alternatives considered:**

- Rely only on an in-workflow `if`: insufficient because a branch-selected workflow can modify its own condition.
- Rely only on reviewer vigilance: insufficient as a machine-enforced trust boundary.
- Split publication behind an additional `repository_dispatch` workflow: stronger default-branch source selection, but adds another privileged broker and broader token permissions. The existing environment's deployment-branch rule plus explicit guards is simpler and independently enforceable.

### 3. Run the authoritative policy check from trusted default-branch code

Add a small `pull_request_target` policy workflow whose checked-out source is explicitly the protected base SHA and whose permissions are read-only. It will never check out the pull-request tree into the executable workspace or invoke a local action from it. A trusted script will fetch the pull-request Git object, archive an allowlisted candidate tree into a temporary data directory, and invoke the base-branch checker with an explicit candidate-root argument.

The candidate root will include complete workflow and maintained build configuration data needed by policy validation. The checker binary, module graph, shell commands, and workflow definition all remain from the protected base revision. The existing pull-request-local check may remain for fast feedback, but documentation and branch protection must identify the trusted check as authoritative.

**Alternatives considered:**

- Execute the checker from the pull-request checkout: rejected because the same change can weaken the checker.
- Download arbitrary pull-request files and execute the candidate script: rejected because it crosses the `pull_request_target` trust boundary.
- Encode all policy in shell/YAML: rejected because structural and cross-workflow checks are clearer and testable in Go.

### 4. Replace raw-text workflow heuristics with a structural policy model

Refactor the policy command into error-returning validation functions and a thin CLI. Parse workflows with `yaml.v3` into a model that records triggers, job permissions and conditions, checkout refs, environments, secret references, caches, and post-checkout steps. Validation will be independent of quoting, flow syntax, and indentation.

For privileged triggers, dynamic checkout refs are untrusted unless a rule proves the ref is bound to a trusted repository, branch, and immutable SHA. `run`, local actions, and external actions that build, interpret, package, or execute workspace content count as execution. Unknown post-checkout actions in a privileged job fail closed unless explicitly classified as inert. Repository-specific topology checks will operate on the parsed model rather than required substrings.

Table-driven tests will cover each permission and topology rule plus bypass forms such as `on: [workflow_dispatch]`, `${{ inputs.ref }}`, `workflow_run.head_sha`, alternate indentation, local actions, and pinned Docker build actions.

**Alternatives considered:**

- Expand the current regular expressions: rejected because every new YAML spelling or indirection creates another bypass class.
- Adopt a general third-party workflow policy engine: deferred because the current rules are repository-specific and `yaml.v3` is already pinned.

### 5. Pin maintained execution dependencies and validate the pins

Task-runner image references used by CI and release paths will retain a readable tag but add a reviewed manifest digest (`name:tag@sha256:...`). This includes the Go release builder and other maintained contexts that execute during lint, test, or release. Exact digests will be resolved from the intended registries during implementation and recorded in the diff.

GitVersion setup will request an exact reviewed release instead of `5.x`. `govulncheck` will use an exact reviewed module version instead of `@latest`; Go's module checksum mechanism provides integrity verification. Existing version/policy scripts will be extended to reject tag-only maintained context images, floating GitVersion selectors, and `@latest` security-tool installs.

**Alternatives considered:**

- Pin only Dockerfile bases: rejected because eirctl task contexts are separate executable images and include the privileged release build.
- Accept tags with Dependabot updates: rejected because update automation does not make a tag immutable between reviews.

### 6. Preserve known-host path tokens and select platform defaults explicitly

Change effective SSH trust configuration from whitespace-delimited strings to ordered path slices. Parse `GIT_SSH_COMMAND` with a quote-aware argument parser rather than `strings.Fields`. For SSH config files, extract `UserKnownHostsFile` and `GlobalKnownHostsFile` from the parser's matched host blocks while retaining original token boundaries; include processing must follow the same precedence as the effective SSH configuration. Normalize `~` only after tokenization.

Default user paths remain under the resolved home directory. System defaults will be selected by an injectable platform helper: Unix uses `/etc/ssh/ssh_known_hosts{,2}`, while Windows includes the standard `%ProgramData%\ssh\ssh_known_hosts` location. Tests will inject the platform and environment rather than depending on the host running the suite.

**Alternatives considered:**

- Continue using `strings.Fields`: rejected because it demonstrably splits a valid quoted path.
- Treat every parser-returned value as one path: rejected because it would lose valid multiple-file configuration.
- Shell out to `ssh -G`: rejected because eirctl should not require an external OpenSSH client merely to resolve trust files.

### 7. Deliver through the current branch and existing pull request

After implementation and local validation, delivery will operate only on the current non-default branch. The delivery sequence is:

1. verify the current branch is named, is not `main`, and matches the intended pull-request head;
2. inspect `git status`, the complete diff from `main`, and staged content so generated files, credentials, temporary artifacts, and unrelated changes are excluded;
3. stage only the intended implementation and planning files, rerun required validation against the staged tree where practical, and create a descriptive conventional commit;
4. push with `git push --set-upstream origin HEAD`, never with force, and record the pushed commit SHA;
5. locate the existing pull request for the current branch;
6. update a clearly delimited managed section of the pull-request body with the implementation summary, security decisions, validation evidence, residual risks, and post-merge manual test plan; if body editing is unavailable or would overwrite unrelated content, add the same information as a PR comment instead; and
7. monitor checks for the pushed head SHA until every required check succeeds and no check is failed, cancelled, timed out, stale, or awaiting action.

The PR update will preserve all human-authored text outside markers such as `<!-- fix-security-review-findings:start -->` and `<!-- fix-security-review-findings:end -->`. If no associated PR exists, authentication is unavailable, the remote does not match the repository, or the branch changed while delivering, the process stops and reports the exact manual action needed. It will not create or merge a PR automatically.

Check monitoring will bind to the pushed SHA rather than only the PR number. A later push invalidates the observed result and restarts monitoring for the new head. `success` is required for required checks; `skipped` or `neutral` is acceptable only when the repository configuration intentionally marks that check non-applicable. On failure, logs are collected and summarized; checks are not blindly rerun. A code or configuration fix requires a new commit and push, followed by a fresh monitoring cycle.

**Alternatives considered:**

- Overwrite the entire PR body: rejected because it can destroy reviewer context and human-authored instructions.
- Always post a new comment: safe but noisy; use it only when a managed body update is not possible.
- Force-push or amend after review starts: rejected because it obscures review history and can invalidate approvals.
- Treat a successful local suite as delivery completion: rejected because GitHub permissions, workflow syntax, repository settings, and hosted-runner behavior require remote evidence.

## Risks / Trade-offs

- **[Broker needs workflow-dispatch authority]** → Keep the broker code on the default branch, perform no checkout, use exact comment and collaborator checks, pin the API action, and grant only the minimum Actions/read scopes.
- **[A dispatched builder is a privileged event type]** → Treat effective job authority, workflow source, immutable input validation, cache isolation, and lack of secrets as mandatory policy invariants.
- **[Structural policy may reject legitimate workflows]** → Use explicit trusted-ref proofs and a reviewed inert-action classification with focused fixtures; fail closed only in privileged checkout paths.
- **[The trusted policy workflow handles untrusted Git objects]** → Archive candidate files into a separate temporary data root, never source or execute them, avoid candidate local actions, and keep permissions read-only.
- **[Digest pins increase update maintenance]** → Keep readable tags beside digests, make validation errors actionable, and update tag and digest together in reviewed dependency changes.
- **[OpenSSH token semantics are subtle]** → Keep parsing narrowly scoped to trust-file directives, preserve order and precedence, and add fixtures for quotes, escapes, repeated values, includes, and Windows paths.
- **[Repository settings are partly external to code]** → Document that the already-created `debug-release` environment must allow deployments only from protected `main`; the workflow and checker provide defense in depth.
- **[Automated Git operations can publish unintended files or credentials]** → Require branch, status, ignored-file, staged-diff, and secret checks before commit; stage explicit paths; never force-push.
- **[PR automation can erase reviewer context]** → Update only a delimited managed section and fall back to a comment when safe body editing is unavailable.
- **[CI status can be stale after another push]** → Bind monitoring and reported evidence to the exact pushed head SHA and restart when the head changes.
- **[Manual publication tests can create real releases]** → Use a dedicated harmless test PR and prerelease tag, require human environment approval, record cleanup, and never approve production deployment as part of an unattended run.

## Migration Plan

1. Add structural checker functions, candidate-root support, and bypass/regression tests while retaining current workflow behavior.
2. Add the trusted base-branch policy workflow and document the required check name; configure branch protection to require it after the workflow exists on `main`.
3. Convert the broker and builder to validated workflow dispatch, then update publisher identity/provenance validation in the same merge so no mixed signaling contract is deployed.
4. Add publisher default-branch guards and confirm the existing `debug-release` environment's deployment-branch restriction before enabling the updated publisher.
5. Resolve and apply image digests and exact tool versions, then enable immutable-pin validation.
6. Implement token-preserving SSH trust paths and platform defaults with focused tests.
7. Run workflow policy, workflow lint, Go-version and immutable-pin checks, targeted race tests, the full Go suite, analyzers, and OpenSpec validation.
8. Review and stage only intended files, commit on the current implementation branch, push the exact validated commit, and update the existing PR's managed summary or add a fallback comment.
9. Monitor all checks for the pushed head SHA, diagnose failures, and require a terminal green result before declaring the change ready for review or merge.
10. After merge, execute the manual testing plan below in a controlled test PR and record evidence, cleanup, and any follow-up defects.

Rollback consists of disabling debug dispatch/publication while retaining read-only tests and reverting the broker, builder, and publisher together. Do not roll back immutable image/tool pins or host-key verification merely to restore the old debug signal.

## Manual Testing Plan

### Entry criteria and evidence

Execute this plan only after the implementation commit is merged to `main`, required CI is green for the merge commit, and the existing `debug-release` environment is confirmed to restrict deployments to protected `main`. Use a dedicated harmless test PR whose head SHA can be changed safely, an authorized maintainer account, and—where noted—a user without write permission. Windows SSH cases require a Windows host or runner with OpenSSH-compatible configuration.

For every case, record the repository, PR number, source and merge SHAs, workflow/run/attempt IDs, artifact ID and name, relevant timestamps, actor, observed permissions/environment, result, and links to logs. Redact tokens, private keys, and sensitive host data. Any unexpected write authority, secret exposure, execution of candidate policy code, host-key bypass, or publication from a non-main workflow definition is a release blocker.

### Debug request and builder

| ID | Procedure | Expected result and evidence |
| --- | --- | --- |
| MT-01 | On the dedicated PR, have an authorized maintainer post exactly `/build-debug`. | The broker authorizes the actor, performs no checkout, dispatches one Debug build for the current PR head, and records the dispatch in logs. The builder validates the PR/SHA before checkout and completes with read-only permissions. |
| MT-02 | Post a malformed command such as `/build-debug please`, then post the exact command from an actor without write permission. | The malformed command does not dispatch. The unauthorized exact command fails authorization and produces no builder run or artifact. |
| MT-03 | Trigger a request and immediately push a new harmless commit before builder validation. | The builder either validates and builds the captured immutable SHA while publication later rejects it as stale, or fails the current-head check before checkout. It never silently substitutes another revision. |
| MT-04 | Manually dispatch the builder with a nonexistent PR, malformed SHA, another repository's SHA, and a stale PR SHA. | Each request fails before untrusted checkout/build. No artifact is uploaded and no environment or write permission is granted. |
| MT-05 | Inspect a successful builder run and artifact. | Checkout is the verified PR SHA; setup-go caching is disabled; no protected environment or secret is present; repository authority is read-only; provenance contains repository, PR, source SHA, workflow event, run ID, and attempt; binaries and provenance are the only expected artifact content. |

### Debug publication boundary

| ID | Procedure | Expected result and evidence |
| --- | --- | --- |
| MT-06 | From `main`, dispatch publication with the valid successful builder run ID, PR number, and current source SHA; review and approve the `debug-release` deployment. | Validation resolves the exact workflow, repository, event, PR, SHA, attempt, artifact ID, and provenance before the publish job requests write authority. Approval creates only the expected prerelease/tag and uploads binaries without executing them. |
| MT-07 | Repeat validation separately with a failed run, wrong workflow run, wrong PR, stale SHA, wrong repository where practical, expired/missing artifact, altered provenance fixture, and mismatched run attempt. | Every case stops before the write-capable publication job. No tag, release, deployment approval request, or artifact execution occurs. |
| MT-08 | Attempt to dispatch the publisher from a non-main branch or tag, including a branch that removes the in-workflow guard. | Job conditions and the existing environment deployment-branch restriction prevent publication. No `contents: write` job starts and no release/tag is created. |
| MT-09 | Inspect the valid prerelease assets as opaque files and compare checksums with the builder artifact. | Published binaries match the selected artifact; provenance is used for validation but is not executed; no unexpected files are published. Remove the test prerelease and tag after evidence is retained. |

### Trusted policy enforcement

| ID | Procedure | Expected result and evidence |
| --- | --- | --- |
| MT-10 | Open or update a test PR that changes its copy of the checker to always succeed and also adds a known unsafe privileged checkout/execution workflow. | The authoritative `pull_request_target` check uses base-branch code, inspects the candidate files as data, and fails. Candidate scripts/local actions are never executed. |
| MT-11 | Exercise candidate workflows using flow-style triggers, alternate indentation, `${{ inputs.ref }}`, `workflow_run.head_sha`, a step-output ref, a local action, and a pinned Docker build action after untrusted checkout. | Structural validation consistently rejects every privileged untrusted-execution form independent of YAML formatting. |
| MT-12 | Run the trusted policy workflow against the unchanged merged workflows. | The check passes from the base SHA with read-only permissions and no candidate checkout in the executable workspace. The stable check name is suitable for branch protection. |

### Immutable dependencies and release regression

| ID | Procedure | Expected result and evidence |
| --- | --- | --- |
| MT-13 | In a temporary branch, remove one context-image digest, change GitVersion to `5.x`, and change `govulncheck` to `@latest`, one mutation at a time. | The immutable-pin validator rejects each mutation with the exact file and selector; the merged pinned configuration passes. |
| MT-14 | Run lint, tests, debug build, and an approved release-build dry run or non-publishing equivalent while inspecting image/tool resolution. | Runtime images resolve to the reviewed digests, GitVersion and `govulncheck` resolve to exact versions, and outputs remain reproducible. Do not approve a production release solely for this test. |
| MT-15 | Observe the normal workflows for the merge commit without approving unrelated production deployments. | Lint/test and trusted policy checks use the merge SHA and pass. Any release workflow uses the validated main SHA and remains gated by its existing production environment policy. |

### Git SSH verification and portability

| ID | Procedure | Expected result and evidence |
| --- | --- | --- |
| MT-16 | On Unix, clone a test Git SSH source using a trusted plain known-host file, a quoted path containing spaces, escaped spaces, multiple files, an included SSH config, an alias, and a non-default port. | Each valid configuration preserves path boundaries and precedence, resolves the effective host/port, verifies the trusted key, and reads the expected repository content. |
| MT-17 | Repeat with no matching key, a changed key, a missing explicitly configured file, and no usable default trust source. | Every connection fails closed before repository data is accepted, with actionable host context and no credential material. No fallback silently disables verification. |
| MT-18 | Set `StrictHostKeyChecking=no` explicitly for the test host. | The connection may proceed and emits the documented warning on every connection; removing the option restores fail-closed verification. |
| MT-19 | On Windows, place a trusted key in the standard ProgramData OpenSSH known-host file and repeat a successful clone; then test a quoted custom Windows path containing spaces. | The standard system file is discovered without an override, the custom path remains intact, and host verification behaves the same as Unix. |

### Completion, cleanup, and rollback

The manual pass is complete only when all applicable cases pass, evidence is linked from the PR or follow-up issue, the dedicated test PR is closed, temporary branches and fixtures are removed, and the debug prerelease/tag is deleted if it was created solely for validation. If a trust-boundary case fails, disable the affected broker/builder/publisher path before further use and open a blocking defect with run IDs and sanitized evidence. Functional SSH regressions may be rolled back independently, but host-key verification and immutable pins must not be disabled as a shortcut.

## Implementation verification

The implementation resolved the maintained execution selections as follows: GitVersion `6.0.5` (the newest exact `6.0.x` release supported by the pinned GitVersion action), `golang.org/x/vuln/cmd/govulncheck@v1.6.0`, and the reviewed manifest-list digests recorded in `docs/ci-security.md` for `bash`, `go1x`, `golint`, and `goreleaser`. The immutable-dependency validator and negative fixtures enforce these selections.

The existing `debug-release` environment was verified on 2026-07-17 to have required reviewers and a custom deployment branch policy that permits `main`. The implementation did not create or replace the environment. Manual-plan owner: the release-maintainer team; retain sanitized run IDs, artifacts, approvals, and logs in the PR managed section or a linked follow-up issue.
