package config_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/Ensono/eirctl/internal/config"
	"github.com/Ensono/eirctl/runner"
	"github.com/Ensono/eirctl/task"
	"github.com/Ensono/eirctl/variables"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/filesystem"
)

var sampleCfg = []byte(`{"tasks": {"task1": {"command": ["true"]}}}`)

// createFilesystemTestRepo creates a real, filesystem-backed Git repo in a tmp dir.
func createFilesystemTestRepo(t *testing.T, files map[string]string, branch string) (repo *git.Repository, dir string) {
	t.Helper()

	// Create temp directory for the repo
	dir, err := os.MkdirTemp("", "testrepo")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	fs := osfs.New(dir)
	dot := osfs.New(filepath.Join(dir, ".git"))
	storer := filesystem.NewStorage(dot, &cache.ObjectLRU{})

	repo, err = git.Init(storer, fs)
	if err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("failed to get worktree: %v", err)
	}

	for path, content := range files {
		fullPath := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("failed to create file dirs: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}
		if _, err := wt.Add(path); err != nil {
			t.Fatalf("failed to add file to index: %v", err)
		}
	}

	commitHash, err := wt.Commit("initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "tester",
			Email: "tester@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	if branch != "" && branch != "master" {
		ref := plumbing.NewHashReference(plumbing.NewBranchReferenceName(branch), commitHash)
		if err := repo.Storer.SetReference(ref); err != nil {
			t.Fatalf("failed to set branch ref: %v", err)
		}
	}

	return repo, dir
}

func TestLoader_Load_Git(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	_, dir := createFilesystemTestRepo(t, map[string]string{"eirctl.yaml": `
contexts:
  local_wth_quote:
    quote: "'"
tasks:
  task:git:
    command:
      - echo "from git"
`}, "")

	testYaml := fmt.Sprintf(`import:
  - git::file://%s//%s

tasks:
  task1:
    command:
      - echo true
`, dir, "eirctl.yaml")

	f, err := os.CreateTemp("", "test-yaml-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.Write([]byte(testYaml))
	defer os.Remove(dir)
	defer os.Remove(f.Name())

	cl := config.NewConfigLoader(config.NewConfig())
	cfg, err := cl.Load(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Tasks["task1"] == nil || cfg.Tasks["task1"].Commands[0] != "echo true" {
		t.Error("yaml parsing failed")
	}

	if cfg.Contexts["local_wth_quote"].Quote != `'` {
		t.Error("context's quote parsing failed")
	}

	cl = config.NewConfigLoader(config.NewConfig())
	cl.WithDir(filepath.Join(cwd, "testdata", "nested"))
	cfg, err = cl.Load("")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg.Tasks["test-task"]; !ok {
		t.Error("yaml parsing failed")
	}

	_, err = cl.LoadGlobalConfig()
	if err != nil {
		t.Fatal()
	}
}

func TestLoader_Load(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	cl := config.NewConfigLoader(config.NewConfig())
	cfg, err := cl.Load(filepath.Join(cwd, "testdata", "test.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Tasks["task1"] == nil || cfg.Tasks["task1"].Commands[0] != "echo true" {
		t.Error("yaml parsing failed")
	}

	if cfg.Contexts["local_wth_quote"].Quote != `'` {
		t.Error("context's quote parsing failed")
	}

	cl = config.NewConfigLoader(config.NewConfig())
	cl.WithDir(filepath.Join(cwd, "testdata", "nested"))
	cfg, err = cl.Load("")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg.Tasks["test-task"]; !ok {
		t.Error("yaml parsing failed")
	}

	_, err = cl.LoadGlobalConfig()
	if err != nil {
		t.Fatal()
	}
}

func Test_LoadImport(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "imprt-tes*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpFile.Name())
	testSrv := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/x-yaml")
		_, err := writer.Write([]byte(fmt.Sprintf(`
import:
  - %s
  - %s
tasks:
  task1:
    command:
      - true
`, tmpFile.Name(), tmpFile.Name())))
		if err != nil {
			t.Errorf("failed to write bytes to response stream")
		}
	}))
	loaderTYaml := fmt.Sprintf(`import:
  - %s
  - %s
  - %s
tasks:
  task2:
    command: echo true`, testSrv.URL, tmpFile.Name(), testSrv.URL)
	if _, err := tmpFile.Write([]byte(loaderTYaml)); err != nil {
		t.Fatal(err)
	}

	cl := config.NewConfigLoader(config.NewConfig())
	cfg, err := cl.Load(tmpFile.Name())
	if len(cfg.Tasks) != 2 {
		t.Errorf("got %v, wanted 2", len(cfg.Tasks))
	}
}
func TestLoader_resolveDefaultConfigFile(t *testing.T) {
	cl := config.NewConfigLoader(config.NewConfig())
	cl.WithDir(filepath.Join(cl.Dir(), "testdata"))

	file, err := cl.ResolveDefaultConfigFile()
	if err != nil {
		t.Fatal(err)
	}

	if filepath.Base(file) != "tasks.yaml" {
		t.Error()
	}

	cl.WithDir("/")
	file, err = cl.ResolveDefaultConfigFile()
	if err == nil || file != "" {
		t.Error()
	}
}

