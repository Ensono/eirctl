package runner_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/Ensono/eirctl/internal/utils"
	"github.com/Ensono/eirctl/runner"
	"github.com/Ensono/eirctl/task"
	"github.com/Ensono/eirctl/variables"
)

var shBin = utils.Binary{
	Bin:  "/bin/sh",
	Args: []string{"-c"},
}

var envFile = utils.NewEnvFile()

func TestTaskCompiler_CompileTask_WithRequired_Runtime(t *testing.T) {
	t.Parallel()

	t.Run("required env has been set", func(t *testing.T) {

		tc := runner.NewTaskCompiler()

		excontextEnvMap := variables.FromMap(map[string]string{"HOME": "/root"})
		taskContextEnvMap := variables.FromMap(map[string]string{"FOO": "foo", "BAZ": "baz"})

		tsk := task.NewTask("tut")
		tsk.Commands = []string{"echo foo"}
		tsk.Env.Set("BAR", "task-var-wins")
		tsk.Required.Env = []string{"FOO", "BAR", "BAZ"}

		_, err := tc.CompileTask(tsk,
			runner.NewExecutionContext(&shBin, "/tmp", excontextEnvMap, envFile, nil, nil, nil, nil),
			&bytes.Buffer{},
			&bytes.Buffer{},
			&bytes.Buffer{},
			taskContextEnvMap,
			variables.NewVariables(),
		)

		if err != nil {
			t.Fatal(err)
		}

	})

	t.Run("error on required env missing", func(t *testing.T) {

		tc := runner.NewTaskCompiler()

		excontextEnvMap := variables.FromMap(map[string]string{"HOME": "/root"})
		taskContextEnvMap := variables.FromMap(map[string]string{"FOO": "foo"})

		tsk := task.NewTask("tut")
		tsk.Commands = []string{"echo foo"}
		tsk.Env.Set("BAR", "task-var-wins")
		tsk.Required.Env = []string{"FOO", "BAR", "BAZ"}

		_, err := tc.CompileTask(tsk,
			runner.NewExecutionContext(&shBin, "/tmp", excontextEnvMap, envFile, nil, nil, nil, nil),
			&bytes.Buffer{},
			&bytes.Buffer{},
			&bytes.Buffer{},
			taskContextEnvMap,
			variables.NewVariables(),
		)

		if !errors.Is(err, task.ErrRequiredInputMissing) {
			t.Errorf("got %v, wanted %v", err, task.ErrRequiredInputMissing)
		}
	})
}

func TestTaskCompiler_CompileTask(t *testing.T) {
	tc := runner.NewTaskCompiler()
	j, err := tc.CompileTask(&task.Task{
		Commands:  []string{"echo 1"},
		Variables: variables.FromMap(map[string]string{"TestInterpolatedVar": "TestVar={{.TestVar}}"}),
	},
		runner.NewExecutionContext(&shBin, "/tmp", variables.FromMap(map[string]string{"HOME": "/root"}), envFile, nil, nil, nil, nil),
		&bytes.Buffer{},
		&bytes.Buffer{},
		&bytes.Buffer{},
		variables.NewVariables(),
		variables.FromMap(map[string]string{"TestVar": "TestVarValue"}),
	)
	if err != nil {
		t.Fatal(err)
	}

	if j.Vars.Get("TestInterpolatedVar").(string) != "TestVar=TestVarValue" {
		t.Error("var interpolation failed")
	}
}
