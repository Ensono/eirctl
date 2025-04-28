# Installation

## Getting started

You can install a prebuilt binary for your system from Github, see below the details.

### Install

Major platform binaries [here](https://github.com/Ensono/eirctl/releases)

*nix binary

```bash
curl -L https://github.com/Ensono/eirctl/releases/latest/download/eirctl-linux-`uname -m` -o eirctl
```

MacOS binary

```bash
curl -L https://github.com/Ensono/eirctl/releases/latest/download/eirctl-darwin-`uname -m` -o eirctl
```

```bash
chmod +x eirctl
sudo mv eirctl /usr/local/bin
```

Windows binaries for your platform can be downloaded manually, or via pwsh (`iwr` or similar) then moved to a $PATH on your computer.

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

```bash
curl -L https://github.com/Ensono/eirctl/releases/download/0.3.7/eirctl-darwin-`uname -m` -o eirctl
```

### Usage

- `eirctl` - run interactive task prompt
- `eirctl pipeline1` - run single pipeline
- `eirctl task1` - run single task
- `eirctl pipeline1 task1` - run one or more pipelines and/or tasks
- `eirctl watch watcher1 watcher2` - start one or more watchers