func TestLoader_LoadDirImport(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	cl := config.NewConfigLoader(config.NewConfig())
	conf, err := cl.Load(filepath.Join(cwd, "testdata", "dir-dep-import.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	if len(conf.Tasks) != 6 {
		t.Error()
	}
}

// TODO: tests for StringSlice in Unmarshall & Clash key

func TestLoader_ReadConfigFromURL(t *testing.T) {
	// yaml needs to be run separately "¯\_(ツ)_/¯"
	t.Run("yaml parsed correctly", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writer.Header().Set("Content-Type", "application/x-yaml")
			_, err := writer.Write([]byte(`
tasks:
  task1:
    command:
      - true
`))
			if err != nil {
				t.Errorf("failed to write bytes to response stream")
			}
		}))

		cl := config.NewConfigLoader(config.NewConfig())
		m, err := cl.Load(srv.URL)
		if err != nil {
			t.Fatal("got error, wanted nil")
		}
		if len(m.Tasks) != 1 {
			t.Errorf("got %v count, wanted %v task count", len(m.Tasks), 1)
		}
	})
}

func TestLoader_errors(t *testing.T) {
	t.Run("on failed status code", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			writer.WriteHeader(500)
		}))
		cl := config.NewConfigLoader(config.NewConfig())
		_, err := cl.Load(srv.URL)
		if err == nil {
			t.Fatal("got nil, wanted error")
		}
	})
}

func TestLoader_LoadGlobalConfig(t *testing.T) {
	h := os.TempDir()
	originalHomeNix, originalHomeWin := os.Getenv("HOME"), os.Getenv("USERPROFILE")
	os.Setenv("HOME", h)
	// windows...
	os.Setenv("USERPROFILE", h)

	defer func() {
		_ = os.RemoveAll(filepath.Join(h, ".eirctl"))
		os.Setenv("HOME", originalHomeNix)
		// windows...
		os.Setenv("USERPROFILE", originalHomeWin)
	}()

	err := os.Mkdir(filepath.Join(h, ".eirctl"), 0744)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(filepath.Join(h, ".eirctl", "config.yaml"), []byte(sampleCfg), 0644)
	if err != nil {
		t.Fatal(err)
	}

	cl := config.NewConfigLoader(config.NewConfig())
	// cl.homeDir = h
	cfg, err := cl.LoadGlobalConfig()
	if err != nil {
		t.Fatal()
	}

	if len(cfg.Tasks) == 0 {
		t.Error()
	}
}

