package cmd_test

import (
	"fmt"
	"os"
	"runtime"
	"testing"
)

func Test_runCommand(t *testing.T) {
	t.Run("errors on graph:task4", func(t *testing.T) {
		cmdRunTestHelper(t, &cmdRunTestInput{args: []string{"-c", "testdata/graph.yaml", "run", "graph:task4", "--raw"}, errored: true})
	})

	t.Run("no task or pipeline supplied", func(t *testing.T) {
		cmdRunTestHelper(t, &cmdRunTestInput{args: []string{"-c", "testdata/graph.yaml", "run", "graph:task4", "--raw"}, errored: true})
	})

	t.Run("correct output assigned without specifying task or pipeline", func(t *testing.T) {
		os.Setenv("EIRCTL_CONFIG_FILE", "testdata/graph.yaml")
		defer os.Unsetenv("EIRCTL_CONFIG_FILE")
		cmdRunTestHelper(t, &cmdRunTestInput{args: []string{"run", "graph:task1", "--raw"}, exactOutput: "hello, world!\n"})
	})

	t.Run("correct with task specified", func(t *testing.T) {
		cmdRunTestHelper(t, &cmdRunTestInput{args: []string{"-c", "testdata/graph.yaml", "run", "task", "graph:task1", "--raw"}, exactOutput: "hello, world!\n"})
	})
	t.Run("correct with pipeline specified", func(t *testing.T) {
		cmdRunTestHelper(t, &cmdRunTestInput{args: []string{"-c", "testdata/graph.yaml", "run", "pipeline", "graph:pipeline1", "--raw"}, output: []string{"hello, world!\n"}})
	})
	t.Run("correct prefixed output", func(t *testing.T) {
		t.Setenv("EIRCTL_CONFIG_FILE", "testdata/graph.yaml")
		cmdRunTestHelper(t, &cmdRunTestInput{args: []string{"--output=prefixed", "run", "graph:pipeline1"}, output: []string{"graph:task1", "graph:task2", "graph:task3", "hello, world!"}})
	})

	t.Run("correct with graph-only - denormalized", func(t *testing.T) {
		t.Setenv("EIRCTL_CONFIG_FILE", "testdata/generate.yml")
		cmdRunTestHelper(t, &cmdRunTestInput{
			args: []string{"run", "graph:pipeline1", "--graph-only"},
			output: []string{`[label="graph:pipeline1->dev_anchor",shape="point",style="invis"]`,
				`[label="graph:pipeline1->graph:pipeline3_anchor",shape="point",style="invis"]`, `label="graph:pipeline1->prod"`,
			},
		})
	})
	t.Run("correctly records and outputs default vars", func(t *testing.T) {
		cmdRunTestHelper(t, &cmdRunTestInput{
			args:        []string{"-c", "testdata/task.yaml", "run", "task", "task:check:default:vars", "--raw"},
			exactOutput: fmt.Sprintf("os: %s arch: %s", runtime.GOOS, runtime.GOARCH),
		})
	})
}

func Test_runCommandWithArgumentsList(t *testing.T) {
	t.Run("with args - first arg - config from env var", func(t *testing.T) {
		t.Setenv("EIRCTL_CONFIG_FILE", "testdata/task.yaml")
		cmdRunTestHelper(t, &cmdRunTestInput{args: []string{"run", "task", "task:task1", "--raw", "--", "first", "second"}, exactOutput: "This is first argument\n"})
	})
	t.Run("with args - second arg", func(t *testing.T) {
		cmdRunTestHelper(t, &cmdRunTestInput{args: []string{"-c", "testdata/task.yaml", "run", "task", "task:task2", "--raw", "--", "first", "second"}, exactOutput: "This is second argument\n"})
	})
	t.Run("with argsList - - first and second arg", func(t *testing.T) {
		cmdRunTestHelper(t, &cmdRunTestInput{args: []string{"-c", "testdata/task.yaml", "run", "task", "task:task3", "--raw", "--", "first", "and", "second"}, exactOutput: "This is first and second arguments\n"})
	})

	t.Run("run with --set Var ", func(t *testing.T) {
		cmdRunTestHelper(t, &cmdRunTestInput{args: []string{"-c", "testdata/task.yaml", "run", "task", "task:requiredVar", "--set", "SetMe=HasBeenSet"}, errored: false, exactOutput: "HasBeenSet\n"})
	})
	t.Run("run without --set Var ", func(t *testing.T) {
		t.Setenv("EIRCTL_CONFIG_FILE", "testdata/task.yaml")
		cmdRunTestHelper(t, &cmdRunTestInput{args: []string{"-c", "testdata/task.yaml", "run", "task", "task:requiredVar", "--set", "SetNOT=HasBeenSet"}, errored: true})
	})
}

func Test_errors_on_run(t *testing.T) {
	t.Run("task not found", func(t *testing.T) {
		t.Setenv("EIRCTL_CONFIG_FILE", "testdata/task.yaml")
		cmdRunTestHelper(t, &cmdRunTestInput{args: []string{"-c", "testdata/task.yaml", "run", "pipeline", "error:task", "--raw", "--", "first", "second"}, errored: true, exactOutput: `error:task does not exist, ensure your first argument is the name of the pipeline or task. supplied argument does not match any pipelines or tasks`})
	})

	t.Run("pipeline not found", func(t *testing.T) {
		t.Setenv("EIRCTL_CONFIG_FILE", "testdata/task.yaml")
		cmdRunTestHelper(t, &cmdRunTestInput{args: []string{"run", "pipeline", "not:found", "--raw", "--", "first", "second"}, errored: true})
	})

	t.Run("errors inside task", func(t *testing.T) {
		cmdRunTestHelper(t, &cmdRunTestInput{args: []string{"-c", "testdata/graph.yaml", "run", "error:task", "--raw", "--no-summary"}, errored: true, output: []string{"exit status 1"}})
	})

	t.Run("errors inside task 2", func(t *testing.T) {
		cmdRunTestHelper(t, &cmdRunTestInput{args: []string{"-c", "testdata/graph.yaml", "run", "error:task2", "--raw", "--no-summary"}, errored: false})
	})

	t.Run("run errors on config not found", func(t *testing.T) {
		cmdRunTestHelper(t, &cmdRunTestInput{args: []string{"-c", "testdata/task-notfound.yaml", "run", "error", "--raw", "--", "first", "second"}, errored: true})
	})

	t.Run("run pipeline errors on config not found", func(t *testing.T) {
		cmdRunTestHelper(t, &cmdRunTestInput{args: []string{"-c", "testdata/task-notfound.yaml", "run", "pipeline", "error", "--raw", "--", "first", "second"}, errored: true})
	})
	t.Run("run task errors on config not found", func(t *testing.T) {
		cmdRunTestHelper(t, &cmdRunTestInput{args: []string{"-c", "testdata/task-notfound.yaml", "run", "task", "error", "--raw", "--", "first", "second"}, errored: true})
	})

	t.Run("pipeline on missing required in task", func(t *testing.T) {
		cmdRunTestHelper(t, &cmdRunTestInput{args: []string{"-c", "testdata/graph.yaml", "run", "pipeline", "missing:required:env"}, errored: true, output: []string{"missing required input", "FOO"}})
	})

}

// func Test_run_check(t *testing.T) {
// 	ttests := map[string]struct {
// 		objType any
// 	}{
// 		"test1": {
// 			objType: nil,
// 		},
// 	}
// 	for name, tt := range ttests {
// 		t.Run(name, func(t *testing.T) {

// 		})
// 	}
// }
