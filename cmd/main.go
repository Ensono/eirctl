package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"slices"
	"syscall"

	eirctlcmd "github.com/Ensono/eirctl/cmd/eirctl"
	"github.com/Ensono/eirctl/internal/cmdutils"
	"github.com/Ensono/eirctl/runner"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func subCommands(cmd *cobra.Command) (commandNames []string) {
	for _, command := range cmd.Commands() {
		commandNames = append(commandNames, append(command.Aliases, command.Name())...)
	}
	commandNames = append(commandNames, "completion")
	return
}

// setDefaultCommandIfNonePresent This is only here for backwards compatibility
//
// If any user names a runnable task or pipeline the same as
// an existing command command will always take precedence ;)
// And will most likely fail as the argument into the command was perceived as a command name
func setDefaultCommandIfNonePresent(cmd *cobra.Command) {
	// to maintain the existing behaviour of
	// displaying a pipeline/task selector
	if len(os.Args) == 1 {
		os.Args = []string{os.Args[0], "run"}
	}

	if len(os.Args) == 2 {
		if slices.Contains([]string{"-h", "--help", "-v", "--version"}, os.Args[1]) {
			// we want the root command to display all options
			// another hack around default command
			return
		}
	}

	if len(os.Args) > 1 {
		// This will turn `eirctl [pipeline task]` => `eirctl run [pipeline task]`
		potentialCommand := os.Args[1]
		if slices.Contains(subCommands(cmd), potentialCommand) {
			return
		}
		os.Args = append([]string{os.Args[0], "run"}, os.Args[1:]...)
	}
}

func runMain(stdoutW io.Writer, errW io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), []os.Signal{os.Interrupt, syscall.SIGTERM, os.Kill}...)
	defer stop()

	eirctlRootCmd := eirctlcmd.NewEirCtlCmd(ctx, stdoutW, errW)

	if err := eirctlRootCmd.InitCommand(eirctlcmd.WithSubCommands()...); err != nil {
		logrus.Fatal(err)
	}

	setDefaultCommandIfNonePresent(eirctlRootCmd.Cmd)

	if err := eirctlRootCmd.Execute(); err != nil {
		logrus.Debugf("main: err type=%T value=%v", err, err)
		fmt.Fprintf(stdoutW, cmdutils.RED_TERMINAL+"\n", err)
		if code, ok := runner.IsExitStatus(err); ok {
			logrus.Debugf("main: exit code=%d", code)
			// propagate the container's actual exit code (e.g. 137 for SIGKILL, 143 for SIGTERM)
			return int(code)
		}
		return 1
	}
	return 0
}

func main() {
	os.Exit(runMain(os.Stdout, os.Stderr))
}
