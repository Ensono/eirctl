package runner_test

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/Ensono/eirctl/output"
	"github.com/Ensono/eirctl/runner"
)

func TestDefaultExecutor_Execute(t *testing.T) {
	t.Parallel()
	b1 := &bytes.Buffer{}
	output := output.NewSafeWriter(b1)

	job1 := runner.NewJobFromCommand("echo 'success'")
	to := 1 * time.Minute
	job1.Timeout = &to
	job1.Stdout = output

	e, err := runner.GetExecutorFactory(&runner.ExecutionContext{}, job1)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := e.Execute(context.Background(), job1); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(output.String(), "success") {
		t.Error()
	}

	job1 = runner.NewJobFromCommand("exit 1")
	job1.Stdout = io.Discard
	job1.Stderr = io.Discard

	_, err = e.Execute(context.Background(), job1)
	if err == nil {
		t.Error()
	}

	if _, ok := runner.IsExitStatus(err); !ok {
		t.Error()
	}

	job2 := runner.NewJobFromCommand("echo {{ .Fail }}")
	_, err = e.Execute(context.Background(), job2)
	if err == nil {
		t.Error()
	}

	job3 := runner.NewJobFromCommand("printf '%s\\nLine-2\\n' '=========== Line 1 ==================' ")
	_, err = e.Execute(context.Background(), job3)
	if err != nil {
		t.Error()
	}
}
