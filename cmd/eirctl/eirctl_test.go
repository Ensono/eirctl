package cmd_test

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	eirctlCmd "github.com/Ensono/eirctl/cmd/eirctl"
	"github.com/Ensono/eirctl/output"
)

type cmdRunTestInput struct {
	args        []string
	errored     bool
	exactOutput string
	output      []string
	ctx         context.Context
}

func cmdRunTestHelper(t *testing.T, testInput *cmdRunTestInput) {
	t.Helper()
	ctx := context.TODO()

	if testInput.ctx != nil {
		ctx = testInput.ctx
	}

	logOut := output.NewSafeWriter(&bytes.Buffer{})
	logErr := output.NewSafeWriter(&bytes.Buffer{})

	cmd := eirctlCmd.NewEirCtlCmd(ctx, logOut, logErr)
	os.Args = append([]string{os.Args[0]}, testInput.args...)

	cmd.Cmd.SetArgs(testInput.args)
	errOut := output.NewSafeWriter(&bytes.Buffer{})
	stdOut := output.NewSafeWriter(&bytes.Buffer{})
	cmd.Cmd.SetErr(errOut)
	cmd.Cmd.SetOut(stdOut)

	if err := cmd.InitCommand(eirctlCmd.WithSubCommands()...); err != nil {
		t.Fatal(err)
	}

	if err := cmd.Execute(); err != nil {
		if testInput.errored {
			if len(testInput.output) > 0 {
				for _, v := range testInput.output {
					if !(strings.Contains(err.Error(), v)) {
						t.Errorf("\nerror: %s\n\ndoes not contain: %v\n", err.Error(), v)
					}
				}
			}
			return
		}
		t.Fatalf("\ngot: %v\nwanted <nil>\n", err)
	}

	if testInput.errored && errOut.Len() < 1 {
		t.Errorf("\ngot: nil\nwanted an error to be thrown")
	}
	if len(testInput.output) > 0 {
		for _, v := range testInput.output {
			if !strings.Contains(logOut.String(), v) {
				t.Errorf("\ngot: %s\vnot found in: %v", logOut.String(), v)
			}
		}
	}
	if testInput.exactOutput != "" && logOut.String() != testInput.exactOutput {
		t.Errorf("output mismatch\ngot: %s\n\nwanted: %s", logOut.String(), testInput.exactOutput)
	}
}
