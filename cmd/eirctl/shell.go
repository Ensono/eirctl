package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/Ensono/eirctl/runner"
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

			// taskRunner, argsStringer, err := rootCmd.buildTaskRunner(args, conf)
			// if err != nil {
			// 	return err
			// }
			configContext, ok := conf.Contexts[args[0]]
			if !ok {
				return fmt.Errorf("%s. %w", args[0], ErrIncorrectPipelineTaskArg)
			}
			// logrus.Debug(argsStringer.argsList)
			// args[0]
			// taskRunner.Stdin
			// runner.NewContainerContext(cc.Container().Image)

			execContext := runner.NewExecutionContext(nil, configContext.Dir, configContext.Env, configContext.Envfile,
				[]string{}, []string{}, []string{}, []string{},
				runner.WithContainerOpts(configContext.Container()))
			ce, err := runner.NewContainerExecutor(execContext)
			if err != nil {
				return err
			}
			_, err = ce.Execute(rootCmd.ctx, &runner.Job{
				Stdin:   os.Stdin,
				Stdout:  rootCmd.ChannelOut,
				Stderr:  rootCmd.ChannelErr,
				Dir:     configContext.Dir,
				IsShell: true,
			})

			if err != nil {
				return err
			}

			return nil
		},
	}
	rootCmd.Cmd.AddCommand(showCmd)
}
