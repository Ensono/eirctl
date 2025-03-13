package runner_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Ensono/eirctl/internal/utils"
	"github.com/Ensono/eirctl/output"
	"github.com/Ensono/eirctl/runner"
	"github.com/Ensono/eirctl/variables"
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

	// b2 := &bytes.Buffer{}
	// o2 := output.NewSafeWriter(b2)

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

func Test_ContainerExecutor(t *testing.T) {
	t.Parallel()
	t.Run("check client does not start with DOCKER_HOST removed", func(t *testing.T) {

	})

	t.Run("docker with alpine:latest", func(t *testing.T) {
		cc := runner.NewContainerContext("alpine:3")
		cc.ShellArgs = []string{"sh", "-c"}

		execContext := runner.NewExecutionContext(&utils.Binary{}, "", variables.NewVariables(), &utils.Envfile{},
			[]string{}, []string{}, []string{}, []string{}, runner.WithContainerOpts(cc))

		if dh := os.Getenv("DOCKER_HOST"); dh == "" {
			t.Fatal("ensure your DOCKER_HOST is set correctly")
		}

		ce, err := runner.GetExecutorFactory(execContext, nil)
		if err != nil {
			t.Error(err)
		}

		so := &bytes.Buffer{}
		se := &bytes.Buffer{}
		_, err = ce.Execute(context.TODO(), &runner.Job{Command: `pwd
for i in $(seq 1 10); do echo "hello, iteration $i"; done`,
			Env:    variables.NewVariables(),
			Vars:   variables.NewVariables(),
			Stdout: output.NewSafeWriter(so),
			Stderr: output.NewSafeWriter(se),
		})

		if err != nil {
			t.Fatal(err)
		}

		if len(se.Bytes()) > 0 {
			t.Errorf("got error %v, expected nil\n\n", se.String())
		}
		if len(so.Bytes()) == 0 {
			t.Errorf("got (%s) no output, expected stdout\n\n", se.String())
		}
		want := `/eirctl
hello, iteration 1
hello, iteration 2
hello, iteration 3
hello, iteration 4
hello, iteration 5
hello, iteration 6
hello, iteration 7
hello, iteration 8
hello, iteration 9
hello, iteration 10
`
		if so.String() != want {
			t.Errorf("outputs do not match\n\tgot: %s\n\twanted:  %s", so.String(), want)
		}
	})

	t.Run("correctly mounts host dir", func(t *testing.T) {
		cc := runner.NewContainerContext("alpine:3")
		cc.ShellArgs = []string{"sh", "-c"}
		pwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}

		cc.WithVolumes(fmt.Sprintf("%s:/eirctl", pwd))
		execContext := runner.NewExecutionContext(&utils.Binary{}, "", variables.NewVariables(), &utils.Envfile{},
			[]string{}, []string{}, []string{}, []string{}, runner.WithContainerOpts(cc))

		if dh := os.Getenv("DOCKER_HOST"); dh == "" {
			t.Fatal("ensure your DOCKER_HOST is set correctly")
		}

		ce, err := runner.GetExecutorFactory(execContext, nil)
		if err != nil {
			t.Error(err)
		}

		so, se := output.NewSafeWriter(&bytes.Buffer{}), output.NewSafeWriter(&bytes.Buffer{})
		_, err = ce.Execute(context.TODO(), &runner.Job{Command: `ls -l .`,
			Env:    variables.NewVariables(),
			Vars:   variables.NewVariables(),
			Stdout: so,
			Stderr: se,
		})

		if err != nil {
			t.Fatalf("got %v, wanted nil", err)
		}
		fmt.Println(so.String())
		if !strings.Contains(so.String(), `compiler.go`) {
			t.Errorf("got (%v), expected error\n\n", so.String())
		}
	})

	t.Run("error docker with alpine:latest", func(t *testing.T) {
		cc := runner.NewContainerContext("alpine:3")
		cc.ShellArgs = []string{"sh", "-c"}

		execContext := runner.NewExecutionContext(&utils.Binary{}, "", variables.NewVariables(), &utils.Envfile{},
			[]string{}, []string{}, []string{}, []string{}, runner.WithContainerOpts(cc))

		if dh := os.Getenv("DOCKER_HOST"); dh == "" {
			t.Fatal("ensure your DOCKER_HOST is set correctly")
		}

		ce, err := runner.GetExecutorFactory(execContext, nil)
		if err != nil {
			t.Error(err)
		}

		so, se := output.NewSafeWriter(&bytes.Buffer{}), output.NewSafeWriter(&bytes.Buffer{})
		_, err = ce.Execute(context.TODO(), &runner.Job{Command: `unknown --version`,
			Env:    variables.NewVariables(),
			Vars:   variables.NewVariables(),
			Stdout: so,
			Stderr: se,
		})

		if err == nil {
			t.Fatalf("got %v, wanted error", err)
		}

		if len(se.String()) == 0 {
			t.Errorf("got error (%v), expected error\n\n", se.String())
		}

		if len(so.String()) > 0 {
			t.Errorf("got (%s) no output, expected stdout\n\n", se.String())
		}
	})
}
