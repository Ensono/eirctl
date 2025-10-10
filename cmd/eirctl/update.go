package cmd

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"runtime"

	"github.com/spf13/cobra"
)

type updateFlags struct {
	baseUrl string
	version string
}

func newUpdateCommand(rootCmd *EirCtlCmd) {
	f := &updateFlags{}

	updateCmd := &cobra.Command{
		Use:     "update",
		Aliases: []string{},
		Short:   `Updates the to the specified or latest version of eirctl.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			currentExecPath, err := os.Executable()
			if err != nil {
				return err
			}
			binary, err := GetVersion(cmd.Context(), f.baseUrl, f.version)
			if err != nil {
				return err
			}
			return os.WriteFile(currentExecPath, binary, 0666)
		},
	}
	updateCmd.PersistentFlags().StringVarP(&f.version, "version", "", "latest", "specific version to update to.")
	updateCmd.PersistentFlags().StringVarP(&f.baseUrl, "baseUrl", "", "https://github.com/Ensono/eirctl/releases", "base url for the release repository")
	rootCmd.Cmd.AddCommand(updateCmd)
}

// GetVersion downloads the binary stream from remote endpoint
// exposed as public for testing purposes
func GetVersion(ctx context.Context, baseUrl, version string) ([]byte, error) {
	c := &http.Client{}
	suffix := fmt.Sprintf("eirctl-%s-%s", runtime.GOOS, runtime.GOARCH)
	specific := "download/%s"
	latest := "latest/download"

	releasePath := path.Join(fmt.Sprintf(specific, version), suffix)

	if version == "latest" {
		releasePath = path.Join(latest, suffix)
	}

	link, err := url.Parse(fmt.Sprintf("%s/%s", baseUrl, releasePath))

	if err != nil {
		return nil, err
	}
	req := &http.Request{
		URL:    link,
		Method: http.MethodGet,
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}
