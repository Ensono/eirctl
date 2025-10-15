//go:build !windows
// +build !windows

package cmd

func PrepSourceBinary(rootCmd *EirCtlCmd, currentExecPath string) {
	// noop
}

func EnrichFinalLink(link string) string {
	return link
}
