package main

import (
	"bytes"
	"context"
	"os"
	"testing"

	eirctlcmd "github.com/Ensono/eirctl/cmd/eirctl"
	"github.com/sirupsen/logrus"
)

func Test_main(t *testing.T) {
	for _, testCase := range []struct {
		name             string
		args             []string
		expectedLogLevel *logrus.Level
	}{
		{name: "main sanity check", args: []string{"eirctl", "run", "unknown"}},
		{name: "main sanity check (explicit debug)", args: []string{"eirctl", "run", "unknown", "--debug"}, expectedLogLevel: logLevel(logrus.DebugLevel)},
		{name: "main sanity check (explicit verbose)", args: []string{"eirctl", "run", "unknown", "--verbose"}, expectedLogLevel: logLevel(logrus.TraceLevel)},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			assertRootCommandFailure(t, testCase.args, testCase.expectedLogLevel)
		})
	}

	for _, testCase := range []struct {
		name         string
		args         []string
		expectedCode int
	}{
		{name: "exit code correctly bubbled up", args: []string{"eirctl", "run", "task", "fail_125", "-c", "testdata/eirctl.yaml"}, expectedCode: 125},
		{name: "exited at eirctl command not found", args: []string{"eirctl", "run", "task", "not-found", "-c", "testdata/eirctl.yaml"}, expectedCode: 1},
		{name: "exited at eirctl with help", args: []string{"eirctl", "--help"}, expectedCode: 0},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			assertMainExit(t, testCase.args, testCase.expectedCode)
		})
	}
}

func logLevel(level logrus.Level) *logrus.Level {
	return &level
}

func assertRootCommandFailure(t *testing.T, args []string, expectedLogLevel *logrus.Level) {
	t.Helper()
	withTestArgs(t, args)
	previousLogLevel := logrus.GetLevel()
	t.Cleanup(func() { logrus.SetLevel(previousLogLevel) })

	eirctlRootCmd := eirctlcmd.NewEirCtlCmd(context.TODO(), os.Stdout, os.Stderr)
	if err := eirctlRootCmd.InitCommand(eirctlcmd.WithSubCommands()...); err != nil {
		t.Fatal(err)
	}

	setDefaultCommandIfNonePresent(eirctlRootCmd.Cmd)
	if err := eirctlRootCmd.Execute(); err == nil {
		t.Error("got nil wanted error")
	}
	if expectedLogLevel != nil && logrus.GetLevel() != *expectedLogLevel {
		t.Errorf("Expected Log Level to be '%s', got: '%s'", *expectedLogLevel, logrus.GetLevel())
	}
}

func assertMainExit(t *testing.T, args []string, expectedCode int) {
	t.Helper()
	withTestArgs(t, args)
	stdout := &bytes.Buffer{}
	if code := runMain(stdout, &bytes.Buffer{}); code != expectedCode {
		t.Fatalf("got exit code %d, wanted %d", code, expectedCode)
	}
	if stdout.Len() < 1 {
		t.Error("got empty error, expected a message")
	}
}

func withTestArgs(t *testing.T, args []string) {
	t.Helper()
	previousArgs := os.Args
	os.Args = args
	t.Cleanup(func() { os.Args = previousArgs })
}
