# Shared Definitions

These are re-useable chunks of contexts or tasks that can be imported via the URL into other projects to avoid repetition.

You can easily see imported contexts/pipelines/tasks by running the `eirctl list` and `eirctl show <task|pipeline|context>` command.

## TF Shared Example

Reusable tasks and contexts defined in the [shared/infra/tf.yaml](./infra/tf.yaml) can be used like this in a consumer eirctl yaml

```yaml
import:
  - https://raw.githubusercontent.com/Ensono/eirctl/e0e0d9abc998f58930d1a2fb371b17456e8b3230/shared/infra/tf.yaml

pipelines:
  run:infra:workspace:
    - task: eirctl:tf:init
      env:
        TF_DIR: aws/infra
    - task: eirctl:tf:plan:workspace
      depends_on:
        - eirctl:tf:init
      env:
        TF_DIR: aws/infra
        WORKSPACE: test1
    - task: eirctl:tf:apply:workspace
      depends_on:
        - eirctl:tf:plan:workspace
      env:
        TF_DIR: aws/infra
```
