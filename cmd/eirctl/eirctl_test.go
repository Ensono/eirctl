package cmd_test

import (
	"bytes"
	"context"
	"io"
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

type mockOsFsOps struct {
	rename func(oldpath string, newpath string) error
	write  func(name string, data []byte, perm os.FileMode) error
}

func (o mockOsFsOps) Rename(oldpath string, newpath string) error {
	return nil
}

func (o mockOsFsOps) WriteFile(name string, data []byte, perm os.FileMode) error {
	return nil
}

func (o mockOsFsOps) Create(name string) (io.Writer, error) {
	return output.NewSafeWriter(&bytes.Buffer{}), nil
}

func cmdRunTestHelper(t *testing.T, testInput *cmdRunTestInput) {
	t.Helper()
	err, logOutput, errOutputLength := executeCommandTest(t, testInput)
	if err != nil {
		assertCommandError(t, testInput, err)
		return
	}

	assertCommandOutput(t, testInput, logOutput, errOutputLength)
}

func executeCommandTest(t *testing.T, testInput *cmdRunTestInput) (error, string, int) {
	t.Helper()
	ctx := testInput.ctx
	if ctx == nil {
		ctx = context.TODO()
	}

	logOut := output.NewSafeWriter(&bytes.Buffer{})
	logErr := output.NewSafeWriter(&bytes.Buffer{})
	cmd := eirctlCmd.NewEirCtlCmd(ctx, logOut, logErr)
	os.Args = append([]string{os.Args[0]}, testInput.args...)

	cmd.Cmd.SetArgs(testInput.args)
	errOut := output.NewSafeWriter(&bytes.Buffer{})
	cmd.Cmd.SetErr(errOut)
	cmd.Cmd.SetOut(output.NewSafeWriter(&bytes.Buffer{}))
	cmd.OsFsOps = mockOsFsOps{}
	if err := cmd.InitCommand(eirctlCmd.WithSubCommands()...); err != nil {
		t.Fatal(err)
	}

	return cmd.Execute(), logOut.String(), errOut.Len()
}

func assertCommandError(t *testing.T, testInput *cmdRunTestInput, err error) {
	t.Helper()
	if !testInput.errored {
		t.Fatalf("\ngot: %v\nwanted <nil>\n", err)
	}
	for _, expectedOutput := range testInput.output {
		if !strings.Contains(err.Error(), expectedOutput) {
			t.Errorf("\nerror: %s\n\ndoes not contain: %v\n", err.Error(), expectedOutput)
		}
	}
}

func assertCommandOutput(t *testing.T, testInput *cmdRunTestInput, logOutput string, errOutputLength int) {
	t.Helper()
	if testInput.errored && errOutputLength < 1 {
		t.Errorf("\ngot: nil\nwanted an error to be thrown")
	}
	for _, expectedOutput := range testInput.output {
		if !strings.Contains(logOutput, expectedOutput) {
			t.Errorf("\ngot: %s\vnot found in: %v", logOutput, expectedOutput)
		}
	}
	if testInput.exactOutput != "" && logOutput != testInput.exactOutput {
		t.Errorf("output mismatch\ngot: %s\n\nwanted: %s", logOutput, testInput.exactOutput)
	}
}
