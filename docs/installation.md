# Installation

## Getting started

You can install a prebuilt binary for your system from Github, see below the details.

### Install

Major platform binaries [here](https://github.com/Ensono/eirctl/releases)

*nix binary

```bash
curl -L https://github.com/Ensono/eirctl/releases/latest/download/eirctl-linux-amd64 -o eirctl
```

MacOS binary

```bash
curl -L https://github.com/Ensono/eirctl/releases/latest/download/eirctl-darwin-arm64 -o eirctl
```

```bash
chmod +x eirctl
sudo mv eirctl /usr/local/bin
```

Windows binaries for your platform can be downloaded manually, or via pwsh (`iwr` or similar) then moved to a `$env:PATH` on your computer.

> Ideally on a path under your user, create one specifically for portable binaries e.g. `C:\Users\<yourname>\bin` or use an existing one from `$env:PATH`.

```sh
https://github.com/Ensono/eirctl/releases/latest/download/eirctl-windows-386.exe
https://github.com/Ensono/eirctl/releases/latest/download/eirctl-windows-amd64.exe
https://github.com/Ensono/eirctl/releases/latest/download/eirctl-windows-arm64.exe
```

Verify installation

```bash
eirctl --version
```

Download specific version:

_ARM_:

```bash
curl -L https://github.com/Ensono/eirctl/releases/download/0.7.2/eirctl-darwin-arm64 -o eirctl
```

_AMD_:

```bash
curl -L https://github.com/Ensono/eirctl/releases/download/0.7.2/eirctl-darwin-amd64 -o eirctl
```

### Usage

- `eirctl` - run interactive task prompt
- `eirctl --help|-h` - see available commands and options
