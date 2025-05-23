package runner

import (
	"context"
	"io"
	"os"
	"strings"

	"github.com/Ensono/eirctl/internal/utils"
	"github.com/sirupsen/logrus"
	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

// DefaultExecutor is a default executor used for jobs
// Uses `mvdan.cc/sh/v3/interp` under the hood
type DefaultExecutor struct {
	dir    string
	env    []string
	interp *interp.Runner
	// doReset resets the execution environment after each run
	doReset bool
}

// newDefaultExecutor creates new default executor
func newDefaultExecutor(stdin io.Reader, stdout, stderr io.Writer) (*DefaultExecutor, error) {
	var err error
	e := &DefaultExecutor{
		env: os.Environ(), // do not want to set the environment here
	}

	e.dir, err = os.Getwd()
	if err != nil {
		return nil, err
	}

	e.interp, err = interp.New(
		interp.StdIO(stdin, stdout, stderr),
	)
	if err != nil {
		return nil, err
	}

	return e, nil
}

// withEnv is used to set more specifically the environment vars inside the executor
// func (e *DefaultExecutor) withEnv(env []string) *DefaultExecutor {
// 	e.env = env
// 	return e
// }

func (e *DefaultExecutor) WithReset(doReset bool) {
	e.doReset = doReset
}

// Execute executes given job with provided context
// Returns job output
func (e *DefaultExecutor) Execute(ctx context.Context, job *Job) ([]byte, error) {
	command, err := utils.RenderString(job.Command, job.Vars.Map())
	if err != nil {
		return nil, err
	}

	cmd, err := syntax.NewParser(syntax.KeepComments(true)).Parse(strings.NewReader(command), "")
	if err != nil {
		return nil, err
	}

	env := e.env
	env = append(env, utils.ConvertEnv(utils.ConvertToMapOfStrings(job.Env.Map()))...)

	if job.Dir == "" {
		job.Dir = e.dir
	}

	logrus.Debugf("Executing Command: \"%s\"", command)

	e.interp.Dir = job.Dir
	e.interp.Env = expand.ListEnviron(env...)

	var timeoutCancelFn context.CancelFunc
	if job.Timeout != nil {
		ctx, timeoutCancelFn = context.WithTimeout(ctx, *job.Timeout)
	}

	defer func() {
		if timeoutCancelFn != nil {
			timeoutCancelFn()
		}
	}()

	// Reset needs to be called before Run
	// even the first time around else the vars won't be cleared correctly
	// and re-injected by the mvdan shell
	if e.doReset {
		e.interp.Reset()
	}

	if err := e.interp.Run(ctx, cmd); err != nil {
		return []byte{}, err
	}
	return []byte{}, nil
}
