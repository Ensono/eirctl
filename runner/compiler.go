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
	if err := renderTaskVariables(vars, t); err != nil {
		return nil, err
	}

	var job, prev *Job

	// creating multiple versions of the same task with different env input
	for _, variant := range t.GetVariations() {
		// each command in the array needs compiling
		for _, command := range t.Commands {
			j, err := tc.compileCommand(compileCommandInput{
				task:         t,
				command:      command,
				executionCtx: executionContext,
				dir:          t.Dir,
				timeout:      t.Timeout,
				stdin:        stdin,
				stdout:       stdout,
				stderr:       stderr,
				env:          env.Merge(variables.FromMap(variant)),
				vars:         vars,
			})
			if err != nil {
				return nil, err
			}

			job, prev = appendJob(job, prev, j)
		}
	}
	if t.Interactive {
		job.IsShell = true
	}

	return job, nil
}

func renderTaskVariables(vars *variables.Variables, t *task.Task) error {
	for key, value := range vars.Map() {
		if reflect.ValueOf(value).Kind() != reflect.String {
			continue
		}

		rendered, err := utils.ParseTemplate(value.(string), vars.Map(), t.Env.Map())
		if err != nil {
			return err
		}
		vars.Set(key, rendered)
	}
	return nil
}

func appendJob(head, tail, job *Job) (*Job, *Job) {
	if head == nil {
		return job, job
	}
	tail.Next = job
	return head, job
}

type compileCommandInput struct {
	task         *task.Task
	command      string
	executionCtx *ExecutionContext
	dir          string
	timeout      *time.Duration
	stdin        io.Reader
	stdout       io.Writer
	stderr       io.Writer
	env          *variables.Variables
	vars         *variables.Variables
}

// CompileCommand compiles command into Job
func (tc *TaskCompiler) compileCommand(input compileCommandInput) (*Job, error) {
	j := &Job{
		Timeout: input.timeout,
		Env:     input.env,
		Stdin:   input.stdin,
		Stdout:  input.stdout,
		Stderr:  input.stderr,
		Vars:    tc.variables.Merge(input.vars),
	}

	// Look at the executable details and check if the command is running `docker` determine if an Envfile is being generated
	// If it has then check to see if the args contains the --env-file flag and if it does modify the path to the envfile
	// if it does not then add the --env-file flag to the args array
	if input.executionCtx.Envfile != nil { // && executionCtx.Executable.IsContainer
		// generate the envfile with supplied env only
		err := input.executionCtx.ProcessEnvfile(input.env)
		if err != nil {
			return nil, err
		}
	}

	c := []string{input.command}
	if input.executionCtx.Executable != nil {
		c = []string{input.executionCtx.Executable.Bin}
		c = append(c, input.executionCtx.Executable.Args...)
		c = append(c, fmt.Sprintf("%s%s%s", input.executionCtx.Quote, input.command, input.executionCtx.Quote))
	}

	j.Command = strings.Join(c, " ")
	j.Env = input.executionCtx.Env
	logrus.Debugf("command: %s", j.Command)

	var err error
	if input.dir != "" {
		j.Dir = input.dir
	} else if input.executionCtx.Dir != "" {
		j.Dir = input.executionCtx.Dir
	}

	j.Dir, err = utils.ParseTemplate(j.Dir, j.Vars.Map(), j.Env.Map())
	if err != nil {
		return nil, err
	}

	// runtime check to see if task can proceed based on inputs
	//
	// NOTE: This could also be a compile time check - but it would preclude any dynamically set environment
	// variables to be checked
	if input.task.Required != nil && input.task.Required.HasRequired() {
		if err := input.task.Required.Check(j.Env.Merge(input.task.Env), j.Vars); err != nil {
			return nil, err
		}
	}

	return j, nil
}
