package config_test

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/Ensono/eirctl/internal/config"
	"github.com/Ensono/eirctl/scheduler"
)

func TestBuildPipeline_Cyclical(t *testing.T) {

	var cyclicalYaml = `pipelines:
  pipeline1:
    - task: task1
      name: task1
      depends_on:
        - last-stage
      dir: "/root"
    - task: task2
      name: task2
      depends_on:
        - task1
      env: {}
    - task: task3
      name: last-stage
      depends_on:
        - task2

tasks:
  task1:
    name: task1
  task2:
    name: task2
  task3:
    name: task3
`

	file, cleanUp := configLoaderTestHelper(t, cyclicalYaml)
	defer cleanUp()
	cl := config.NewConfigLoader(config.NewConfig())
	_, err := cl.Load(file)
	if !errors.Is(err, scheduler.ErrCycleDetected) {
		t.Errorf("cycles detection failed")
	}
}

func TestBuildPipeline_Error(t *testing.T) {
	t.Run("no such task", func(t *testing.T) {
		var errorYaml = `pipelines:
  pipeline1:
    - task: task4
      name: task4
      depends_on:
        - last-stage
      dir: "/root"
tasks:
  task1:
    name: task1
  task2:
    name: task2
  task3:
    name: task3
`

		file, cleanUp := configLoaderTestHelper(t, errorYaml)
		defer cleanUp()
		cl := config.NewConfigLoader(config.NewConfig())
		_, err := cl.Load(file)
		if err == nil || !strings.Contains(err.Error(), "no such task") {
			t.Error()
		}
	})
	t.Run("no such pipeline", func(t *testing.T) {
		var errorYaml = `pipelines:
  pipeline1:
    - pipeline: task4
      name: task4
      depends_on:
        - last-stage
      dir: "/root"
tasks:
  task1:
    name: task1
  task2:
    name: task2
  task3:
    name: task3
`

		file, cleanUp := configLoaderTestHelper(t, errorYaml)
		defer cleanUp()
		cl := config.NewConfigLoader(config.NewConfig())
		_, err := cl.Load(file)
		if err == nil || !strings.Contains(err.Error(), "no such pipeline") {
			t.Error()
		}
	})
	t.Run("stage with same name", func(t *testing.T) {
		var errorYaml = `pipelines:
  pipeline1:
    - task: task1
      name: task1
      depends_on:
        - last-stage
      dir: "/root"
    - task: task1
      name: task1
      depends_on:
        - last-stage
      dir: "/root"
tasks:
  task1:
    name: task1
  task2:
    name: task2
  task3:
    name: task3
`
		file, cleanUp := configLoaderTestHelper(t, errorYaml)
		defer cleanUp()
		cl := config.NewConfigLoader(config.NewConfig())
		_, err := cl.Load(file)
		if err == nil || !strings.Contains(err.Error(), "stage with same name") {
			t.Error()
		}
	})
}

func TestConfig_TaskLoader(t *testing.T) {
	t.Run("task variables with string values", func(t *testing.T) {
		yaml := `
tasks:
  task1:
    command:
      - echo hello
    variables:
      DockerEntrypoint: /bin/bash
      Version: "1.2.3"
`
		file, cleanUp := configLoaderTestHelper(t, yaml)
		defer cleanUp()
		cl := config.NewConfigLoader(config.NewConfig())
		cfg, err := cl.Load(file)
		if err != nil {
			t.Fatalf("unexpected load error: %v", err)
		}
		task, ok := cfg.Tasks["task1"]
		if !ok {
			t.Fatal("task1 not found in config")
		}
		if got := task.Variables.Get("DockerEntrypoint"); got != "/bin/bash" {
			t.Errorf("DockerEntrypoint = %v, want /bin/bash", got)
		}
		if got := task.Variables.Get("Version"); got != "1.2.3" {
			t.Errorf("Version = %v, want 1.2.3", got)
		}
	})

	t.Run("task variables with sequence (list) value", func(t *testing.T) {
		yaml := `
tasks:
  task1:
    command:
      - echo hello
    variables:
      DockerEntrypoint: /bin/bash
      TestCmdList:
        - Cmd: php --version
          Path: php_v
        - Cmd: php -m
          Path: php_m
        - Cmd: configmanager --version
          Path: cfgmgr
`
		file, cleanUp := configLoaderTestHelper(t, yaml)
		defer cleanUp()
		cl := config.NewConfigLoader(config.NewConfig())
		cfg, err := cl.Load(file)
		if err != nil {
			t.Fatalf("unexpected load error (sequence variable should be supported): %v", err)
		}
		task, ok := cfg.Tasks["task1"]
		if !ok {
			t.Fatal("task1 not found in config")
		}
		if got := task.Variables.Get("DockerEntrypoint"); got != "/bin/bash" {
			t.Errorf("DockerEntrypoint = %v, want /bin/bash", got)
		}
		list, ok := task.Variables.Get("TestCmdList").([]any)
		if !ok {
			t.Fatalf("TestCmdList is %T, want []any", task.Variables.Get("TestCmdList"))
		}
		if len(list) != 3 {
			t.Fatalf("TestCmdList length = %d, want 3", len(list))
		}
		first, ok := list[0].(config.VariablesVarMapType)
		if !ok {
			t.Fatalf("TestCmdList[0] is %T, want config.VariablesVarMapType", list[0])
		}
		if first["Cmd"] != "php --version" {
			t.Errorf("TestCmdList[0].Cmd = %v, want 'php --version'", first["Cmd"])
		}
		if first["Path"] != "php_v" {
			t.Errorf("TestCmdList[0].Path = %v, want 'php_v'", first["Path"])
		}
	})

	t.Run("task correctly built from config using envfile as well as env keys", func(t *testing.T) {
		tmpEnv, _ := os.CreateTemp("", "*.env")
		defer os.Remove(tmpEnv.Name())
		_, _ = tmpEnv.Write([]byte(`FOO=taskX
ANOTHER_VAR=moo`))

		yamlTasks := fmt.Sprintf(`
contexts:
  podman:
    container:
      name: podman:latest
tasks:
  task-p2:1:
    command:
      - |
        echo "hello, p2 ${FOO} env: ${ENV_NAME:-unknown}"
    context: podman
    env:
      FOO: task1
      GLOBAL_VAR: overwritteninTask
    envfile:
      path: %s

  task-p2:2:
    command:
      - |
        for i in $(seq 1 5); do
          echo "hello, p2 ${FOO} - env: ${ENV_NAME:-unknown} - iteration $i"
          sleep 0
        done
    env:
      FOO: task2`, tmpEnv.Name())
		file, cleanUp := configLoaderTestHelper(t, yamlTasks)
		defer cleanUp()
		cl := config.NewConfigLoader(config.NewConfig())
		eirctlCfg, err := cl.Load(file)
		if err != nil {
			t.Fatal(err)
		}
		val, ok := eirctlCfg.Tasks["task-p2:1"]
		if !ok {
			t.Error("failed to add task to config")
		}
		if val.EnvFile == nil {
			t.Fatal("failed to read the env file")
		}
		if val.EnvFile.PathValue[0] != tmpEnv.Name() {
			t.Error("incorrect env file name")
		}
	})
}

