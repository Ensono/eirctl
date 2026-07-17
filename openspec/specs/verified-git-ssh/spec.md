# Purpose

TBD: Define verified host-key behavior for Git-over-SSH connections.

# Requirements

## Requirement: Git SSH connections verify server host keys by default
The system SHALL authenticate the effective SSH server hostname and port against trusted known-host entries before exchanging Git repository data, and SHALL fail closed when verification cannot be completed.

### Scenario: Server presents a trusted key
- **WHEN** a Git SSH server presents a key matching a known-host entry for the resolved hostname and port
- **THEN** the connection proceeds to Git authentication and repository access

### Scenario: Server is unknown
- **WHEN** the resolved SSH server has no matching known-host entry
- **THEN** the connection fails with an actionable unknown-host error before repository data is accepted

### Scenario: Server key changed
- **WHEN** the SSH server presents a key that conflicts with an existing known-host entry
- **THEN** the connection fails with a host-key-mismatch error and does not silently replace or bypass the trusted key

## Requirement: Known-host configuration follows SSH configuration
The system SHALL resolve user and system known-host files using OpenSSH-compatible configuration, including the effective hostname and port after host aliases and supported `GIT_SSH_COMMAND` options are applied. Parsing SHALL preserve configured path boundaries, quoting, escaping, and platform-native path syntax.

### Scenario: SSH host alias changes destination
- **WHEN** SSH configuration maps a requested host alias to a different hostname or non-default port
- **THEN** host-key verification uses the effective destination and the correct known-host host-and-port form

### Scenario: User selects a known-host file
- **WHEN** `UserKnownHostsFile` is set through the selected SSH configuration or a supported `GIT_SSH_COMMAND` option
- **THEN** the system uses the configured file for host verification according to the documented precedence

### Scenario: Configured path contains spaces
- **WHEN** a known-host directive contains a quoted or escaped path with spaces
- **THEN** the system treats that value as the intended single path rather than splitting it into nonexistent files

### Scenario: Multiple known-host files are configured
- **WHEN** SSH configuration supplies multiple known-host files using supported OpenSSH syntax
- **THEN** the system preserves each configured path and loads the usable files in precedence order

### Scenario: No custom file is selected
- **WHEN** no custom known-host file is configured
- **THEN** the system uses readable standard user and operating-system-specific system known-host locations

### Scenario: Windows system trust file exists
- **WHEN** the program runs on Windows and the standard ProgramData OpenSSH known-host file exists
- **THEN** the system includes that file among the default system trust sources without requiring a custom override

### Scenario: No usable trust source exists
- **WHEN** no configured or default known-host source can be used
- **THEN** the connection fails with guidance for provisioning a trusted host key

## Requirement: Insecure host-key bypass is explicit and observable
The system SHALL permit host-key verification bypass only when the user explicitly configures `StrictHostKeyChecking=no`, and SHALL emit a clear warning for every connection that uses the bypass.

### Scenario: Explicit compatibility opt-out
- **WHEN** the effective SSH configuration sets `StrictHostKeyChecking=no`
- **THEN** the connection may proceed without host-key verification and the system warns that server identity is not being verified

### Scenario: Verification fails without opt-out
- **WHEN** normal host-key verification fails and no explicit opt-out is configured
- **THEN** the system returns the verification error and does not fall back to insecure behavior

## Requirement: SSH trust failures protect sensitive data
Host verification errors and warnings SHALL identify the host and corrective action without exposing private key contents, passphrases, tokens, or other credentials.

### Scenario: Host verification error is logged
- **WHEN** an SSH host is unknown or presents a mismatched key
- **THEN** diagnostic output contains safe host context and remediation guidance but no private credential material
