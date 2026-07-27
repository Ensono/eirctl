# SonarCloud Evidence Ledger

This token-free planning snapshot was obtained through authenticated SonarCloud API calls on 2026-07-27. `SONAR_TOKEN` was loaded only into the querying subprocess and was not printed, persisted, or placed in command arguments. Implementation task 1.1 must refresh this ledger before editing because issue status and locations can drift.

## Refreshed pre-change evidence (2026-07-27)

Task 1.1 re-queried `Ensono_eirctl` / `main` using a subprocess-only `SONAR_TOKEN` and an in-process HTTP Authorization header (the token was neither printed, persisted, nor placed in command arguments). The latest analysis remains `67a4b92e-0472-47fa-98d9-b2bfc6e2cc72` for revision `bb3d10dc2360a50752e2c73fe507c427bf14af92` at `2026-07-27T08:38:23+0000`, version `0.11.4`. Its previous-version period begins at `0.11.3` / `2026-05-27T15:20:06+0000`; the gate remains `ERROR` solely because `new_code_smells_severity` is `20` against the unchanged `GT 14` threshold. The other conditions remain `OK`: new coverage `82.8` (`LT 80`), duplicated-lines density `0.0` (`GT 3`), bugs severity `0` (`GT 9`), and vulnerabilities severity `0` (`GT 9`).

All 23 scoped keys were returned by the authenticated query. Every Tier 0 key remains `OPEN` with the rule, severity, component, and line recorded in the table below; no mandatory key drifted or was missing. The current checkout contains the analyzed revision (`bb3d10dc2360a50752e2c73fe507c427bf14af92` is an ancestor of `HEAD`).

## Baseline checks (2026-07-27)

The worktree contained only the OpenSpec change directory before implementation; no scoped source or test file had unrelated edits, and `git diff --check` passed. The repository-level `AGENTS.md` instructions apply. The following focused baseline completed successfully:

```text
go test ./scripts/check-workflow-policy ./scripts/materialize-sonar-source ./internal/config ./cmd/eirctl ./output ./variables ./internal/cmdutils ./internal/genci ./internal/utils ./runner ./scheduler
```

No focused-package baseline failures were observed; later tasks must not normalize any failure as pre-existing.

## Analysis and gate baseline

- Project/branch: `Ensono_eirctl` / `main`
- Analysis key: `67a4b92e-0472-47fa-98d9-b2bfc6e2cc72`
- Analysis date: `2026-07-27T08:38:23+0000`
- Revision: `bb3d10dc2360a50752e2c73fe507c427bf14af92`
- Project version: `0.11.4`
- Previous-version period: begins at version `0.11.3` on `2026-05-27T15:20:06Z`, as recorded by the archived authenticated verification at `openspec/changes/archive/2026-07-27-secure-sonarcloud-pr-analysis/verification.md`
- Overall gate: `ERROR`

| Condition | Status | Actual | Comparator / threshold |
|---|---|---:|---|
| `new_coverage` | OK | 82.8 | `LT 80` |
| `new_duplicated_lines_density` | OK | 0.0 | `GT 3` |
| `new_bugs_severity` | OK | 0 | `GT 9` |
| `new_code_smells_severity` | ERROR | 20 | `GT 14` |
| `new_vulnerabilities_severity` | OK | 0 | `GT 9` |

## Scoped issue snapshot

“Gate blocker” is based on the archived authenticated previous-version attribution, updated for the main analysis that automatically closed `AZ-N-y0t3_y5QfimGTZ3` as fixed. Final acceptance still requires a fresh measure/issue query because the aggregate gate is authoritative.