func TestBuildPipeline_Variables(t *testing.T) {
	t.Run("pipeline stage variables with string values", func(t *testing.T) {
		yaml := `
pipelines:
  pipeline1:
    - task: task1
      name: task1
      variables:
        DockerEntrypoint: /bin/bash
        Version: "1.2.3"
tasks:
  task1:
    command:
      - echo hello
`
		file, cleanUp := configLoaderTestHelper(t, yaml)
		defer cleanUp()
		cl := config.NewConfigLoader(config.NewConfig())
		cfg, err := cl.Load(file)
		if err != nil {
			t.Fatalf("unexpected load error: %v", err)
		}
		stage, err := cfg.Pipelines["pipeline1"].Node("task1")
		if err != nil {
			t.Fatalf("stage not found: %v", err)
		}
		if got := stage.Variables().Get("DockerEntrypoint"); got != "/bin/bash" {
			t.Errorf("DockerEntrypoint = %v, want /bin/bash", got)
		}
		if got := stage.Variables().Get("Version"); got != "1.2.3" {
			t.Errorf("Version = %v, want 1.2.3", got)
		}
	})

	t.Run("pipeline stage variables with sequence (list) value", func(t *testing.T) {
		yaml := `
pipelines:
  pipeline1:
    - task: task1
      name: task1
      variables:
        DockerEntrypoint: /bin/bash
        TestCmdList:
          - Cmd: php --version
            Path: php_v
          - Cmd: php -m
            Path: php_m
          - Cmd: configmanager --version
            Path: cfgmgr
tasks:
  task1:
    command:
      - echo hello
`
		file, cleanUp := configLoaderTestHelper(t, yaml)
		defer cleanUp()
		cl := config.NewConfigLoader(config.NewConfig())
		cfg, err := cl.Load(file)
		if err != nil {
			t.Fatalf("unexpected load error (sequence variable should be supported): %v", err)
		}
		stage, err := cfg.Pipelines["pipeline1"].Node("task1")
		if err != nil {
			t.Fatalf("stage not found: %v", err)
		}
		if got := stage.Variables().Get("DockerEntrypoint"); got != "/bin/bash" {
			t.Errorf("DockerEntrypoint = %v, want /bin/bash", got)
		}
		list, ok := stage.Variables().Get("TestCmdList").([]any)
		if !ok {
			t.Fatalf("TestCmdList is %T, want []any", stage.Variables().Get("TestCmdList"))
		}
		if len(list) != 3 {
			t.Fatalf("TestCmdList length = %d, want 3", len(list))
		}
		first, ok := list[0].(config.VariablesVarMapType)
		if !ok {
			t.Fatalf("TestCmdList[0] is %T, want config.VariablesVarMapType", list[0])
		}
		if first["Cmd"] != "php --version" {
			t.Errorf("TestCmdList[0].Cmd = %v, want 'php --version'", first["Cmd"])
		}
		if first["Path"] != "php_v" {
			t.Errorf("TestCmdList[0].Path = %v, want 'php_v'", first["Path"])
		}
	})
}

func configLoaderTestHelper(t *testing.T, configInput string) (file string, cleanUp func()) {
	t.Helper()
	tmpfile, _ := os.CreateTemp(os.TempDir(), "config-pipeline-*.yml")

	_ = os.WriteFile(tmpfile.Name(), []byte(configInput), 0777)

	return tmpfile.Name(), func() {
		os.Remove(tmpfile.Name())
	}
}
