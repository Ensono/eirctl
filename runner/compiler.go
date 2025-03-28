package runner

import (
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"

	"github.com/Ensono/eirctl/internal/utils"
	"github.com/Ensono/eirctl/task"
	"github.com/Ensono/eirctl/variables"
	"github.com/sirupsen/logrus"
)

// TaskCompiler compiles tasks into jobs for executor
type TaskCompiler struct {
	variables *variables.Variables
}

// NewTaskCompiler create new TaskCompiler instance
func NewTaskCompiler() *TaskCompiler {
	return &TaskCompiler{variables: variables.NewVariables()}
}

// CompileTask compiles task into Job (linked list of commands) executed by Executor
func (tc *TaskCompiler) CompileTask(t *task.Task, executionContext *ExecutionContext, stdin io.Reader, stdout, stderr io.Writer, env, vars *variables.Variables) (*Job, error) {
	vars = t.Variables.Merge(vars)

	var job, prev *Job

	for k, v := range vars.Map() {
		if reflect.ValueOf(v).Kind() != reflect.String {
			continue
		}

		renderedStr, err := utils.RenderString(v.(string), vars.Map())
		if err != nil {
			return nil, err
		}
		vars.Set(k, renderedStr)
	}

	// creating multiple versions of the same task with different env input
	for _, variant := range t.GetVariations() {
		// each command in the array needs compiling
		for _, command := range t.Commands {
			j, err := tc.CompileCommand(
				t.Name,
				command,
				executionContext,
				t.Dir,
				t.Timeout,
				stdin,
				stdout,
				stderr,
				env.Merge(variables.FromMap(variant)),
				vars,
			)
			if err != nil {
				return nil, err
			}

			if job == nil {
				job = j
			}

			if prev == nil {
				prev = j
			} else {
				prev.Next = j
				prev = prev.Next
			}
		}
	}
	if t.Interactive {
		job.IsShell = true
	}

	return job, nil
}

// CompileCommand compiles command into Job
func (tc *TaskCompiler) CompileCommand(
	taskName string,
	command string,
	executionCtx *ExecutionContext,
	dir string,
	timeout *time.Duration,
	stdin io.Reader,
	stdout, stderr io.Writer,
	env, vars *variables.Variables,
) (*Job, error) {
	j := &Job{
		Timeout: timeout,
		Env:     env,
		Stdin:   stdin,
		Stdout:  stdout,
		Stderr:  stderr,
		Vars:    tc.variables.Merge(vars),
	}

	// Look at the executable details and check if the command is running `docker` determine if an Envfile is being generated
	// If it has then check to see if the args contains the --env-file flag and if it does modify the path to the envfile
	// if it does not then add the --env-file flag to the args array
	if executionCtx.Envfile != nil { // && executionCtx.Executable.IsContainer
		// generate the envfile with supplied env only
		err := executionCtx.ProcessEnvfile(env)
		if err != nil {
			return nil, err
		}
	}

	c := []string{command}
	if executionCtx.Executable != nil {
		c = []string{executionCtx.Executable.Bin}
		c = append(c, executionCtx.Executable.Args...)
		c = append(c, fmt.Sprintf("%s%s%s", executionCtx.Quote, command, executionCtx.Quote))
	}

	j.Command = strings.Join(c, " ")
	j.Env = executionCtx.Env
	logrus.Debugf("command: %s", j.Command)

	var err error
	if dir != "" {
		j.Dir = dir
	} else if executionCtx.Dir != "" {
		j.Dir = executionCtx.Dir
	}

	j.Dir, err = utils.RenderString(j.Dir, j.Vars.Map())
	if err != nil {
		return nil, err
	}

	return j, nil
}
