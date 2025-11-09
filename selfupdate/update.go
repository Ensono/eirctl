package selfupdate

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"

	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
)

type UpdateCmdFlags struct {
	BaseUrl string
	Version string
}

type updateOsFSOpsIface interface {
	Executable() (string, error)
	Rename(oldpath string, newpath string) error
	WriteFile(name string, data []byte, perm os.FileMode) error
}

type osFsOps struct {
}

func (o osFsOps) Rename(oldpath string, newpath string) error {
	return os.Rename(oldpath, newpath)
}

func (o osFsOps) WriteFile(name string, data []byte, perm os.FileMode) error {
	return os.WriteFile(name, data, perm)
}

func (o osFsOps) Executable() (string, error) {
	return os.Executable()
}

type UpdateCmd struct {
	flags        UpdateCmdFlags
	name         string
	suffix       string
	ghReleaseUrl string
	OsFsOps      updateOsFSOpsIface
	getVersionFn func(ctx context.Context, flags UpdateCmdFlags, w io.WriteCloser) error
}

type Opt func(*UpdateCmd)

func New(name string, ghReleaseUrl string, opts ...Opt) *UpdateCmd {
	uc := &UpdateCmd{
		flags:        UpdateCmdFlags{},
		name:         name,
		ghReleaseUrl: ghReleaseUrl,
		OsFsOps:      osFsOps{},
	}
	uc.getVersionFn = uc.GetVersion

	for _, opt := range opts {
		opt(uc)
	}

	return uc
}

func WithOsFsOps(osfs updateOsFSOpsIface) Opt {
	return func(uc *UpdateCmd) {
		uc.OsFsOps = osfs
	}
}

func WithGetVersionFunc(fn func(ctx context.Context, flags UpdateCmdFlags, w io.WriteCloser) error) Opt {
	return func(uc *UpdateCmd) {
		uc.getVersionFn = fn
	}
}

func WithDownloadSuffix(suffix string) Opt {
	return func(uc *UpdateCmd) {
		uc.suffix = suffix
	}
}

// AddToRootCommand
func (uc *UpdateCmd) AddToRootCommand(rootCmd *cobra.Command) {

	updateCmd := &cobra.Command{
		Use:     "update",
		Aliases: []string{"self-update"},
		Short:   `Updates the binary to the specified or latest version.`,
		Long: `Updates the binary to the specified or latest version.

Supports GitHub releases OOTB, but custom functions for GetVersion can be provided.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			currentExecPath, err := uc.OsFsOps.Executable()
			if err != nil {
				return err
			}

			// perform the rename of the current file prior
			// to writing the new version in to the current executable
			if err := uc.prepSourceBinary(currentExecPath); err != nil {
				return err
			}

			err := uc.getVersionFn(cmd.Context(), uc.flags)
			if err != nil {
				return err
			}
			return nil
			// f, _ := os.Create(currentExecPath)
			// f.Write()
			bb, _ := io.ReadAll(binary)
			return uc.OsFsOps.WriteFile(currentExecPath, bb, 0666)
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
func (uc *UpdateCmd) GetVersion(ctx context.Context, flags UpdateCmdFlags, w io.WriteCloser) error {
	c := &http.Client{}

	// supplying a custom suffix will override the default suffix
	suffix := fmt.Sprintf("%s-%s-%s", uc.name, runtime.GOOS, runtime.GOARCH)
	if uc.suffix != "" {
		suffix = uc.suffix
	}

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

	bar := progressbar.DefaultBytes(
		resp.ContentLength,
		"downloading",
	)
	f := &bytes.Buffer{}

	if _, err = io.Copy(io.MultiWriter(f, bar), resp.Body); err != nil {
		return nil, err
	}

	return f, nil
}

// prepSourceBinary as a rule of thumb on all platforms
// We move the current binary to a binaryName.old
func (uc *UpdateCmd) prepSourceBinary(currentExecPath string) error {
	oldName := filepath.Base(currentExecPath) + ".old"

	// move current file to old
	return uc.OsFsOps.Rename(currentExecPath, path.Join(path.Dir(currentExecPath), oldName))
}
