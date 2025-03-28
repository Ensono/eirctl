package cmd

import (
	"errors"
	"fmt"

	"github.com/Ensono/eirctl/runner"
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

			configContext, ok := conf.Contexts[args[0]]
			if !ok {
				return fmt.Errorf("%s. %w", args[0], ErrIncorrectPipelineTaskArg)
			}

			tr, err := rootCmd.initTaskRunner(conf, &variables.Variables{})
			if err != nil {
				return err
			}

			execContext := runner.NewExecutionContext(nil, configContext.Dir, configContext.Env, configContext.Envfile,
				[]string{}, []string{}, []string{}, []string{},
				runner.WithContainerOpts(configContext.Container()))
			ce, err := runner.NewContainerExecutor(execContext)
			if err != nil {
				return err
			}
			
			if _, err := ce.Execute(rootCmd.ctx, &runner.Job{
				Stdin:   tr.Stdin,
				Stdout:  tr.Stdout,
				Stderr:  tr.Stderr,
				Dir:     configContext.Dir,
				IsShell: true,
			}); err != nil {
				return err
			}
			return nil
		},
	}
	rootCmd.Cmd.AddCommand(showCmd)
}
