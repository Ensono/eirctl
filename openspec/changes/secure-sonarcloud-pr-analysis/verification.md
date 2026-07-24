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

## Revised PR Static Acceptance — 2026-07-21

The no-checkout implementation was pushed at revision `1abff98005c54a91b77af36d00c01ed8f56da1df`. [CodeQL run 29823553510](https://github.com/Ensono/eirctl/actions/runs/29823553510) passed both `Analyze (actions)` and `Analyze (go)`. The repository API reports alert [#21](https://github.com/Ensono/eirctl/security/code-scanning/21), `actions/untrusted-checkout/high`, as `fixed` at `2026-07-21T10:43:13Z`; there are no open or dismissed matching alerts for pull request #114. `dismissed_at` and `dismissed_reason` remain null, and the active `main-is-main` threshold and empty bypass list were not changed.

The same revision passed [Trusted workflow policy run 29823554649](https://github.com/Ensono/eirctl/actions/runs/29823554649) and every `Lint and Test` job in [run 29823555786](https://github.com/Ensono/eirctl/actions/runs/29823555786), including `Lint` and `Test (Linux)`. The PR's ordinary `SonarCloud analysis` job remained skipped and received no secret, as designed.

## Scorecard Dangerous-Workflow Follow-up — 2026-07-21

The default-branch Scorecard scan at `f2526393f439bdb99df14aa6deae105465c89ed1` has five open critical `DangerousWorkflowID` alerts: [#12](https://github.com/Ensono/eirctl/security/code-scanning/12) and [#13](https://github.com/Ensono/eirctl/security/code-scanning/13) in `.github/workflows/release.yml`, [#14](https://github.com/Ensono/eirctl/security/code-scanning/14) and [#15](https://github.com/Ensono/eirctl/security/code-scanning/15) in `.github/workflows/release_container.yml`, and [#16](https://github.com/Ensono/eirctl/security/code-scanning/16) in `.github/workflows/trusted-workflow-policy.yml`. The dynamic checkout expressions predate pull request #114; this branch had changed only their action pins, not introduced the expressions.

The branch now removes all five dynamic checkout refs. `pull_request_target` uses its implicit protected base revision. Release jobs checkout literal `main` without persisted credentials and immediately compare `HEAD` to the trusted upstream run SHA before any versioning, build, registry, tag, or release operation. The binary release's write token is exposed only to the post-build Git Data API tag-creation step. The protected structural checker mirrors Scorecard's rule by rejecting any `github.event.pull_request` or `github.event.workflow_run` checkout ref and requires the release verification and non-persistent credential settings. Live closure remains pending until the branch reaches `main` and the Scorecard workflow reruns; no alert will be dismissed and no threshold or ruleset will be weakened.

## Live CODEOWNERS Governance — 2026-07-21

The active `main-is-main` ruleset requires code-owner review, one approval, last-push approval, stale-review dismissal, and resolved review threads, with no bypass actors. Pull request #114 changes `.github/CODEOWNERS` and protected workflow paths; GitHub requests review from the `Ensono/digital-tools-maintainers` team. The repository's `sonar-project.properties` remains covered by the same CODEOWNERS rule even though this revision does not modify it.

## SonarQube Cloud Identity and Access Blocker — 2026-07-22

Organization `ensono` and project key `Ensono_eirctl` are confirmed fixed identifiers. The current maintainer cannot administer the SonarQube Cloud project, change its binding, generate a replacement analysis token, or inspect the existing token value and has requested the required access. No alternate key, project rename, duplicate project, token disclosure, or credential workaround was attempted.

Trusted-main [`Lint and Test` run 29914871551](https://github.com/Ensono/eirctl/actions/runs/29914871551) ran for revision `882d7623cf5d428cc8995d6cf9c1304d99b82c9b` from `2026-07-22T11:13:13Z` to `2026-07-22T11:21:34Z` and failed only in the `SonarCloud analysis` job after the tests and report generation succeeded. The immutable `SonarSource/sonarqube-scan-action` revision `22918119ff8e1ca75a623e15c8296b6ea4fbe28f` installed SonarScanner CLI `8.1.0.6389`, downloaded its detached signature, imported the SonarSource signing key, verified the distribution, and invoked the scanner with trusted organization `ensono`, project key `Ensono_eirctl`, revision `882d7623cf5d428cc8995d6cf9c1304d99b82c9b`, and quality-gate waiting enabled. `SONAR_TOKEN` was present only as a masked scanner-step environment value.

The scanner reached SonarQube Cloud and attempted to load settings for `Ensono_eirctl`. It then reported `NOT_FOUND`, detected project binding `NONEXISTENT`, and ended with `Not authorized or project not found`. The former `/eirctl/.scannerwork` permission failure did not recur. This result proves scanner installation and connectivity but cannot distinguish an inaccessible or invalid token from authenticated project-state or binding problems.

Unauthenticated public API evidence collected on `2026-07-22` shows:

- organization `ensono` resolves as `Ensono`; the public response grants no administration or provisioning action;
- component `Ensono_eirctl` returns `404` with `Project doesn't exist`; because this request is unauthenticated, it is not treated as authority to change the confirmed fixed key or create another project;
- public component `Ensono_taskctl` exists in organization `ensono`, is named `taskctl`, reports version `2.0.5`, and was last analyzed on `2025-03-04`; it is recorded as historical public evidence only and is not assumed to be a migration source; and
- the GitHub repository exposes a `SONAR_TOKEN` Actions secret last updated at `2025-08-27T09:20:00Z`; its value, credential type, owner, expiry, validity, and project authorization were neither retrieved nor inferred.

### Authorized-maintainer handoff

1. Use authenticated SonarQube Cloud administration to verify that fixed project `Ensono_eirctl` belongs to organization `ensono`, is bound to `Ensono/eirctl`, and uses `main` as its main branch. Repair or provision the binding only under that same fixed identity; do not rename the key, substitute `Ensono_taskctl`, or create a blind duplicate.
2. Confirm the `ensono` plan and select the supported least-privilege analysis credential: a project-scoped Scoped Organization Token granting only **Execute analysis** when available, otherwise a minimally authorized maintained-identity personal access token.
3. Replace the GitHub repository `SONAR_TOKEN` secret without placing its value in a ticket, pull request, command output, workflow diagnostic, or this repository. Record credential type, owner, and expiry in the team's secret-management process.
4. Rerun trusted `main` analysis and require successful project-settings loading, report ingestion, submission, scanner completion, and quality-gate waiting before revoking the superseded credential.
5. After trusted-main success, complete same-repository and adversarial fork PR exercises, document no-`.git` SCM/new-code fidelity, observe the external SonarCloud check context and integration ID, and only then add that exact check to `main-is-main`.

## Live Report Artifact Layout Regression — 2026-07-22

Planning pull request [#120](https://github.com/Ensono/eirctl/pull/120) passed `Lint`, `Test (Linux)`, and report publication in [`Lint and Test` run 29918022977](https://github.com/Ensono/eirctl/actions/runs/29918022977), [`Trusted workflow policy` run 29918021729](https://github.com/Ensono/eirctl/actions/runs/29918021729), and [`CodeQL` run 29918020842](https://github.com/Ensono/eirctl/actions/runs/29918020842). The dedicated artifact `sonar-reports-29918022977-1` contains exactly two regular root-level files: `out` (1,707,398 bytes) and `report-junit.xml` (68,798 bytes). GitHub's `upload-artifact` action selected `.coverage/out` and `.coverage/report-junit.xml` from the untrusted workspace but stripped their common `.coverage` parent in the downloaded artifact.

Protected [`Trusted SonarCloud` run 29918354589](https://github.com/Ensono/eirctl/actions/runs/29918354589) successfully checked out only the default-branch helpers, resolved immutable upstream provenance, and downloaded the verified artifact by ID. It then failed before source materialization and before any token-bearing scanner step because the validator expected an extracted `.coverage` directory and rejected root entry `out`. The configuration, materializer, and scanner steps were skipped, so this failure neither exposed `SONAR_TOKEN` nor tested SonarQube Cloud authorization.

The corrected bounded contract accepts exactly root-level regular files `out` and `report-junit.xml`, retains the 50 MiB and 10 MiB limits, and continues to reject directories, symlinks, special files, traversal-derived paths, missing coverage, oversized content, and every unexpected entry. After validation, protected code creates `reports/.coverage/`, moves the two fixed report paths beneath it, and enforces mode `0644`; trusted scanner configuration and structural policy keep requiring exact `reports/.coverage/out` and `reports/.coverage/report-junit.xml` paths. Focused validator and workflow-policy tests pass, and the corrected validator accepts and normalizes the exact downloaded artifact from run `29918022977`.

A first attempt changed the workflow's scanner paths to the artifact root. Protected [`Trusted workflow policy` run 29919273702](https://github.com/Ensono/eirctl/actions/runs/29919273702) correctly rejected that same-PR workflow-policy transition because default-branch policy still required the established `.coverage` paths. The final approach leaves the protected workflow topology and scanner paths unchanged, performs normalization inside the already-reviewed validator step, and hardens policy matching so the complete trusted configuration script and scanner argument set must match exactly; suffix drift, duplicate settings, conflicting values, and extra commands or arguments are rejected.

Because `workflow_run` executes protected default-branch workflow and helper code, pull request #120 cannot live-test the corrected helper in its own trusted analyzer run. After the fix merges to `main`, a subsequent same-repository pull request must confirm that validation reaches source materialization and stops only at the separately recorded SonarQube Cloud access blocker until the credential is replaced.

## Authorized SonarQube Cloud Validation and Trusted-Main Success — 2026-07-23

An authorized administrator confirmed through authenticated SonarQube Cloud access that fixed project `Ensono_eirctl` belongs to organization `ensono`, is bound to GitHub repository `Ensono/eirctl`, and uses `main` as its main branch. The organization is on the Base plan, which does not provide Scoped Organization Tokens; the required least-privilege credential path is therefore a maintained-identity personal access token, with its owner and expiry recorded only in the team's secret-management process.

GitHub records that the repository `SONAR_TOKEN` Actions secret was updated at `2026-07-23T07:53:41Z`. The replacement is a maintained-identity personal access token; its value, owner, and expiry are intentionally not recorded here because they are held in the team's secret-management process. The superseded credential was revoked. Trusted-main [`Lint and Test` run 29925703928](https://github.com/Ensono/eirctl/actions/runs/29925703928) analyzed `main` revision `50021beb5ff8b48feb01ae2bf496fb24cbffea76`. Its [`SonarCloud analysis` job](https://github.com/Ensono/eirctl/actions/runs/29925703928/job/89149198435) completed successfully: report generation and the immutable scanner step both succeeded, SonarQube Cloud displayed the main-branch analysis, and the scanner reported `QUALITY GATE STATUS: PASSED`.

This successful run replaces the prior operational blocker: no `.scannerwork` permission failure, `NOT_FOUND`, `NONEXISTENT` binding, or authorization failure was reported. The remaining live acceptance work is a same-repository PR followed by the specified adversarial fork PR; only then may the observed external SonarCloud check identity be required by `main-is-main`.

## Same-Repository PR Exercise — 2026-07-23

Documentation-only pull request [#126](https://github.com/Ensono/eirctl/pull/126) exercised same-repository revision `fa1b0940c4afd7632790f43c288ce4dee3c6df29`. Its untrusted [`Lint and Test` run 29994242556](https://github.com/Ensono/eirctl/actions/runs/29994242556) completed successfully and uploaded verified artifact `sonar-reports-29994242556-1` (artifact ID `8558398722`, digest `sha256:9c421358ad9451917a22ed6c49960c75e0c742db54f8119704c50a348a333f5b`). The ordinary `SonarCloud analysis` job shown on the pull request was skipped by its trusted-main-only condition, as designed; this does not mean that the separate protected analyzer was absent.

Protected [`Trusted SonarCloud` run 29994597333](https://github.com/Ensono/eirctl/actions/runs/29994597333) started three seconds after the upstream workflow completed. It successfully resolved run `29994242556`, pull request #126, the exact head SHA, and artifact ID; downloaded only that artifact; and then failed closed in `Validate bounded passive report artifact` with `artifact contains unexpected entry: out`. Source materialization, scanner configuration, and the token-bearing scanner step were skipped. The run therefore confirms that the `workflow_run` trigger and immutable provenance binding worked and that no secret reached untrusted execution, but it cannot satisfy task 8.6 because SonarQube Cloud analysis and PR decoration were never reached.

The failure was produced by protected `main` revision `350fd3ec0520413d0ab5fa944b2e890d8f99f514`, whose validator still expected `.coverage/out` and `.coverage/report-junit.xml`. Pull request [#120](https://github.com/Ensono/eirctl/pull/120) contains the already tested correction that accepts the actual root-level `out` and `report-junit.xml` artifact contract and then normalizes those files under `.coverage/` for the established scanner paths. A local probe downloaded the exact PR #126 artifact and confirmed the corrected validator accepts `out` (1,733,199 bytes) and `report-junit.xml` (70,063 bytes), moves both beneath `.coverage/`, and enforces mode `0644`. Because `workflow_run` always executes protected default-branch helpers, updating or rerunning pull request #126 cannot test that correction until #120 merges to `main`. After merge, synchronize #126 (or open an equivalent documentation-only same-repository PR) to generate a new upstream run and complete task 8.6.

## Same-Repository Coverage Namespace Regression — 2026-07-23

After pull request #120 merged as `b21030ce9017ae2c0cfbb6178eee5f3bddacf919`, pull request #126 was reopened without changing head revision `fa1b0940c4afd7632790f43c288ce4dee3c6df29`. Untrusted [`Lint and Test` run 30002191122](https://github.com/Ensono/eirctl/actions/runs/30002191122) passed and uploaded `sonar-reports-30002191122-1`. Protected [`Trusted SonarCloud` run 30002506806](https://github.com/Ensono/eirctl/actions/runs/30002506806) successfully completed provenance resolution, exact artifact download, bounded report validation, trusted configuration creation, and API-based Go source materialization. The scanner loaded project settings, confirmed the project binding as `BOUND`, created pull-request analysis 126 for the exact revision, parsed 105 isolated Go files, uploaded the analysis report, and published external check `SonarCloud Code Analysis` from SonarQubeCloud integration ID `12526`.

The quality gate failed only on `0.0% Coverage on New Code` against a required `80%`. Scanner diagnostics showed that every repository-relative coverage key such as `internal/schema/schema.go` was ignored because analyzed files are intentionally keyed beneath `source/`, such as `source/internal/schema/schema.go`. The coverage report was present and parsed, but its paths did not match the isolated API-materialized source namespace. No `.scannerwork` permission, project binding, authorization, provenance, report-layout, materialization, or secret-scope failure recurred.

The follow-up keeps source beneath `analysis/source` and retains the no-checkout boundary. Protected report validation now requires UTF-8 inputs, a supported Go coverage mode, and canonical repository-relative `.go` records; rejects malformed, absolute, traversal, backslash, control-character, duplicate-separator, and overlong paths; and deterministically prefixes accepted record paths with `source/` before source materialization or secret exposure. The scanner continues to receive only the established normalized report path.

A local exact-artifact probe downloaded `sonar-reports-30002191122-1`, applied the corrected protected validator, and independently materialized the verified PR revision through the production Git Data API helper. All 40,768 coverage records normalized into `source/`; their 55 unique file keys matched files among the 107 verified materialized Go blobs, with zero missing keys. The hostile validator fixtures, workflow-policy and materializer tests, immutable dependency check, CODEOWNERS check, workflow security check, full Go suite, OpenSpec validation, and diff check all passed. Task 8.6 remains open until this correction reaches protected `main` and a repeated live analysis imports coverage and passes its quality gate.
