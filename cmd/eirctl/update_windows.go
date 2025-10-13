//go:build windows
// +build windows

package cmd

import (
	"fmt"
	"os"
	"path"
)

func PrepSourceBinary(rootCmd *EirCtlCmd, currentExecPath string) {
	// move current file to tmp
	if err := rootCmd.OsFsOps.Rename(currentExecPath, path.Join(path.Dir(currentExecPath), "eirctl.old")); err != nil {
		fmt.Fprintf(os.Stdout, "rename err: %v", err)
	}
}

func EnrichFinalLink(link string) string {
	return link + ".exe"
}
