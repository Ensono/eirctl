package selfupdate_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Ensono/eirctl/cmdutils/selfupdate"
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
}
