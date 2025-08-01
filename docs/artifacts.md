# Artifacts

> [!NOTE]
> Removed the scrape of stdout as a default Output storage after every task

Any task can assign outputs in a form of files or a special dotenv format output which is then added to the taskctl runner and available to all the subsequent tasks.

## Dotenv

The artifacts are only useful inside pipelines, ensuring you provide a depends on will make sure the variables are ready.

The outputs are available across contexts.

Example below uses the default context to generate some stored artifact and then it is used in a container context.

> [!TIP]
> after commands are not stored as artifacts

```yaml
summary: true
debug: true
output: prefixed

contexts:
  newdocker:ctx:
    container:
      name: alpine:latest 
    envfile:
      exclude:
        # these will be excluded by default
        - PATH
        - HOME
        - TMPDIR

pipelines:
  tester:
    - task: task:one
    - task: task:two
      depends_on:
        - task:one

tasks:
  task:one:
    env:
      ORIGINAL_VAR: foo123
    command:
      - echo "task one ORIGINAL_VAR => ${ORIGINAL_VAR} should be foo123"
      - echo ORIGINAL_VAR=foo333 > .artifact.env
    after:
      # should run in a new context
      - echo ORIGINAL_VAR=shouldNOTBEUSED > .artifact.env
    artifacts:
      name: test_env_from_task_one
      path: .artifact.env
      type: dotenv

  task:two:
    command:
      - echo "task:two ORIGINAL_VAR => ${ORIGINAL_VAR} should be foo333"
    context: newdocker:ctx
```

## Env [Experimental]

When writing a command and a bash style setting of a variable or export is placed in the command along with a artifacts capture specified as `env` the downstream tasks in a pipeline are able to pick them up as environment variables

> [!IMPORTANT]
> **Experimental** - take care when using in production. currently there is no prefixing of variables across tasks in pipelines so the runner environment will just be overwritten with newly set variables

Using the prefix of `EIRCTL_TASK_OUTPUT_`

```yaml
tasks:
  run:container:nouveau:
    context: nouveau:container
    command:
      - |
        echo EIRCTL_TASK_OUTPUT_FOO=$FOO
    env:
      FOO: bar
    artifacts:
      type: env

  consume:out:
    command: |
      echo $FOO

pipelines:
  set-out:
    - task: run:container:nouveau
    - task: consume:out
      depends_on: run:container:nouveau
```

> Current limitation is that it has to be set directly in the task command and _NOT_ in a script called from command
