package cmd

import (
	"errors"
	"fmt"

	"github.com/Ensono/eirctl/task"
	"github.com/Ensono/eirctl/variables"
	"github.com/spf13/cobra"
)

var ErrIncorrectContextName = errors.New("supplied argument does not match any container context")

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
			if _, ok := conf.Contexts[contextName]; !ok {
				return fmt.Errorf("%s, %w", contextName, ErrIncorrectPipelineTaskArg)
			}

			tr, err := rootCmd.initTaskRunner(conf, &variables.Variables{})
			if err != nil {
				return err
			}
			// create a sample task
			interactiveTask := &task.Task{
				Interactive: true,
				Context:     contextName, //
				Variables:   &variables.Variables{},
				Commands:    []string{""},
				Env:         &variables.Variables{},
				EnvFile:     nil,
			}
			return tr.Run(interactiveTask)
		},
	}
	rootCmd.Cmd.AddCommand(showCmd)
}
