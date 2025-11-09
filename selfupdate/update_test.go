package selfupdate_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
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

type mockCloser struct {
	io.Writer
}

func (m mockCloser) Close() error {
	return nil
}

func Test_Update_GetVersion(t *testing.T) {

	t.Run("download specific version", func(t *testing.T) {
		ts := httptest.NewServer(specificVersionHandler(t))
		defer ts.Close()
		su := selfupdate.New("my-binary", ts.URL)
		binary := &bytes.Buffer{}
		err := su.GetVersion(context.TODO(), selfupdate.UpdateCmdFlags{Version: "0.11.23", BaseUrl: ts.URL}, mockCloser{binary})
		if err != nil {
			t.Fatal(err)
		}
		if binary.String() != "version 0.11.23 downloaded" {
			t.Fail()
		}
	})
	t.Run("download latest version", func(t *testing.T) {

		ts := httptest.NewServer(latestVersionHandler(t))
		defer ts.Close()
		binary := &bytes.Buffer{}
		su := selfupdate.New("my-binary", ts.URL)
		err := su.GetVersion(context.TODO(), selfupdate.UpdateCmdFlags{Version: "latest", BaseUrl: ts.URL}, mockCloser{binary})
		if err != nil {
			t.Fatal(err)
		}
		if binary.String() != "latest version downloaded" {
			t.Fail()
		}
	})

	t.Run("with own suffix", func(t *testing.T) {

		ts := httptest.NewServer(latestVersionHandler(t))
		defer ts.Close()
		binary := &bytes.Buffer{}

		su := selfupdate.New("my-binary", ts.URL, selfupdate.WithDownloadSuffix("linux-amr64-mybinary"))

		err := su.GetVersion(context.TODO(), selfupdate.UpdateCmdFlags{Version: "latest", BaseUrl: ts.URL}, mockCloser{binary})
		if err != nil {
			t.Fatal(err)
		}
		if binary.String() != "latest version downloaded" {
			t.Fail()
		}
	})

}

type mockOsFsOps struct {
	exec   func() (string, error)
	rename func(oldpath string, newpath string) error
	create func(name string) (io.WriteCloser, error)
}

func (o mockOsFsOps) Rename(oldpath string, newpath string) error {
	if o.rename != nil {
		return o.rename(oldpath, newpath)
	}
	return nil
}

func (o mockOsFsOps) Create(name string) (io.WriteCloser, error) {
	if o.create != nil {
		return o.create(name)
	}
	return mockCloser{&bytes.Buffer{}}, nil
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
		getFunc := func(ctx context.Context, flags selfupdate.UpdateCmdFlags, w io.WriteCloser) error {
			// You can encapsulate the entire fetch logic in a custom function
			return nil
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
		getFunc := func(ctx context.Context, flags selfupdate.UpdateCmdFlags, w io.WriteCloser) error {
			// You can encapsulate the entire fetch logic in a custom function
			return nil
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
		getFunc := func(ctx context.Context, flags selfupdate.UpdateCmdFlags, w io.WriteCloser) error {
			// You can encapsulate the entire fetch logic in a custom function
			return fmt.Errorf("failed to fetch version")
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
		getFunc := func(ctx context.Context, flags selfupdate.UpdateCmdFlags, w io.WriteCloser) error {
			// You can encapsulate the entire fetch logic in a custom function
			return nil
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
		getFunc := func(ctx context.Context, flags selfupdate.UpdateCmdFlags, w io.WriteCloser) error {
			// You can encapsulate the entire fetch logic in a custom function
			return nil
		}
		errOut := output.NewSafeWriter(&bytes.Buffer{})
		stdOut := output.NewSafeWriter(&bytes.Buffer{})
		rootCmd := cmdHelper(t, stdOut, errOut)

		// See cmd/eirctl/eirctl.go for a more complete example
		uc := selfupdate.New("my-binary", "http://ignored.com",
			selfupdate.WithGetVersionFunc(getFunc),
			selfupdate.WithOsFsOps(mockOsFsOps{
				create: func(name string) (io.WriteCloser, error) {
					return nil, fmt.Errorf("failed to write new binary")
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
	getFunc := func(ctx context.Context, flags selfupdate.UpdateCmdFlags, w io.WriteCloser) error {
		// You can encapsulate the entire fetch logic in a custom function
		setOutput = "binary(contents....)"
		return nil
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

	// Output: binary(contents....)
	// my-binary has been updated
}
