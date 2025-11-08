package selfupdate_test

import (
	"bytes"
	"context"
	"fmt"
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

// Example with custom GetVersionFunc
func ExampleUpdateCmd_withOwnGetFunc() {

	setOutput := ""
	getFunc := func(ctx context.Context, flags selfupdate.UpdateCmdFlags) ([]byte, error) {
		// You can encapsulate the entire fetch logic in a custom function
		setOutput = "my binary downloaded"
		return []byte(setOutput), nil
	}

	// See cmd/eirctl/eirctl.go for a more complete example
	rootCmd := &cobra.Command{}
	uc := selfupdate.New("my-binary", "http://ignored.com", selfupdate.WithGetVersionFunc(getFunc), selfupdate.WithOsFsOps(mockOsFsOps{}))
	uc.AddToRootCommand(rootCmd)

	//
	rootCmd.SetArgs([]string{"self-update"})
	errOut := output.NewSafeWriter(&bytes.Buffer{})
	stdOut := output.NewSafeWriter(&bytes.Buffer{})
	rootCmd.SetErr(errOut)
	rootCmd.SetOut(stdOut)

	if err := rootCmd.ExecuteContext(context.TODO()); err != nil {
		fmt.Println(err)
	}

	fmt.Println(string(setOutput))
	fmt.Println(stdOut.String())

	// Output: my binary downloaded
	// my-binary has been updated
}
