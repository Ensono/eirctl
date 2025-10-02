# Imports

Eirctl supports merging of various configuration files from both local and remote sources.

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/Ensono/eirctl/refs/heads/main/schemas/schema_v1.json

import:
  - https://raw.githubusercontent.com/Ensono/eirctl/refs/heads/main/shared/infra/tf.yaml
  - git::ssh://private-repo.org/org123/shared-libs//eirctl/security/scaning.yaml?ref=v0.0.1
  - ./local/relative/to/cli/eirctl.yaml
  - /absolute/path/eirctl.yaml
```

## HTTPS

When importing over HTTPS make sure the link returns the contents directly - e.g. on Github use a `raw.githubusercontent.com` snippet.

> [!NOTE]
> Ideally it should be a .yaml extension

## GIT

When importing over Git - ideally you should use SSH with any private repositories and can use git over https with public ones if the provider does not support raw content urls like GH.

> [!IMPORTANT]
> Pattern must follow this regex `^git::(ssh|https?|file)://(.+?)//([^?]+)(?:\?ref=([^&]+))?$`

Below protocols supported with Git

- SSH
- HTTPS
- FILE

> [!TIP]
> Optional ref can be set to either point to a branch/tag/commit_sha

> [!IMPORTANT]
> The environment variable `GIT_SSH_PASSPHRASE` should be set if your SSH key is encrypted with a passphrase

## FILESYSTEM

Supports relative or absolute paths.
