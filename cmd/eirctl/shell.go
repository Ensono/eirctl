package cmd

import (
	"errors"
	"fmt"

	"github.com/Ensono/eirctl/task"
	"github.com/Ensono/eirctl/variables"
	"github.com/spf13/cobra"
)

var ErrIncorrectContextName = errors.New("supplied argument does not match any container context")
var ErrNotV2Context = errors.New("not a native container context")

func newShellCmd(rootCmd *EirCtlCmd) {

	showCmd := &cobra.Command{
		Use:     "shell",
		Aliases: []string{},
		Short:   `shell into the supplied container context`,
		Args:    cobra.RangeArgs(1, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conf, err := rootCmd.initConfig()
			if err != nil {
				return err
			}
			contextName := args[0]
			configCtx, ok := conf.Contexts[contextName]
			if !ok {
				return fmt.Errorf("%w, %s", ErrIncorrectContextName, contextName)
			}

			if configCtx.Container() == nil {
				return fmt.Errorf("%w, %s", ErrNotV2Context, contextName)
			}

			tr, err := rootCmd.initTaskRunner(conf, &variables.Variables{})
			if err != nil {
				return err
			}

			// create an interactive task
			it := task.NewTask("interactive")
			it.Interactive = true
			it.Context = contextName
			it.Variables = &variables.Variables{}
			it.Commands = []string{""}
			it.Env = &variables.Variables{}
			it.EnvFile = nil

			return tr.Run(it)
		},
	}
	rootCmd.Cmd.AddCommand(showCmd)
}
