package selfupdate

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"runtime"

	"github.com/Ensono/eirctl/cmdutils"
	"github.com/spf13/cobra"
)

type UpdateCmdFlags struct {
	BaseUrl string
	Version string
}

type UpdateCmd struct {
	flags        UpdateCmdFlags
	name         string
	suffix       string
	ghReleaseUrl string
	OsFsOps      cmdutils.OsFSOpsIface
	getVersionFn func(ctx context.Context, flags UpdateCmdFlags) ([]byte, error)
}

type Opt func(*UpdateCmd)

func New(name string, ghReleaseUrl string, opts ...Opt) *UpdateCmd {
	uc := &UpdateCmd{
		flags:        UpdateCmdFlags{},
		name:         name,
		ghReleaseUrl: ghReleaseUrl,
		OsFsOps:      cmdutils.OsFsOps{},
	}
	uc.getVersionFn = uc.GetVersion

	for _, opt := range opts {
		opt(uc)
	}

	return uc
}

func WithOsFsOps(osfs cmdutils.OsFSOpsIface) Opt {
	return func(uc *UpdateCmd) {
		uc.OsFsOps = osfs
	}
}

func WithGetVersionFunc(fn func(ctx context.Context, flags UpdateCmdFlags) ([]byte, error)) Opt {
	return func(uc *UpdateCmd) {
		uc.getVersionFn = fn
	}
}

func WithDownloadSuffix(suffix string) Opt {
	return func(uc *UpdateCmd) {
		uc.suffix = suffix
	}
}

// AddUpdateCommand
func (uc *UpdateCmd) AddUpdateCommand(rootCmd *cobra.Command) {

	updateCmd := &cobra.Command{
		Use:     "update",
		Aliases: []string{"self-update"},
		Short:   `Updates the binary to the specified or latest version.`,
		Long: `Updates the binary to the specified or latest version.

Supports GitHub releases OOTB, but custom functions for GetVersion can be provided.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			currentExecPath, err := os.Executable()
			if err != nil {
				return err
			}
			binary, err := uc.getVersionFn(cmd.Context(), uc.flags)
			if err != nil {
				return err
			}

			uc.prepSourceBinary(currentExecPath)

			return uc.OsFsOps.WriteFile(currentExecPath, binary, 0666)
		},
	}
	updateCmd.PersistentFlags().StringVarP(&uc.flags.Version, "version", "", "latest", "specific version to update to.")
	updateCmd.PersistentFlags().StringVarP(&uc.flags.BaseUrl, "baseUrl", "", uc.ghReleaseUrl, "base url for the github release repository")
	rootCmd.AddCommand(updateCmd)
}

// GetVersion downloads the binary stream from remote endpoint
//
// NOTE: exposed as public for testing purposes
//
// This can be overwritten completely to support any kind of fetcher
func (uc *UpdateCmd) GetVersion(ctx context.Context, flags UpdateCmdFlags) ([]byte, error) {
	c := &http.Client{}
	suffix := fmt.Sprintf("%s-%s-%s", uc.name, runtime.GOOS, runtime.GOARCH)
	specific := "download/%s"
	latest := "latest/download"

	releasePath := path.Join(fmt.Sprintf(specific, flags.Version), suffix)

	if flags.Version == "latest" {
		releasePath = path.Join(latest, suffix)
	}

	link, err := url.Parse(fmt.Sprintf("%s/%s", flags.BaseUrl, EnrichFinalLink(releasePath)))
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

// prepSourceBinary as a rule of thumb on all platforms
// We move the current binary to a binaryName.old
func (uc *UpdateCmd) prepSourceBinary(currentExecPath string) error {
	oldName := uc.name + ".old"
	if runtime.GOOS == "windows" {
		oldName += ".exe"
	}

	// move current file to old
	return uc.OsFsOps.Rename(currentExecPath, path.Join(path.Dir(currentExecPath), oldName))
}
