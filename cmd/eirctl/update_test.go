package cmd_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"

	cmd "github.com/Ensono/eirctl/cmd/eirctl"
)

func getBinarySuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}

	return ""
}

func specificVersionHandler(t *testing.T) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc(fmt.Sprintf("/download/0.11.23/eirctl-%s-%s%s", runtime.GOOS, runtime.GOARCH, getBinarySuffix()), func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write([]byte(`version 0.11.23 downloaded`))
	})
	return mux
}

func latestVersionHandler(t *testing.T) http.Handler {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc(fmt.Sprintf("/latest/download/eirctl-%s-%s%s", runtime.GOOS, runtime.GOARCH, getBinarySuffix()), func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write([]byte(`latest version downloaded`))
	})
	return mux
}
func Test_Update_GetVersion(t *testing.T) {

	t.Run("download specific version", func(t *testing.T) {
		ts := httptest.NewServer(specificVersionHandler(t))
		defer ts.Close()
		binary, err := cmd.GetVersion(context.TODO(), ts.URL, "0.11.23")
		if err != nil {
			t.Fatal(err)
		}
		if string(binary) != "version 0.11.23 downloaded" {
			t.Fatal(string(binary))
		}
	})
	t.Run("download latest version", func(t *testing.T) {
		ts := httptest.NewServer(latestVersionHandler(t))
		defer ts.Close()
		binary, err := cmd.GetVersion(context.TODO(), ts.URL, "latest")
		if err != nil {
			t.Fatal(err)
		}
		if string(binary) != "latest version downloaded" {
			t.Fail()
		}
	})
	t.Run("integration test", func(t *testing.T) {
		t.Skip()
		binary, err := cmd.GetVersion(context.TODO(), "https://github.com/Ensono/eirctl/releases", "latest")
		if err != nil {
			t.Fatal(err)
		}
		if string(binary) != "latest version downloaded" {
			t.Fail()
		}
	})
}

func Test_Update_Command(t *testing.T) {
	// we cannot realistically test the entire flow as it would overwrite the go binary :|
	t.Run("successfully returns the help for subcommand", func(t *testing.T) {
		//
		cmdRunTestHelper(t, &cmdRunTestInput{
			args:    []string{"-c", "testdata/graph.yaml", "update", "-h"},
			errored: false,
		})
	})
	t.Run("successfully writes the new binary", func(t *testing.T) {
		//
		cmdRunTestHelper(t, &cmdRunTestInput{
			args:    []string{"-c", "testdata/imports.yaml", "update"},
			errored: false,
		})
	})
}
