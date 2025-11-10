// Package selfupdate provides the subcommand functionality for
// updating the binary itself.
//
// It supports an OOTB GitHub releases URLs for fetching with some customisation,
// though if too rigid an entire fetch function can be provided.
//
// Example:
//
//	WithGetVersionFunc()
package selfupdate

import (
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
	Create(name string) (io.WriteCloser, error)
	Chmod(name string, mode os.FileMode) error
}

type osFsOps struct {
}

func (o osFsOps) Rename(oldpath string, newpath string) error {
	return os.Rename(oldpath, newpath)
}

func (o osFsOps) Create(name string) (io.WriteCloser, error) {
	return os.Create(name)
}

func (o osFsOps) Executable() (string, error) {
	return os.Executable()
}

func (o osFsOps) Chmod(name string, mode os.FileMode) error {
	return os.Chmod(name, mode)
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

// WithGetVersionFunc accepts a custom function that will encapsulate the entire fetch logic
// w io.WriteCloser will point to the current executable.
//
// Ensure your custom function handles the `w.Write(resp.Body)`
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
		Long:    `Updates the binary to the specified or latest version.`,
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

			f, err := uc.OsFsOps.Create(currentExecPath)
			if err != nil {
				// enrich error here
				return err
			}
			if err := uc.getVersionFn(cmd.Context(), uc.flags, f); err != nil {
				// enrich error here
				return err
			}
			// we need to change mode back to an executable now to ensure it's in the same state as before
			if err := uc.OsFsOps.Chmod(currentExecPath, 0755); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(rootCmd.OutOrStdout(), "%s has been updated\n", uc.name)
			return nil
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
	defer w.Close()

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
		return err
	}

	req := &http.Request{
		URL:    link,
		Method: http.MethodGet,
	}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	bar := progressbar.DefaultBytes(
		resp.ContentLength,
		"downloading",
	)

	if _, err = io.Copy(io.MultiWriter(w, bar), resp.Body); err != nil {
		return err
	}

	return nil
}

// prepSourceBinary as a rule of thumb on all platforms
// We move the current binary to a binaryName.old
func (uc *UpdateCmd) prepSourceBinary(currentExecPath string) error {
	oldName := filepath.Base(currentExecPath) + ".old"

	// move current file to old
	return uc.OsFsOps.Rename(currentExecPath, path.Join(path.Dir(currentExecPath), oldName))
}
