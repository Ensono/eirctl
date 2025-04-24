package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newValidateCmd(rootCmd *EirCtlCmd) {
	c := &cobra.Command{
		Use:   "validate",
		Short: `validates config file`,
		Args:  cobra.MinimumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := rootCmd.initConfig()
			if err != nil {
				return err
			}
			fmt.Fprintf(rootCmd.ChannelOut, "file (%s) is valid\n", cfg.SourceFile)
			return nil
		},
		PostRunE: func(cmd *cobra.Command, args []string) error {
			return nil // postRunReset()
		},
	}
	rootCmd.Cmd.AddCommand(c)
}
