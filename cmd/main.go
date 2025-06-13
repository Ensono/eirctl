package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"slices"
	"syscall"

	eirctlcmd "github.com/Ensono/eirctl/cmd/eirctl"
	"github.com/Ensono/eirctl/internal/cmdutils"
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

func cmdSetUp() (*eirctlcmd.EirCtlCmd, context.CancelFunc) {
	ctx, stop := signal.NotifyContext(context.Background(), []os.Signal{os.Interrupt, syscall.SIGTERM, os.Kill}...)

	eirctlRootCmd := eirctlcmd.NewEirCtlCmd(ctx, os.Stdout, os.Stderr)

	if err := eirctlRootCmd.InitCommand(eirctlcmd.WithSubCommands()...); err != nil {
		logrus.Fatal(err)
	}

	setDefaultCommandIfNonePresent(eirctlRootCmd.Cmd)

	return eirctlRootCmd, stop
}

func main() {
	eirctlRootCmd, stop := cmdSetUp()
	defer stop()
	if err := eirctlRootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stdout, cmdutils.RED_TERMINAL+"\n", err)
		os.Exit(1)
	}
}
