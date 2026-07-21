package runner

import (
	"context"
	"errors"
	"fmt"

	"mvdan.cc/sh/v3/interp"
)

type ExecutorIface interface {
	// WithEnv(env []string) ExecutorIface
	WithReset(doReset bool)
	Execute(ctx context.Context, job *Job) ([]byte, error)
}

// GetExecutorFactory returns a factory instance of the executor
func GetExecutorFactory(execContext *ExecutionContext, job *Job) (ExecutorIface, error) {
	switch execContext.GetExecutorType() {
	case DefaultExecutorTyp:
		return newDefaultExecutor(job.Stdin, job.Stdout, job.Stderr)
	case ContainerExecutorTyp:
		return NewContainerExecutor(execContext)
	default:
		return nil, fmt.Errorf("unknown executor type")
	}
}

// IsExitStatus checks if given `err` is an exit status
func IsExitStatus(err error) (uint8, bool) {
	var es interp.ExitStatus
	if errors.As(err, &es) {
		return uint8(es), true
	}
	return 0, false
}
