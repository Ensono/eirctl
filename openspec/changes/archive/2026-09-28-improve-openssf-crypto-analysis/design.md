## Context

`eirctl` is a Go CLI whose release builds use `CGO_ENABLED=0`. Its Git-import path resolves a user-selected OpenSSH identity, parses it through `golang.org/x/crypto/ssh`, and uses it for Git-over-SSH authentication. The current BadgeApp record leaves `dynamic_analysis_unsafe`, `crypto_working`, `crypto_weaknesses`, and `crypto_pfs` unanswered and correctly records `crypto_keylength` as unmet because the application neither restricts nor provides a mechanism to disable undersized user keys.

The project must preserve OpenSSH configuration resolution and cannot make key selection or key length a hard failure. The requested outcome is visibility, not enforcement, and must not overstate BadgeApp compliance.

## Goals / Non-Goals

**Goals:**
- Produce truthful, durable evidence for the four applicable unanswered BadgeApp criteria.
- Warn safely when the selected SSH identity is below the BadgeApp's documented 2030 strength baseline.
- Keep normal secure-key imports and all currently supported OpenSSH identity-selection behavior working.
- Use only explicit, non-legacy SSH transport algorithms so the BadgeApp cryptographic claims describe the effective eirctl policy instead of library compatibility fallbacks.
- Provide focused, deterministic tests and an auditable delivery sequence.

**Non-Goals:**
- Do not reject, replace, regenerate, or mandate SSH identity keys on the basis of the warning-only strength assessment.
- Do not change `StrictHostKeyChecking=no`, known-host verification defaults, SSH user/config selection, or public CLI/API contracts.
- Do not retain compatibility with a Git server or identity that can authenticate only with an excluded legacy SSH algorithm.
- Do not add a hosted scanning service, change deployment settings, or represent warning-only behavior as satisfying `crypto_keylength`.
- Do not claim a BadgeApp status without current authoritative criteria and repository-backed evidence.

## Decisions

### Inspect the parsed public key after normal identity selection

After the existing SSH identity parsing succeeds, derive only the public-key algorithm and non-sensitive strength information needed for classification. Compare it with the BadgeApp 2030 minimums: 112-bit symmetric security, 2048-bit factoring modulus, 224-bit discrete-logarithm key, 2048-bit discrete-logarithm group, 224-bit elliptic curve, and 224-bit hash.

For common SSH identities, the implementation should classify RSA modulus length and elliptic-curve/Ed25519 strength directly. Unsupported, certificate-wrapped, or non-inspectable identity forms must remain usable and produce a clear non-fatal diagnostic only if their strength cannot be established safely.

**Rationale:** This observes the key actually selected through the existing OpenSSH-compatible path, without copying private material into configuration, logs, or new persistence.

**Alternatives considered:**
- Reject undersized identities: rejected because the requested behavior is warning-only and compatibility is a project constraint.
- Parse identity-file text independently before `ssh.ParsePrivateKey`: rejected because it duplicates parser behavior and risks processing private material unnecessarily.
- Change the user's OpenSSH configuration or invoke the `ssh` executable: rejected because it changes the established library-backed transport architecture and user environment behavior.

### Configure an explicit secure SSH transport policy

The Git SSH authentication implementation will use an internal `go-git` SSH
auth method that creates `ssh.ClientConfig` directly, because
`gitssh.PublicKeys` exposes only host-key algorithm configuration. The method
will preserve the selected user, parsed signer, and existing host-key callback,
then set explicit allowlists for key exchange, ciphers, MACs, host keys, and
public-key authentication.

The allowlists must use only the pinned `x/crypto/ssh` algorithms that are not
listed as insecure and must exclude SHA-1 compatibility fallbacks, SSH CBC,
RC4, DSA, and `ssh-rsa`. The public-key authentication path must not silently
fall back to `ssh-rsa`; when the selected identity cannot authenticate with a
non-legacy algorithm, the connection may fail rather than weakening the
transport policy.

**Rationale:** The library's default negotiation still includes legacy
compatibility fallbacks. An explicit eirctl policy makes the effective defaults
reviewable and supports a truthful `crypto_weaknesses` answer while preserving
identity selection and server trust verification.

**Alternatives considered:**
- Retain the library defaults and mark `crypto_weaknesses` Unmet: rejected by
  the approved expansion of this change.
- Modify user OpenSSH configuration: rejected because it changes user-managed
  configuration rather than the application's transport policy.

### Make warnings observable but non-authoritative

Emit a warning through the project's established logger when a key is below a known threshold or cannot be classified. The message must identify only the safe algorithm category, observed strength where available, the recommended baseline, and that connection behavior is unchanged; it must never include key bytes, paths, passphrases, repository URLs, tokens, or host credentials.

**Rationale:** A warning helps users act without creating an unexpected authentication outage. The retained keylength BadgeApp answer must stay `Unmet` because warning-only behavior does not meet its requirement to use secure defaults and make smaller lengths completely disableable.

**Alternatives considered:**
- Mark `crypto_keylength` Met because users receive a warning: rejected as untruthful.
- Add a silent metric: rejected because it is invisible to the user and introduces unnecessary telemetry/operational scope.

