## MODIFIED Requirements

### Requirement: Known-host configuration follows SSH configuration
The system SHALL resolve user and system known-host files using OpenSSH-compatible configuration, including the effective hostname and port after host aliases and supported `GIT_SSH_COMMAND` options are applied. Parsing SHALL preserve configured path boundaries, quoting, escaping, and platform-native path syntax.

#### Scenario: SSH host alias changes destination
- **WHEN** SSH configuration maps a requested host alias to a different hostname or non-default port
- **THEN** host-key verification uses the effective destination and the correct known-host host-and-port form

#### Scenario: User selects a known-host file
- **WHEN** `UserKnownHostsFile` is set through the selected SSH configuration or a supported `GIT_SSH_COMMAND` option
- **THEN** the system uses the configured file for host verification according to the documented precedence

#### Scenario: Configured path contains spaces
- **WHEN** a known-host directive contains a quoted or escaped path with spaces
- **THEN** the system treats that value as the intended single path rather than splitting it into nonexistent files

#### Scenario: Multiple known-host files are configured
- **WHEN** SSH configuration supplies multiple known-host files using supported OpenSSH syntax
- **THEN** the system preserves each configured path and loads the usable files in precedence order

#### Scenario: No custom file is selected
- **WHEN** no custom known-host file is configured
- **THEN** the system uses readable standard user and operating-system-specific system known-host locations

#### Scenario: Windows system trust file exists
- **WHEN** the program runs on Windows and the standard ProgramData OpenSSH known-host file exists
- **THEN** the system includes that file among the default system trust sources without requiring a custom override

#### Scenario: No usable trust source exists
- **WHEN** no configured or default known-host source can be used
- **THEN** the connection fails with guidance for provisioning a trusted host key
