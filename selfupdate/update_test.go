package selfupdate_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/Ensono/eirctl/output"
	"github.com/Ensono/eirctl/selfupdate"
	"github.com/spf13/cobra"
)

func specificVersionHandler(t *testing.T) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/download/{_...}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write([]byte(`version 0.11.23 downloaded`))
	})
	return mux
}

func latestVersionHandler(t *testing.T) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/latest/download/{_...}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write([]byte(`latest version downloaded`))
	})
	return mux
}

func Test_Update_GetVersion(t *testing.T) {

	t.Run("download specific version", func(t *testing.T) {
		ts := httptest.NewServer(specificVersionHandler(t))
		defer ts.Close()
		su := selfupdate.New("my-binary", ts.URL)
		binary, err := su.GetVersion(context.TODO(), selfupdate.UpdateCmdFlags{Version: "0.11.23", BaseUrl: ts.URL})
		if err != nil {
			t.Fatal(err)
		}
		if string(binary) != "version 0.11.23 downloaded" {
			t.Fail()
		}
	})
	t.Run("download latest version", func(t *testing.T) {

		ts := httptest.NewServer(latestVersionHandler(t))
		defer ts.Close()
		su := selfupdate.New("my-binary", ts.URL)
		binary, err := su.GetVersion(context.TODO(), selfupdate.UpdateCmdFlags{Version: "latest", BaseUrl: ts.URL})
		if err != nil {
			t.Fatal(err)
		}
		if string(binary) != "latest version downloaded" {
			t.Fail()
		}
	})

	t.Run("with own suffix", func(t *testing.T) {

		ts := httptest.NewServer(latestVersionHandler(t))
		defer ts.Close()
		su := selfupdate.New("my-binary", ts.URL, selfupdate.WithDownloadSuffix("linux-amr64-mybinary"))
		binary, err := su.GetVersion(context.TODO(), selfupdate.UpdateCmdFlags{Version: "latest", BaseUrl: ts.URL})
		if err != nil {
			t.Fatal(err)
		}
		if string(binary) != "latest version downloaded" {
			t.Fail()
		}
	})

}

type mockOsFsOps struct {
	exec   func() (string, error)
	rename func(oldpath string, newpath string) error
	write  func(name string, data []byte, perm os.FileMode) error
}

func (o mockOsFsOps) Rename(oldpath string, newpath string) error {
	if o.rename != nil {
		return o.rename(oldpath, newpath)
	}
	return nil
}

func (o mockOsFsOps) WriteFile(name string, data []byte, perm os.FileMode) error {
	if o.write != nil {
		return o.write(name, data, perm)
	}
	return nil
}

func (o mockOsFsOps) Executable() (string, error) {
	if o.exec != nil {
		return o.exec()
	}
	return "/my/exec/binary", nil
}

func cmdHelper(t *testing.T, out, errOut io.Writer) *cobra.Command {
	t.Helper()
	rootCmd := &cobra.Command{}
	rootCmd.SetArgs([]string{"self-update"})
	rootCmd.SetErr(errOut)
	rootCmd.SetOut(out)
	return rootCmd
}

