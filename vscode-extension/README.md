# eirctl extension for VS Code

This extension adds YAML support for eirctl configuration files and workspace discovery for eirctl-driven projects.

## Features

- Rich YAML editing support for eirctl configuration files
- Workspace detection for eirctl.yaml and eirctl/**/*.yaml
- Language server integration for validation and editor features
- Configurable LSP startup via process or TCP transport
- Cross-platform support for Linux, macOS, and Windows

## Requirements

- VS Code 1.125.0 or newer
- eirctl installed and available on your PATH when using process-based LSP startup
- Optional: a running eirctl language server for TCP mode

## Getting Started

1. Install the extension.
2. Open a workspace containing an eirctl.yaml file or eirctl configuration directories.
3. Use the default settings or customize the language server behavior in VS Code settings.

## Extension Settings

This extension contributes the following settings:

- eirctl.languageServer.transport: Controls how the language server is reached. Use process to spawn the server or tcp to connect to an already-running server.
- eirctl.languageServer.command: Command used to start the language server when process transport is selected.
- eirctl.languageServer.args: Additional arguments passed to the language server.
- eirctl.languageServer.cwd: Working directory used when starting the server.
- eirctl.languageServer.tcpHost: Host used for TCP transport.
- eirctl.languageServer.tcpPort: Port used for TCP transport.

## Troubleshooting

- If the language server fails to start, check that the configured command exists and is executable.
- If using process mode, ensure eirctl is installed and available in PATH.
- If using TCP mode, confirm the server is already running and reachable.

## Release Notes

### 0.0.2

- Initial VS Code extension release
- Added YAML support for eirctl files
- Added configurable language server startup