| Tier | Gate blocker | Issue | Status | Rule | Severity | Assignee | Component |
|---:|:---:|---|---|---|---|---|---|
| 0 | Yes | `AZ-N-ywQ3_y5QfimGTZy` | OPEN | `go:S3776` | CRITICAL | `dnitsch@github` | `internal/config/cache_test.go:53` |
| 0 | Yes | `AZ-N-ywQ3_y5QfimGTZz` | OPEN | `go:S3776` | CRITICAL | `dnitsch@github` | `internal/config/cache_test.go:126` |
| 0 | Yes | `AZ-N-yxh3_y5QfimGTZ0` | OPEN | `go:S3776` | CRITICAL | `richards-ensono-Gqhdf@github` | `internal/config/loader_git.go:559` |
| 0 | Yes | `AZ-N-y0t3_y5QfimGTZ2` | OPEN | `go:S1192` | CRITICAL | `richards-ensono-Gqhdf@github` | `scripts/check-workflow-policy/main.go:344` |
| 0 | Yes | `AZ-N-y0t3_y5QfimGTZ7` | OPEN | `go:S3776` | CRITICAL | `richards-ensono-Gqhdf@github` | `scripts/check-workflow-policy/main.go:365` |
| 0 | Yes | `AZ-N-y0t3_y5QfimGTZ4` | OPEN | `go:S1192` | CRITICAL | `richards-ensono-Gqhdf@github` | `scripts/check-workflow-policy/main.go:387` |
| 1 | No | `AZ-N-y1d3_y5QfimGTaC` | OPEN | `godre:S8205` | MINOR | `richards-ensono-Gqhdf@github` | `scripts/materialize-sonar-source/main.go:96` |
| 1 | No | `AZ-N-y1d3_y5QfimGTaD` | OPEN | `godre:S8205` | MINOR | `richards-ensono-Gqhdf@github` | `scripts/materialize-sonar-source/main.go:102` |
| 2 | No | `AZWKA6kX03lIpGBL3rEn` | OPEN | `go:S1192` | CRITICAL | `dnitsch@github` | `cmd/eirctl/eirctl.go:74` |
| 2 | No | `AZWKA6j703lIpGBL3rEk` | OPEN | `go:S1192` | CRITICAL | `dnitsch@github` | `cmd/eirctl/init.go:59` |
| 2 | No | `AZ1t2WyVtvGp9fbwa0Kx` | OPEN | `go:S3776` | CRITICAL | `dnitsch@github` | `cmd/eirctl/eirctl_test.go:40` |
| 2 | No | `AZ1t2Wy_tvGp9fbwa0K3` | OPEN | `go:S3776` | CRITICAL | unassigned | `cmd/main_test.go:13` |
| 2 | No | `AZ1t2WvrtvGp9fbwa0Km` | OPEN | `go:S3776` | CRITICAL | `dnitsch@github` | `output/prefixed_test.go:12` |
| 2 | No | `AZ1t2WzPtvGp9fbwa0K4` | OPEN | `go:S3776` | CRITICAL | `dnitsch@github` | `variables/variables_test.go:51` |
| 2 | No | `AZbPupl0Qf_52EOIHgtb` | OPEN | `go:S3776` | CRITICAL | `dnitsch@github` | `internal/cmdutils/cmdutils.go:63` |
| 2 | No | `AZ1t2WjltvGp9fbwa0IF` | OPEN | `go:S3776` | CRITICAL | `dnitsch@github` | `internal/genci/githubimpl_test.go:58` |
| 2 | No | `AZ1t2WqptvGp9fbwa0Ir` | OPEN | `go:S3776` | CRITICAL | unassigned | `internal/utils/utils_test.go:152` |
| 2 | No | `AZWKA6je03lIpGBL3rEh` | OPEN | `go:S3776` | CRITICAL | `dnitsch@github` | `runner/compiler.go:27` |
| 2 | No | `AZWKA6je03lIpGBL3rEi` | OPEN | `go:S107` | MAJOR | `dnitsch@github` | `runner/compiler.go:84` |
| 2 | No | `AZZ8glSK1wqWcqTip8XF` | OPEN | `go:S1186` | CRITICAL | `dnitsch@github` | `runner/executor_container.go:102` |
| 2 | No | `AZxSHQDOhNFpWUf3zR1y` | OPEN | `godre:S8188` | MAJOR | `dnitsch@github` | `runner/executor_default.go:83` |
| 2 | No | `AZ1t2W0ytvGp9fbwa0K8` | OPEN | `go:S4144` | MAJOR | `dnitsch@github` | `scheduler/scheduler_test.go:263` |
| 2 | No | `AZ1t2W1PtvGp9fbwa0LE` | OPEN | `go:S3776` | CRITICAL | `dnitsch@github` | `scheduler/graph_test.go:60` |

## Tier 2 admission rule

Tier 2 addresses the user's request to include other safe Critical/Major findings, but it must not delay or broaden Tier 0/Tier 1 recovery. A Tier 2 issue is admitted only if refreshed evidence still matches this ledger, focused tests characterize the behavior, the change remains private and behavior-preserving, and the task-specific postconditions can be proven. Otherwise it receives an authenticated drift/defer disposition and moves to a separate proposal.

## Implementation records

During `/opsx-apply`, update this file with:

- refreshed pre-change API evidence;
- per-key precondition and final disposition;
- links or commit IDs for implementation slices;
- independent review records for Git SSH and workflow policy;
- focused and repository-wide command results;
- protected PR and trusted-main analysis/run URLs;
- exact final revision and gate conditions;
- residual deferred findings and rollback commits.

No credentials, private token metadata, or secret values belong in this ledger.

## Per-key execution ledger

The authenticated snapshot above is the precondition for every row. “Stop” always includes an issue that is closed/moved, a mismatched rule/component, unrelated scoped-file edits, missing characterization, or a required behavior/API/security change. Each focused check must pass and be recorded before its task can be checked.