### Evidence is criterion-specific and version-aware

Update `.bestpractices.json` only for the four requested criteria after checking the current BadgeApp definitions. Justifications will link to stable repository files and distinguish:

- `dynamic_analysis_unsafe`: N/A, supported by Go-only source, absence of cgo imports, and `CGO_ENABLED=0` release builds;
- `crypto_working`: maintained, purpose-built Go cryptographic and transport libraries under secure defaults;
- `crypto_weaknesses`: the explicit SSH transport algorithm policy excludes broken or seriously weak algorithms/modes, with SHA-1 Git-object identity use explicitly excluded from production cryptographic security;
- `crypto_pfs`: default SSH key exchange and standard HTTPS/TLS transport provide forward secrecy where key agreement occurs.

The implementation will retain or clarify `crypto_keylength` as `Unmet` with a justification that accurately describes the warning-only posture.

**Rationale:** The BadgeApp answers are public assertions, so the project should record only evidence that can be independently verified and avoid equating dependency presence with an unverified runtime guarantee.

**Alternatives considered:**
- Add fuzzing to satisfy `dynamic_analysis_unsafe`: rejected for this change because the criterion permits N/A for Go-only produced software and the requested scope is minimum credible evidence.
- Set every unanswered crypto criterion to N/A: rejected because SSH and HTTPS are security mechanisms to which the crypto criteria apply.

### Publish Go vulnerability findings to GitHub code scanning

The Go vulnerability task will generate SARIF and a GitHub Actions workflow
will upload it through the official `upload-sarif` action with only the
`security-events: write` permission required for code scanning. The workflow
must upload the SARIF artifact even when `govulncheck` reports findings, while
preserving the raw scanner report rather than filtering or ignoring
vulnerability identifiers. The SARIF formatter reports findings in the artifact
for GitHub code scanning to triage; it does not retain the human-format
non-zero exit behavior.

GitHub code scanning is the system of record for vulnerability triage,
mitigation, and risk acceptance. Repository guidance may link to the relevant
GitHub finding or issue, but it must not implement a local ignore list or
otherwise remove accepted findings from `govulncheck` input or output.

**Rationale:** SARIF makes the scanner's raw findings reviewable in GitHub
Advanced Security while separating evidence-based risk decisions from the
scanner itself.

**Alternatives considered:**
- A `govulncheck` ignore file: rejected because the tool does not support
  per-finding suppression and hiding findings would weaken vulnerability
  reporting.
- Keeping OpenVEX output only: rejected because it does not provide the
  requested GitHub code-scanning triage workflow.

### Test classifications at the SSH boundary

Add table-driven tests for secure, undersized, and unclassifiable supported identity forms. Tests will assert that parsing and authentication construction continue after a warning, secure keys do not create an understrength warning, and diagnostics have no secret-bearing data. Add transport-policy tests that assert the constructed SSH client configuration excludes legacy key-exchange, cipher, MAC, host-key, and public-key authentication algorithms. Existing Git SSH host-key verification tests remain the regression boundary for `StrictHostKeyChecking=no` and known-host behavior.

**Rationale:** It verifies the requested warning-only contract without real network access or user private keys.

## Risks / Trade-offs

- **[Threshold classification differs by SSH key type]** → Keep the classifier small, key-type-aware, and covered by representative table tests; leave unsupported forms usable and explicit in diagnostics.
- **[Warnings may be missed in automated use]** → Use the established logger and document the wording/meaning in the relevant user-facing material or evidence record; do not convert the warning to a failure.
- **[Library defaults evolve]** → Cite the pinned dependency/version and re-check evidence whenever the SSH or Go runtime dependency changes.
- **[BadgeApp requirements evolve]** → Retrieve current public criterion text during implementation and retain only statuses that still meet it.
- **[User-controlled SSH compatibility remains risky]** → Preserve it by request, make understrength keys observable, and keep the BadgeApp keylength response explicitly Unmet.

## Migration Plan

1. Add the classification and warning behavior behind the existing identity-parsing flow; no configuration or persisted-state migration is required.
2. Configure and validate SARIF production and GitHub code-scanning upload while preserving raw Go vulnerability findings.
3. Run targeted tests, then the relevant repository validation suite and OpenSpec validation.
4. Review the exact BadgeApp-answer diff against current public criteria before making any status claim.
5. If warnings create unexpected automation impact, rollback by removing the warning-only change; existing identity parsing and configuration resolution remain intact.
6. After all implementation tasks and validations are complete, archive the accepted OpenSpec change, create a delivery branch, commit the bounded changes, push it, and open a pull request.

## Open Questions

- Which non-RSA SSH key forms exposed by the selected `x/crypto/ssh` version can be classified reliably enough for a strength warning, and which should be reported as unclassifiable?
- Should an unclassifiable key warn by default, or only appear as a debug-level notice? The chosen behavior must remain non-blocking and avoid confusing common certificate-wrapped identities.
- Which existing documentation location is the most durable public evidence for the Go-only release composition and the warning-only key-strength policy?
