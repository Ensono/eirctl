# Installation

## Getting started

You can install a prebuilt binary for your system from Github, see below the details.

### Install

`eirctl` is compiled for all major platforms (Windows, Mac, Linux) and architectures (`amd64`, `arm64`) and downloadable from [GitHub Releases](https://github.com/Ensono/eirctl/releases).

#### Linux  binary

> [!TIP]
> Windows Subsystem for Linux users can use the linux instructions from within a WSL terminal.

```bash
curl -L https://github.com/Ensono/eirctl/releases/latest/download/eirctl-linux-amd64 -o eirctl
```

#### MacOS binary

```bash
curl -L https://github.com/Ensono/eirctl/releases/latest/download/eirctl-darwin-arm64 -o eirctl
```

For Linux and Mac users these files can be installed system wide by placing them in `/usr/local/bin` this should be in the pat for any normal user:

```bash
chmod +x eirctl
sudo mv eirctl /usr/local/bin
```

It is also possible to install on a per-user basis by copying to `~/.local/bin`:

```sh
chmod +x eirctl
mv eirctl $HOME/.local/bin
```

#### Windows Binary

Windows binaries for your platform can be downloaded manually, or via pwsh (`iwr` or similar) then moved to a `$env:PATH` on your computer.

> [!IMPORTANT]
> Ideally use a path under your user's home directory, create one specifically for portable binaries e.g. `C:\Users\$USERNAME\bin` or use an existing one from `$env:PATH`.

```pwsh
$binDir = "${env:HOMEDRIVE}${env:HOMEPATH}\bin";
New-Item -Path $binDir -ItemType Directory -ErrorAction SilentlyContinue;
Invoke-WebRequest -Uri https://github.com/Ensono/eirctl/releases/latest/download/eirctl-windows-amd64.exe -OutFile "${binDir}\eirctl.exe";
$env:PATH="${env:PATH};${binDir}"
```

```sh
https://github.com/Ensono/eirctl/releases/latest/download/eirctl-windows-amd64.exe
https://github.com/Ensono/eirctl/releases/latest/download/eirctl-windows-arm64.exe
https://github.com/Ensono/eirctl/releases/latest/download/eirctl-windows-386.exe
```

### Verify installation

```bash
eirctl --version
```

```output
eirctl version v0.7.5-332295a31e95686cc9b20376d23a38cc98b45a00
```

### Different Architectures

Ensure that you download the correct version for the target architecture: amd64 (i.e. Intel / AMD) or ARM (i.e. Mac M1), if needed adjust the command as follows:

#### ARM (i.e. Mac M\[1-4\])

```bash
curl -L https://github.com/Ensono/eirctl/releases/latest/download/eirctl-darwin-arm64 -o eirctl
```

#### AMD (i.e. Intel Mac)

```bash
curl -L https://github.com/Ensono/eirctl/releases/latest/download/eirctl-darwin-amd64 -o eirctl
```

### Usage

- `eirctl` - run interactive task prompt
- `eirctl --help|-h` - see available commands and options
