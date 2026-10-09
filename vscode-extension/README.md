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
- A packaged extension matching your OS/architecture (it includes `eirctl-lsp`), or a separately built `eirctl-lsp` executable
- Optional: a running `eirctl-lsp` language server for TCP mode

The language server is a separate executable, not the `eirctl` task-runner CLI. For standalone installation and Neovim/Vim setup across plugin managers, see [Editor integration](../docs/editors.adoc).

## Getting Started

1. Install a VSIX built for your platform (see below).
2. Open a workspace containing an `eirctl.yaml` root configuration.
3. Open `eirctl.yaml`, a YAML file directly inside an `eirctl/` directory, or a cached import under `.eirctl/cache/`.

The current document selector covers `**/eirctl.yaml`, `**/eirctl/*.yaml`, and `**/.eirctl/cache/*`; the workspace watcher also watches nested `eirctl/**/*.yaml`. Nested files elsewhere are not automatically selected just because discovery watches them.

Process transport starts the bundled `.bin/eirctl-lsp` relative to the extension installation directory. It does **not** search `PATH`. To use your own server, set an absolute command path and an existing working directory in settings:

```json
{
  "eirctl.languageServer.transport": "process",
  "eirctl.languageServer.command": "/home/you/.local/bin/eirctl-lsp",
  "eirctl.languageServer.args": [],
  "eirctl.languageServer.cwd": "/home/you/projects/example"
}
```

On Windows use JSON-escaped backslashes, for example `"eirctl.languageServer.command": "C:\\Users\\you\\bin\\eirctl-lsp.exe"`. The server defaults to stdio; do not add `--stdio` or TCP flags in process mode.

For TCP mode, start the server first:

```shell
eirctl-lsp -use-tcp -host 127.0.0.1 -port 11103
```

Then select `tcp` transport with host `127.0.0.1` and port `11103`. Keep the listener on loopback: this transport has no authentication or TLS.

For local testing you can use the `Debug Extension` configuration.

To build a local VSIX from the repository root, use the packaging pipeline with the target OS/architecture. The pipeline bundles the server; it requires the repository's Go/Node container contexts and a Docker-compatible runtime. POSIX shell examples:

Linux:
> `BUILD_GOARCH=amd64 BUILD_GOOS=linux eirctl run build:package:vscode:extension`
> `BUILD_GOARCH=arm64 BUILD_GOOS=linux eirctl run build:package:vscode:extension`

Mac:
> `BUILD_GOARCH=arm64 BUILD_GOOS=darwin eirctl run build:package:vscode:extension`
> `BUILD_GOARCH=amd64 BUILD_GOOS=darwin eirctl run build:package:vscode:extension`

Windows target from a POSIX shell (note `BINARY_SUFFIX`):
> `BUILD_GOARCH=amd64 BUILD_GOOS=windows BINARY_SUFFIX=.exe eirctl run build:package:vscode:extension`

From PowerShell:

```powershell
$env:BUILD_GOARCH = 'amd64'
$env:BUILD_GOOS = 'windows'
$env:BINARY_SUFFIX = '.exe'
eirctl run build:package:vscode:extension
```

Use `arm64` instead of `amd64` for a Windows ARM64 target.

Then install with `code --install-extension ./vscode-extension/eirctl-latest.vsix`.

Uninstall: `code --uninstall-extension ensono-digital-tools.eirctl`

## Extension Settings

This extension contributes the following settings:

- eirctl.languageServer.transport: Controls how the language server is reached. Use process to spawn the server or tcp to connect to an already-running server.
- eirctl.languageServer.command: Command used to start the language server when process transport is selected.
- eirctl.languageServer.args: Additional arguments passed to the language server.
- eirctl.languageServer.cwd: Working directory used when starting the server, and the base directory for relative commands. If empty, the implementation uses the extension's `.bin` directory.
- eirctl.languageServer.tcpHost: Host used for TCP transport.
- eirctl.languageServer.tcpPort: Port used for TCP transport (default `11103`).
- eirctl.languageServer.tcpRetryAttempts: Connection attempt limit for TCP startup (default `5`).
- eirctl.languageServer.tcpRetryDelayMs: Initial delay between TCP connection attempts (default setting `500` ms).

## Troubleshooting

- Inspect the **eirctl Language Server** output channel for the resolved command, working directory, and connection errors.
- If using process mode, check that the bundled server matches your platform, or that your absolute `eirctl-lsp` path exists and is executable. A CLI installed on `PATH` alone does not satisfy the extension's command resolution.
- If using a custom `cwd`, ensure it exists. Empty `cwd` uses the extension's `.bin` directory, despite the setting description currently referring to a repository root.
- If using TCP mode, confirm the server is already running on the configured loopback host/port. Retries are bounded; start the server before opening the extension.
- Set the document's language mode to YAML and check that its path matches the selector above. The server provides completion, diagnostics, hover, definitions, references, and document symbols, but not formatting or rename.

## Release Notes

### 0.0.2

- Initial VS Code extension release
- Added YAML support for eirctl files
- Added configurable language server startup