func TestLoader_contexts(t *testing.T) {
	dir, _ := os.MkdirTemp(os.TempDir(), "context*")
	fname := filepath.Join(dir, "context.yaml")

	f, _ := os.Create(fname)
	defer os.RemoveAll(dir)
	f.Write([]byte(`contexts:
  docker:context:
    executable:
      bin: docker
      args:
        - "run"
        - "--rm"
        - "alpine"
        - "sh"
        - "-c"
    quote: "'"
    envfile:
      generate: true
      exclude:
        - PATH
  powershell:
    container:
      name: ensono/eir-infrastructure:1.1.251
      shell: pwsh
      shell_args:
        - -NonInteractive
        - -Command
      container_args: []
    envfile:
      exclude:
        - SOURCEVERSIONMESSAGE
        - JAVA
        - GO
        - HOMEBREW
  dind:
    container:
      name: ensono/eir-infrastructure:1.1.251
      enable_dind: true
      entrypoint: ["/usr/bin/env"]
      shell: bash
      shell_args:
        - -c
      container_args: []
    envfile:
      exclude:
        - SOURCEVERSIONMESSAGE
        - JAVA
        - GO
        - HOMEBREW
`))
	_ = os.Unsetenv("DOCKER_HOST")

	loader := config.NewConfigLoader(config.NewConfig())
	loader.WithStrictDecoder()
	def, err := loader.Load(fname)
	if err != nil {
		t.Fatal(err)
	}
	if len(def.Contexts) != 3 {
		t.Errorf("got: %v\nwanted: 3\n", len(def.Contexts))
	}
	pwshContainer, ok := def.Contexts["powershell"]
	if !ok {
		t.Errorf("powershell context not found")
	}
	dindContainer, ok := def.Contexts["dind"]
	if !ok {
		t.Errorf("dind context not found")
	}

	oldDockerContext, ok := def.Contexts["docker:context"]
	if !ok {
		t.Errorf("powershell context not found")
	}

	if pwshContainer.Container() == nil {
		t.Errorf("\npwshContainer IsContainer not correctly processed\n\ngot: %v\nwanted: false", pwshContainer.Container())
	}

	if dindContainer.Container() == nil {
		t.Errorf("\ndindContainer IsContainer not correctly processed\n\ngot: %v\nwanted: false", dindContainer.Container())
	}

	if oldDockerContext.Executable == nil {
		t.Errorf("\noldDockerContext IsContainer not correctly processed\n\ngot: %v\nwanted: false", oldDockerContext.Executable)
	}

	if len(dindContainer.Container().Volumes()) != 2 {
		t.Errorf("dindContainer incorrectly parsed args: %v", 2)
	}
}

func TestLoader_contexts_with_containerArgs(t *testing.T) {
	ttests := map[string]struct {
		contexts        []byte
		expectVolsCount int
	}{
		"includes forbidden args": {
			contexts: []byte(`contexts:
  test:args:
    container:
      name: ensono/eir-infrastructure:1.1.251
      shell: pwsh
      shell_args:
        - -NonInteractive
        - -Command
      container_args: ["--user foo","-v /var/run/docker.sock:/var/run/docker.sock"]
    envfile:
      exclude:
        - SOURCEVERSIONMESSAGE
        - JAVA
        - GO
        - HOMEBREW`),
			expectVolsCount: 1,
		},
		"includes NO forbidden args": {
			contexts: []byte(`contexts:
  test:args:
    container:
      name: ensono/eir-infrastructure:1.1.251
      shell: pwsh
      shell_args:
        - -NonInteractive
        - -Command
      container_args: ["--user foo","-v /foo:/foo"]
      enable_dind: true
    envfile:
      exclude:
        - SOURCEVERSIONMESSAGE
        - JAVA
        - GO
        - HOMEBREW`),
			expectVolsCount: 3,
		},
		"includes ONLY forbidden args": {
			contexts: []byte(`contexts:
  test:args:
    container:
      name: ensono/eir-infrastructure:1.1.251
      shell: pwsh
      shell_args:
        - -NonInteractive
        - -Command
      container_args: ["--privileged","-v /var/run/docker.sock:/var/run/docker.sock"]
    envfile:
      exclude:
        - SOURCEVERSIONMESSAGE
        - JAVA
        - GO
        - HOMEBREW`),
			expectVolsCount: 1,
		},
	}
	for name, tt := range ttests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir, _ := os.MkdirTemp(os.TempDir(), "context*")
			fname := filepath.Join(dir, "context.yaml")

			f, _ := os.Create(fname)
			defer os.RemoveAll(dir)
			f.Write(tt.contexts)
			loader := config.NewConfigLoader(config.NewConfig())
			loader.WithStrictDecoder()
			def, err := loader.Load(fname)
			if err != nil {
				t.Fatal(err)
			}

			testArgsContainer, ok := def.Contexts["test:args"]
			if !ok {
				t.Errorf("test:args context not found")
			}

			if testArgsContainer.Container() == nil {
				t.Errorf("\ntest:args IsContainer not correctly processed\n\ngot: %v\nwanted: Container", testArgsContainer.Container())
			}
			gotVols := testArgsContainer.Container().Volumes()
			if len(gotVols) != tt.expectVolsCount {
				t.Errorf("dindContainer set volumes count to %v, wanted: %v\n", len(gotVols), tt.expectVolsCount)
			}
			if !slices.Equal(testArgsContainer.Container().ShellArgs, []string{"pwsh", "-NonInteractive", "-Command"}) {
				t.Errorf("dindContainer incorrectly parsed shellArgs: %v, wanted: %v\n", testArgsContainer.Container().ShellArgs, []string{"pwsh", "-NonInteractive", "-Command"})
			}
		})
	}
}

