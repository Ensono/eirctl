## 1. Confirm authoritative evidence and warning policy

- [x] 1.1 Retrieve and record the current authoritative BadgeApp requirements for `dynamic_analysis_unsafe`, `crypto_keylength`, `crypto_working`, `crypto_weaknesses`, and `crypto_pfs`; confirm that warning-only handling leaves `crypto_keylength` Unmet.
- [x] 1.2 Inspect the pinned Go and `golang.org/x/crypto/ssh` behavior needed to support each proposed BadgeApp justification, including SSH key exchange defaults and supported public-key metadata.
- [x] 1.3 Define the non-blocking warning classification for RSA, supported elliptic-curve, Ed25519, and safely unclassifiable SSH identities using the documented 2030 baseline; confirm no diagnostic can expose credential material.

## 2. Implement warning-only SSH identity assessment

- [x] 2.1 Add an internal SSH public-key strength classifier that operates only after the existing identity parser succeeds and returns safe algorithm/strength classification data.
- [x] 2.2 Add an internal Git SSH authentication method that preserves the selected user, signer, and host-key callback while setting explicit non-legacy key-exchange, cipher, MAC, host-key, and public-key authentication algorithm allowlists.
- [x] 2.3 Integrate non-blocking warnings for undersized and unclassifiable identities into the hardened Git SSH authentication path without changing identity selection, parsing, known-host verification, or `StrictHostKeyChecking=no` behavior.
- [x] 2.4 Add focused table-driven tests for secure, undersized, and unclassifiable identities; assert that warnings disclose no private key, passphrase, token, repository URL, or identity-file path, authentication construction remains usable, and the resulting client configuration excludes legacy fallback algorithms.

## 3. Update Best Practices evidence

- [x] 3.1 Add or update durable repository evidence for the Go-only release composition and warning-only SSH key-strength posture, using only statements verified from the implementation and build configuration.
- [x] 3.2 Update `.bestpractices.json` with criterion-specific, current, reviewable statuses and justifications for `dynamic_analysis_unsafe`, `crypto_working`, `crypto_weaknesses`, and `crypto_pfs`.
- [x] 3.3 Retain `crypto_keylength` as `Unmet` and update its justification, if needed, to state that warnings do not disable or reject undersized keys.
- [x] 3.4 Validate the resulting Best Practices answer file against the same current criterion snapshot used for the evidence review.

## 4. Publish Go vulnerability findings for GitHub triage

- [x] 4.1 Configure the Go vulnerability scan to emit SARIF, upload the report to GitHub code scanning with the least required permission, and preserve findings without repository-local suppression.
- [x] 4.2 Verify that a reported Go vulnerability produces valid SARIF locally and that the workflow is configured to upload it while the raw scanner result remains visible for GitHub triage.

## 5. Validate the completed change

- [x] 5.1 Run focused SSH identity tests and the relevant full Go test, lint, vulnerability, and workflow/security validation checks without weakening existing assertions or controls.
- [x] 5.2 Validate the OpenSpec change and review the complete diff for correctness, compatibility preservation, truthful BadgeApp claims, and absence of secrets or private data.
- [x] 5.3 Confirm that all implementation tasks are complete and that the working tree contains only the approved change scope.

## 6. Archive and deliver the accepted change

- [x] 6.1 Archive the completed OpenSpec change after implementation and validation are accepted, preserving its final specifications and evidence.
- [x] 6.2 Create a dedicated delivery branch from the approved base, commit the complete bounded change with GPG signing, and run configured pre-commit checks before committing when present.
- [x] 6.3 Push the delivery branch and create a pull request that summarizes the warning-only compatibility posture, BadgeApp answer changes, validation results, and any remaining `crypto_keylength` limitation.
