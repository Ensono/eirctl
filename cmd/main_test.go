package main

import (
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
}
