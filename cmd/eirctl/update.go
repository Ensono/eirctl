package cmd

import "github.com/spf13/cobra"

func newUpdateCommand(rootCmd *EirCtlCmd) {
	updateCmd := &cobra.Command{
		Use:     "update",
		Aliases: []string{},
		Short:   `Updates the to the specified or latest version of EirCTL.`,
		Args:    cobra.RangeArgs(1, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			conf, err := rootCmd.initConfig()
			if err != nil {
				return err
			}
			return nil
		},
	}
}