| Key | Tier | Intended files and bounded invariant | Focused check | Stop condition |
|---|---:|---|---|---|
| `AZ-N-ywQ3_y5QfimGTZy` | 0 | `internal/config/cache_test.go`; split only setup/assertion complexity and retain success, mkdir, copy, and mock-error cases. | `go test ./internal/config -run Test_StoreInCache`; `go test ./internal/config` | Production cache behavior or any assertion would change. |
| `AZ-N-ywQ3_y5QfimGTZz` | 0 | `internal/config/cache_test.go`; retain missing-file, error, YAML/import, copy, and successful-read paths. | `go test ./internal/config -run Test_Get_fromCache`; `go test ./internal/config` | Cache ownership or close semantics must change. |
| `AZ-N-yxh3_y5QfimGTZ0` | 0 | `internal/config/loader_git.go` and tests; preserve command precedence, defaults, repeated trust paths, explicit opt-out warning, and fail-closed trust. | `go test ./internal/config`; `go test -race ./internal/config` | SSH parsing or error contract would broaden/change. |
| `AZ-N-y0t3_y5QfimGTZ2` | 0 | `scripts/check-workflow-policy/main.go`; one private `validate-build` identifier constant only. | `go test ./scripts/check-workflow-policy`; `go vet ./scripts/check-workflow-policy`; `bash scripts/check-workflow-security.sh` | Any literal has different semantics. |
| `AZ-N-y0t3_y5QfimGTZ7` | 0 | `scripts/check-workflow-policy/main.go`; explicit ordered topology phases with one-to-one predicates/errors. | policy tests, vet, workflow-security, immutable-dependencies, and CODEOWNERS checks | Any predicate/order/trust boundary cannot be mapped. |
| `AZ-N-y0t3_y5QfimGTZ4` | 0 | `scripts/check-workflow-policy/main.go`; centralize only `actions/github-script@`; retain prefix and independent SHA validation. | policy tests, vet, and workflow-security check | Prefix becomes exact matching or any pin/predicate changes. |
| `AZ-N-y1d3_y5QfimGTaC` | 1 | `scripts/materialize-sonar-source/main.go`; private named base-repository response preserves all JSON names/tags/zeros. | `go test ./scripts/materialize-sonar-source`; `go vet ./scripts/materialize-sonar-source` | Fixture decoding changes. |
| `AZ-N-y1d3_y5QfimGTaD` | 1 | Same file; smallest named head-repository response type preserves provenance/path/blob/head-recheck behavior. | materializer tests, vet, workflow-security check | API response shape or provenance logic changes. |
| `AZWKA6kX03lIpGBL3rEn` | 2 | `cmd/eirctl/eirctl.go`; private `no-summary` key constant preserves flag/default/binding/lookup. | `go test ./cmd/eirctl` | Flag/config precedence changes. |
| `AZWKA6j703lIpGBL3rEk` | 2 | `cmd/eirctl/init.go`; private `no-prompt` key constant preserves prompt behavior/binding errors. | `go test ./cmd/eirctl` | Occurrences are distinct external names. |
| `AZZ8glSK1wqWcqTip8XF` | 2 | `runner/executor_container.go`; explanatory no-op comment only. | `go test ./runner` | Interface or execution isolation would change. |
| `AZxSHQDOhNFpWUf3zR1y` | 2 | `runner/executor_default.go`; defer timeout cancel in its creation branch with unchanged timing. | `go test ./runner` | A timeout path cannot guarantee cancellation. |
| `AZ1t2WyVtvGp9fbwa0Kx` | 2 | `cmd/eirctl/eirctl_test.go`; named helpers retain callers, errors, output, and cancellation. | `go test ./cmd/eirctl` | Helper coverage/assertions weaken. |
| `AZ1t2Wy_tvGp9fbwa0K3` | 2 | `cmd/main_test.go`; isolate cases without leaking args/log state; retain 0/1/125 assertions. | `go test ./cmd -run Test_main`; `go test ./cmd` | Case inventory changes. |
| `AZ1t2WvrtvGp9fbwa0Km` | 2 | `output/prefixed_test.go`; helpers retain byte-exact header/body/blank/newline/footer checks. | `go test ./output -run TestOutput_prefixedOutputDecorator`; `go test ./output` | Exact output assertions weaken. |
| `AZ1t2WzPtvGp9fbwa0K4` | 2 | `variables/variables_test.go`; exact map/value assertion helper only. | `go test ./variables -run TestVariables_MergeV2`; `go test ./variables` | Production merge behavior changes. |
| `AZ1t2WjltvGp9fbwa0IF` | 2 | `internal/genci/githubimpl_test.go`; preserve graph/YAML/job/step ordering. | `go test ./internal/genci -run TestGenCi_GithubImpl_ordering`; `go test ./internal/genci` | Any exact order check weakens. |
| `AZ1t2WqptvGp9fbwa0Ir` | 2 | `internal/utils/utils_test.go`; exact key set must reject extras and retain non-map case. | `go test ./internal/utils -run TestMapKeys`; `go test ./internal/utils` | Non-map behavior changes. |
| `AZ1t2W1PtvGp9fbwa0LE` | 2 | `scheduler/graph_test.go`; preserve lookup, root-child, and `ErrNodeNotFound`. | focused test; `go test ./scheduler`; `go test -race ./scheduler` | Any graph assertion is lost. |
| `AZ1t2W0ytvGp9fbwa0K8` | 2 | `scheduler/scheduler_test.go`; remove only a proven duplicate or add genuine required-input coverage. | both focused tests; scheduler and race tests | Coverage is distinct. |
| `AZbPupl0Qf_52EOIHgtb` | 2 | `internal/cmdutils`; characterize byte-exact statuses/errors before status renderer extraction. | `go test ./internal/cmdutils` | Output compatibility cannot be proven. |
| `AZWKA6je03lIpGBL3rEh` | 2 | `runner/compiler.go`; helpers preserve ordering, templates, timeout/environment, interactive mutation. | focused compiler tests; `go test ./runner` | Any compile contract changes. |
| `AZWKA6je03lIpGBL3rEi` | 2 | `runner/compiler.go`; private input struct replaces ten arguments without ownership/default changes. | focused compiler tests; `go test ./runner` | Caller needs positional omission/mutation not represented by struct. |