func TestUpdateCmd_RunFromRoot(t *testing.T) {

	t.Run("clean run", func(t *testing.T) {
		getFunc := func(ctx context.Context, flags selfupdate.UpdateCmdFlags) ([]byte, error) {
			// You can encapsulate the entire fetch logic in a custom function
			return []byte("my binary downloaded"), nil
		}
		errOut := output.NewSafeWriter(&bytes.Buffer{})
		stdOut := output.NewSafeWriter(&bytes.Buffer{})
		rootCmd := cmdHelper(t, stdOut, errOut)

		// See cmd/eirctl/eirctl.go for a more complete example
		uc := selfupdate.New("my-binary", "http://ignored.com", selfupdate.WithGetVersionFunc(getFunc), selfupdate.WithOsFsOps(mockOsFsOps{}))
		uc.AddToRootCommand(rootCmd)

		if err := rootCmd.ExecuteContext(context.TODO()); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("fails to get executable", func(t *testing.T) {
		getFunc := func(ctx context.Context, flags selfupdate.UpdateCmdFlags) ([]byte, error) {
			// You can encapsulate the entire fetch logic in a custom function
			return []byte("my binary downloaded"), nil
		}
		errOut := output.NewSafeWriter(&bytes.Buffer{})
		stdOut := output.NewSafeWriter(&bytes.Buffer{})
		rootCmd := cmdHelper(t, stdOut, errOut)

		// See cmd/eirctl/eirctl.go for a more complete example
		uc := selfupdate.New("my-binary", "http://ignored.com", selfupdate.WithGetVersionFunc(getFunc), selfupdate.WithOsFsOps(mockOsFsOps{exec: func() (string, error) { return "", fmt.Errorf("failed to get executable") }}))
		uc.AddToRootCommand(rootCmd)

		err := rootCmd.ExecuteContext(context.TODO())

		if err == nil {
			t.Fatal(err)
		}
	})

	t.Run("fails to get version from fetcher", func(t *testing.T) {
		getFunc := func(ctx context.Context, flags selfupdate.UpdateCmdFlags) ([]byte, error) {
			// You can encapsulate the entire fetch logic in a custom function
			return []byte{}, fmt.Errorf("fialed to fetch version")
		}
		errOut := output.NewSafeWriter(&bytes.Buffer{})
		stdOut := output.NewSafeWriter(&bytes.Buffer{})
		rootCmd := cmdHelper(t, stdOut, errOut)

		// See cmd/eirctl/eirctl.go for a more complete example
		uc := selfupdate.New("my-binary", "http://ignored.com",
			selfupdate.WithGetVersionFunc(getFunc),
			selfupdate.WithOsFsOps(mockOsFsOps{}))

		uc.AddToRootCommand(rootCmd)

		err := rootCmd.ExecuteContext(context.TODO())

		if err == nil {
			t.Fatal(err)
		}
	})
	t.Run("fails to prep source binary", func(t *testing.T) {
		getFunc := func(ctx context.Context, flags selfupdate.UpdateCmdFlags) ([]byte, error) {
			// You can encapsulate the entire fetch logic in a custom function
			return []byte{}, nil
		}
		errOut := output.NewSafeWriter(&bytes.Buffer{})
		stdOut := output.NewSafeWriter(&bytes.Buffer{})
		rootCmd := cmdHelper(t, stdOut, errOut)

		// See cmd/eirctl/eirctl.go for a more complete example
		uc := selfupdate.New("my-binary", "http://ignored.com",
			selfupdate.WithGetVersionFunc(getFunc),
			selfupdate.WithOsFsOps(mockOsFsOps{
				rename: func(oldpath, newpath string) error {
					return fmt.Errorf("failed to prep binary")
				},
			}))

		uc.AddToRootCommand(rootCmd)

		err := rootCmd.ExecuteContext(context.TODO())

		if err == nil {
			t.Fatal(err)
		}
	})
	t.Run("fails to write new binary", func(t *testing.T) {
		getFunc := func(ctx context.Context, flags selfupdate.UpdateCmdFlags) ([]byte, error) {
			// You can encapsulate the entire fetch logic in a custom function
			return []byte{}, nil
		}
		errOut := output.NewSafeWriter(&bytes.Buffer{})
		stdOut := output.NewSafeWriter(&bytes.Buffer{})
		rootCmd := cmdHelper(t, stdOut, errOut)

		// See cmd/eirctl/eirctl.go for a more complete example
		uc := selfupdate.New("my-binary", "http://ignored.com",
			selfupdate.WithGetVersionFunc(getFunc),
			selfupdate.WithOsFsOps(mockOsFsOps{
				write: func(name string, data []byte, perm os.FileMode) error {
					return fmt.Errorf("failed to write new binary")
				},
			}))

		uc.AddToRootCommand(rootCmd)

		err := rootCmd.ExecuteContext(context.TODO())

		if err == nil {
			t.Fatal(err)
		}
	})
}

// Example with custom GetVersionFunc
func ExampleUpdateCmd_withOwnGetFunc() {

	setOutput := ""
	getFunc := func(ctx context.Context, flags selfupdate.UpdateCmdFlags) ([]byte, error) {
		// You can encapsulate the entire fetch logic in a custom function
		setOutput = "my binary downloaded"
		return []byte(setOutput), nil
	}

	errOut := output.NewSafeWriter(&bytes.Buffer{})
	stdOut := output.NewSafeWriter(&bytes.Buffer{})
	rootCmd := cmdHelper(&testing.T{}, stdOut, errOut)

	// See cmd/eirctl/eirctl.go for a more complete example
	uc := selfupdate.New("my-binary", "http://ignored.com", selfupdate.WithGetVersionFunc(getFunc), selfupdate.WithOsFsOps(mockOsFsOps{}))
	uc.AddToRootCommand(rootCmd)

	if err := rootCmd.ExecuteContext(context.TODO()); err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(string(setOutput))
	fmt.Println(stdOut.String())

	// Output: my binary downloaded
	// my-binary has been updated
}