func TestLoader_contexts_with_containerArgs_errors(t *testing.T) {
	ttests := map[string]struct {
		contexts []byte
	}{
		"includes user args duplicates": {
			contexts: []byte(`contexts:
  test:args:
    container:
      name: ensono/eir-infrastructure:1.1.251
      shell: pwsh
      shell_args:
        - -NonInteractive
        - -Command
      container_args: ["--user foo","-u foo", "-v /var/run/docker.sock:/var/run/docker.sock"]
    envfile:
      exclude:
        - SOURCEVERSIONMESSAGE
        - JAVA
        - GO
        - HOMEBREW`),
		},
	}
	for name, tt := range ttests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir, _ := os.MkdirTemp(os.TempDir(), "context*")
			fname := filepath.Join(dir, "context.yaml")

			f, _ := os.Create(fname)
			defer os.RemoveAll(dir)
			f.Write(tt.contexts)
			loader := config.NewConfigLoader(config.NewConfig())
			loader.WithStrictDecoder()
			_, err := loader.Load(fname)
			if err == nil {
				t.Fatal(err)
			}
		})
	}
}

func Test_Loader_Validate(t *testing.T) {

	t.Run("correctly references config", func(t *testing.T) {
		t.Parallel()
		mcfg := &config.Config{
			Tasks: map[string]*task.Task{
				"foo":         {Context: "exists"},
				"no_ctx_task": {Name: "no_ctx_task", Context: ""},
			},
			Contexts: map[string]*runner.ExecutionContext{
				"exists": runner.NewExecutionContext(nil, "", variables.NewVariables(), nil, nil, nil, nil, nil),
			},
		}
		cfg := config.NewConfigLoader(mcfg)
		if _, err := cfg.Validate(); err != nil {
			t.Errorf("got %v, wanted nil", err)
		}
	})

	t.Run("errors on missing context reference", func(t *testing.T) {
		t.Parallel()
		mcfg := &config.Config{
			Tasks: map[string]*task.Task{
				"foo": {Name: "foo", Context: "not_found"},
			},
			Contexts: map[string]*runner.ExecutionContext{
				"exists": runner.NewExecutionContext(nil, "", variables.NewVariables(), nil, nil, nil, nil, nil),
			},
		}
		cfg := config.NewConfigLoader(mcfg)
		_, err := cfg.Validate()
		if err == nil {
			t.Errorf("got nil, wanted %v", config.ErrValidation)
		}
		if !errors.Is(err, config.ErrValidation) {
			t.Errorf("incorrect error type thrown, got %q, wanted %q", err, config.ErrValidation)
		}
	})
}
