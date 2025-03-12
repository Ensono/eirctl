package main

import (
	"os"
	"testing"
)

func Test_main(t *testing.T) {
	t.Run("main sanity check", func(t *testing.T) {
		os.Args = []string{"eirctl run unknown"}
		eirctlRootCmd, stop := cmdSetUp()
		defer stop()
		if err := eirctlRootCmd.Execute(); err == nil {
			t.Error("got nil wanted error")
		}
	})
}
