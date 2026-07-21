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
	t.Run("main sanity check", func(t *testing.T) {
		os.Args = []string{"eirctl", "run", "unknown"}

		eirctlRootCmd := eirctlcmd.NewEirCtlCmd(context.TODO(), os.Stdout, os.Stderr)

		if err := eirctlRootCmd.InitCommand(eirctlcmd.WithSubCommands()...); err != nil {
			logrus.Fatal(err)
		}

		setDefaultCommandIfNonePresent(eirctlRootCmd.Cmd)

		if err := eirctlRootCmd.Execute(); err == nil {
			t.Error("got nil wanted error")
		}
	})

	t.Run("main sanity check (explicit debug)", func(t *testing.T) {
		os.Args = []string{"eirctl", "run", "unknown", "--debug"}

		eirctlRootCmd := eirctlcmd.NewEirCtlCmd(context.TODO(), os.Stdout, os.Stderr)

		if err := eirctlRootCmd.InitCommand(eirctlcmd.WithSubCommands()...); err != nil {
			logrus.Fatal(err)
		}

		setDefaultCommandIfNonePresent(eirctlRootCmd.Cmd)

		if err := eirctlRootCmd.Execute(); err == nil {
			t.Error("got nil wanted error")
		}

		logLevel := logrus.GetLevel()
		if logLevel != logrus.DebugLevel {
			t.Errorf("Expected Log Level to be '%s', got: '%s'", logrus.DebugLevel, logLevel)
		}
	})

	t.Run("main sanity check (explicit verbose)", func(t *testing.T) {
		os.Args = []string{"eirctl", "run", "unknown", "--verbose"}

		eirctlRootCmd := eirctlcmd.NewEirCtlCmd(context.TODO(), os.Stdout, os.Stderr)

		if err := eirctlRootCmd.InitCommand(eirctlcmd.WithSubCommands()...); err != nil {
			logrus.Fatal(err)
		}

		setDefaultCommandIfNonePresent(eirctlRootCmd.Cmd)

		if err := eirctlRootCmd.Execute(); err == nil {
			t.Error("got nil wanted error")
		}

		logLevel := logrus.GetLevel()
		if logLevel != logrus.TraceLevel {
			t.Errorf("Expected Log Level to be '%s', got: '%s'", logrus.TraceLevel, logLevel)
		}
	})
	t.Run("exit code correctly bubbled up", func(t *testing.T) {
		os.Args = []string{"eirctl", "run", "task", "fail_125", "-c", "testdata/eirctl.yaml"}
		moutW := &bytes.Buffer{}
		merrW := &bytes.Buffer{}
		ec := runMain(moutW, merrW)

		if ec != 125 {
			t.Fatalf("process ran wihout error")
		}

		if len(moutW.String()) < 1 {
			t.Errorf("got empty error, expected a message")
		}
	})
	t.Run("exited at eirctl command not found", func(t *testing.T) {
		os.Args = []string{"eirctl", "run", "task", "not-found", "-c", "testdata/eirctl.yaml"}
		moutW := &bytes.Buffer{}
		merrW := &bytes.Buffer{}
		ec := runMain(moutW, merrW)

		if ec != 1 {
			t.Fatalf("process ran wihout error")
		}

		if len(moutW.String()) < 1 {
			t.Errorf("got empty error, expected a message")
		}
	})
	t.Run("exited at eirctl with help", func(t *testing.T) {
		os.Args = []string{"eirctl", "--help"}
		moutW := &bytes.Buffer{}
		merrW := &bytes.Buffer{}
		ec := runMain(moutW, merrW)

		if ec != 0 {
			t.Fatalf("process ran wih error")
		}

		if len(moutW.String()) < 1 {
			t.Errorf("got empty error, expected a message")
		}
	})
}
