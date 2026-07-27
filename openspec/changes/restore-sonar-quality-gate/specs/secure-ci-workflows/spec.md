## MODIFIED Requirements

### Requirement: SonarCloud analysis covers trusted main and every pull request
The CI system SHALL submit SonarCloud analysis for every trusted push to `main` and every pull-request revision targeting `main`, including revisions originating from forks, SHALL wait for the configured SonarCloud quality gate without exposing protected credentials to the untrusted build workflow, and SHALL require the exact protected pull-request revision to pass that gate after remediating the six Tier 0 keys normatively identified in `evidence.md`: `AZ-N-ywQ3_y5QfimGTZy`, `AZ-N-ywQ3_y5QfimGTZz`, `AZ-N-yxh3_y5QfimGTZ0`, `AZ-N-y0t3_y5QfimGTZ2`, `AZ-N-y0t3_y5QfimGTZ7`, and `AZ-N-y0t3_y5QfimGTZ4`.

#### Scenario: Trusted main push is analyzed
- **WHEN** tests succeed for a trusted push to `main`
- **THEN** the workflow loads settings for organization `ensono` and the bound `Ensono_eirctl` project whose main branch is `main`, generates the configured Go reports, runs the approved SonarCloud scanner successfully, and waits for the quality-gate result

#### Scenario: Same-repository pull request is analyzed
- **WHEN** the untrusted pull-request workflow completes for a branch in `Ensono/eirctl` after the protected report-layout and coverage-namespace corrections are active on `main`
- **THEN** the default-branch analyzer submits analysis for the exact current pull-request head SHA, imports the expected Go coverage with no unresolved report paths, and SonarCloud decorates that pull request

#### Scenario: Fork pull request is analyzed
- **WHEN** the untrusted pull-request workflow completes for a fork-originated revision targeting `main`
- **THEN** the default-branch analyzer submits analysis for the exact fork revision without passing `SONAR_TOKEN` to the fork workflow or executing fork-controlled content

#### Scenario: Pull-request tests do not produce coverage
- **WHEN** the upstream pull-request run completes without the expected coverage report
- **THEN** the analyzer produces the explicitly configured source-only or failed-preparation outcome and does not silently skip SonarCloud reporting

#### Scenario: Coverage imports but another quality-gate condition fails
- **WHEN** the scanner loads the expected Go coverage report, resolves its file paths, and submits analysis, but SonarCloud fails the quality gate on a different condition such as a code smell
- **THEN** the workflow fails visibly on the reported condition, retains the verified report path and `source/` namespace, and requires remediation without issue suppression, threshold reduction, or ruleset bypass

#### Scenario: Previous-version period contains mixed quality debt
- **WHEN** authenticated Sonar issue data for a broad previous-version new-code period contains change-attributable Critical smells and documented earlier findings that continue to fail the trusted-main quality gate
- **THEN** implementation remediates the change-attributable findings and the explicitly selected pre-existing gate blockers through issue-scoped fixes, leaves unrelated historical debt outside the change, records authenticated issue evidence, and does not suppress, accept, reclassify, dismiss, exclude, or bypass any finding

#### Scenario: Remediated protected pull-request revision passes
- **WHEN** all authenticated Critical findings selected as gate blockers are remediated and the resulting exact pull-request revision is analyzed by the protected workflow
- **THEN** coverage still imports through the verified report path and `source/` namespace, the unchanged `new_code_smells_severity` condition is below its configured threshold for the unchanged new-code period, every other configured condition remains satisfied, and the overall SonarCloud quality gate reports passing