Independent review is mandatory before marking the Git SSH and workflow-policy review tasks complete. Final issue closure and gate acceptance remain contingent on authenticated PR and trusted-main analysis.

## Implementation results

- `AZ-N-y0t3_y5QfimGTZ2` (Tier 0): verified the three uses are the expected-permission map, the debug-publication job lookup, and its dependency check. Replaced only those literals with private `debugReleaseValidateJob`; YAML key, lookup behavior, predicate order, and error text are unchanged. `gofmt`, `go test ./scripts/check-workflow-policy`, `go vet ./scripts/check-workflow-policy`, and `bash scripts/check-workflow-security.sh` passed on 2026-07-27. Local structural remediation is not a claim of Sonar closure.
- `AZ-N-y0t3_y5QfimGTZ4` (Tier 0): verified all seven uses are prefix arguments to `jobUses`/`stepWithContains`, while immutable SHA validation remains the separate `pinnedAction` predicate in `validateActions`. Replaced only these prefixes with private `githubScriptActionPrefix`; no action pin or trust predicate changed. `gofmt`, `go test ./scripts/check-workflow-policy`, `go vet ./scripts/check-workflow-policy`, and `bash scripts/check-workflow-security.sh` passed on 2026-07-27. Local structural remediation is not a claim of Sonar closure.
- `AZWKA6kX03lIpGBL3rEn` (Tier 2): verified the three uses are the flag declaration, Viper binding key, and persistent-flag lookup. Replaced only them with package-local `noSummaryFlag`; spelling, default, binding, and lookup semantics are unchanged. `gofmt` and `go test ./cmd/eirctl` passed on 2026-07-27.
- `AZZ8glSK1wqWcqTip8XF` (Tier 2): verified `WithReset` is required by `ExecutorIface`, `GetExecutorFactory` creates a new container executor for each execution, and `ContainerExecutor` has no resettable state. Added only an in-body explanatory comment. `gofmt` and `go test ./runner` passed on 2026-07-27. Local structural remediation is not a claim of Sonar closure.
- `AZxSHQDOhNFpWUf3zR1y` (Tier 2): verified `timeoutCancelFn` is created only for non-nil `job.Timeout`; `TestDefaultExecutor_Execute` exercises both a timed successful command and subsequent no-timeout execution. Moved the existing defer into the timeout branch, preserving cancellation timing for every timeout path. `gofmt` and `go test ./runner` passed on 2026-07-27. Local structural remediation is not a claim of Sonar closure.
- `AZ-N-y1d3_y5QfimGTaC` and `AZ-N-y1d3_y5QfimGTaD` (Tier 1): replaced the two anonymous pull-request repository response types with the shared private `pullRepositoryResponse`, retaining `FullName`, its `json:"full_name"` tag, and the zero-value behavior. Base/head provenance, identity, path, blob, and head-recheck logic are unchanged. `gofmt`, `go test ./scripts/materialize-sonar-source`, `go vet ./scripts/materialize-sonar-source`, and `bash scripts/check-workflow-security.sh` passed on 2026-07-27. Local structural remediation is not a claim of Sonar closure.
