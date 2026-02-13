package config_test

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
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
	defer os.RemoveAll(dir)
	defer os.RemoveAll(f.Name())

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
	cfg, _ := cl.Load(tmpFile.Name())

	if cfg == nil {
		t.Fatal("got nil cfg")
	}

	if len(cfg.Tasks) != 2 {
		t.Fatalf("got %v, wanted 2", len(cfg.Tasks))
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

func Test_ImportFiles_LocalFile(t *testing.T) {
	// Create a temp directory for the project
	projectDir, err := os.MkdirTemp("", "import-files-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(projectDir)

	// Create a script file to import
	scriptContent := "#!/bin/bash\necho deployed\n"
	scriptFile := filepath.Join(projectDir, "deploy.sh")
	if err := os.WriteFile(scriptFile, []byte(scriptContent), 0755); err != nil {
		t.Fatal(err)
	}

	// Create eirctl config with import referencing the local file
	configContent := fmt.Sprintf(`
import:
  - src: %s
    dest: deploy.sh

tasks:
  task1:
    command:
      - echo hello
`, scriptFile)

	configFile := filepath.Join(projectDir, "eirctl.yaml")
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	cl := config.NewConfigLoader(config.NewConfig())
	cl.WithDir(projectDir)
	_, err = cl.Load(configFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the file was written to project root (explicit dest)
	destPath := filepath.Join(projectDir, "deploy.sh")
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("expected file at %s, got error: %v", destPath, err)
	}
	if string(content) != scriptContent {
		t.Errorf("got %q, wanted %q", string(content), scriptContent)
	}
}

func Test_ImportFiles_NestedDest(t *testing.T) {
	projectDir, err := os.MkdirTemp("", "import-files-nested-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(projectDir)

	scriptContent := "#!/bin/bash\necho terraform init\n"
	scriptFile := filepath.Join(projectDir, "init.sh")
	if err := os.WriteFile(scriptFile, []byte(scriptContent), 0755); err != nil {
		t.Fatal(err)
	}

	// dest includes subdirectory
	configContent := fmt.Sprintf(`
import:
  - src: %s
    dest: terraform/init.sh

tasks:
  task1:
    command:
      - echo hello
`, scriptFile)

	configFile := filepath.Join(projectDir, "eirctl.yaml")
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	cl := config.NewConfigLoader(config.NewConfig())
	cl.WithDir(projectDir)
	_, err = cl.Load(configFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	destPath := filepath.Join(projectDir, "terraform", "init.sh")
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("expected file at %s, got error: %v", destPath, err)
	}
	if string(content) != scriptContent {
		t.Errorf("got %q, wanted %q", string(content), scriptContent)
	}
}

func Test_ImportFiles_URL(t *testing.T) {
	scriptContent := "#!/bin/bash\necho from url\n"
	testSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(scriptContent))
	}))
	defer testSrv.Close()

	projectDir, err := os.MkdirTemp("", "import-files-url-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(projectDir)

	configContent := fmt.Sprintf(`
import:
  - src: %s/scripts/deploy.sh
    dest: deploy.sh

tasks:
  task1:
    command:
      - echo hello
`, testSrv.URL)

	configFile := filepath.Join(projectDir, "eirctl.yaml")
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	cl := config.NewConfigLoader(config.NewConfig())
	cl.WithDir(projectDir)
	_, err = cl.Load(configFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	destPath := filepath.Join(projectDir, "deploy.sh")
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("expected file at %s, got error: %v", destPath, err)
	}
	if string(content) != scriptContent {
		t.Errorf("got %q, wanted %q", string(content), scriptContent)
	}
}

func Test_ImportFiles_Git(t *testing.T) {
	scriptContent := "#!/bin/bash\necho from git\n"
	_, dir := createFilesystemTestRepo(t, map[string]string{
		"scripts/deploy.sh": scriptContent,
	}, "")
	defer os.RemoveAll(dir)

	projectDir, err := os.MkdirTemp("", "import-files-git-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(projectDir)

	configContent := fmt.Sprintf(`
import:
  - src: git::file://%s//scripts/deploy.sh
    dest: deploy.sh

tasks:
  task1:
    command:
      - echo hello
`, dir)

	configFile := filepath.Join(projectDir, "eirctl.yaml")
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	cl := config.NewConfigLoader(config.NewConfig())
	cl.WithDir(projectDir)
	_, err = cl.Load(configFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	destPath := filepath.Join(projectDir, "deploy.sh")
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("expected file at %s, got error: %v", destPath, err)
	}
	if string(content) != scriptContent {
		t.Errorf("got %q, wanted %q", string(content), scriptContent)
	}
}

func Test_ImportFiles_EmptySrc(t *testing.T) {
	projectDir, err := os.MkdirTemp("", "import-files-empty-src-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(projectDir)

	configContent := `
import:
  - src: ""
    dest: deploy.sh

tasks:
  task1:
    command:
      - echo hello
`

	configFile := filepath.Join(projectDir, "eirctl.yaml")
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	cl := config.NewConfigLoader(config.NewConfig())
	cl.WithDir(projectDir)
	_, err = cl.Load(configFile)
	if err == nil {
		t.Fatal("expected error for empty src")
	}
	if !errors.Is(err, config.ErrImportFileFailed) {
		t.Errorf("incorrect error type, got %v, wanted %v", err, config.ErrImportFileFailed)
	}
}

func Test_ImportFiles_NoImportFiles(t *testing.T) {
	// Ensure no error when no file imports are present
	projectDir, err := os.MkdirTemp("", "import-files-none-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(projectDir)

	configContent := `
tasks:
  task1:
    command:
      - echo hello
`

	configFile := filepath.Join(projectDir, "eirctl.yaml")
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	cl := config.NewConfigLoader(config.NewConfig())
	cl.WithDir(projectDir)
	_, err = cl.Load(configFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func Test_ImportFiles_PathTraversal(t *testing.T) {
	projectDir, err := os.MkdirTemp("", "import-files-traversal-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(projectDir)

	scriptContent := "#!/bin/bash\necho pwned\n"
	scriptFile := filepath.Join(projectDir, "evil.sh")
	if err := os.WriteFile(scriptFile, []byte(scriptContent), 0755); err != nil {
		t.Fatal(err)
	}

	configContent := fmt.Sprintf(`
import:
  - src: %s
    dest: ../../../etc/evil.sh

tasks:
  task1:
    command:
      - echo hello
`, scriptFile)

	configFile := filepath.Join(projectDir, "eirctl.yaml")
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	cl := config.NewConfigLoader(config.NewConfig())
	cl.WithDir(projectDir)
	_, err = cl.Load(configFile)
	if err == nil {
		t.Fatal("expected error for path traversal in dest")
	}
	if !errors.Is(err, config.ErrPathTraversal) {
		t.Errorf("incorrect error type, got %v, wanted %v", err, config.ErrPathTraversal)
	}
}

func Test_GetBaseFilename(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "git URL with ref query param",
			src:  "git::ssh://github.com/org/repo//scripts/deploy.sh?ref=v1.0.32",
			want: "deploy.sh",
		},
		{
			name: "git URL with branch ref",
			src:  "git::file:///path/to/repo//create-local-envfile.sh?ref=main",
			want: "create-local-envfile.sh",
		},
		{
			name: "https URL with query params",
			src:  "https://example.com/script.sh?token=abc123",
			want: "script.sh",
		},
		{
			name: "local path no query",
			src:  "/path/to/script.sh",
			want: "script.sh",
		},
		{
			name: "relative path no query",
			src:  "scripts/deploy.sh",
			want: "deploy.sh",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := config.GetBaseFilename(tt.src)
			if got != tt.want {
				t.Errorf("getBaseFilename(%q) = %q, want %q", tt.src, got, tt.want)
			}
		})
	}
}

func Test_IsDirectoryImport(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want bool
	}{
		{
			name: "git URL with trailing slash",
			src:  "git::ssh://github.com/org/repo//scripts/",
			want: true,
		},
		{
			name: "git URL with trailing slash and ref",
			src:  "git::ssh://github.com/org/repo//scripts/?ref=main",
			want: true,
		},
		{
			name: "git URL without trailing slash",
			src:  "git::ssh://github.com/org/repo//scripts/deploy.sh",
			want: false,
		},
		{
			name: "local path with trailing slash",
			src:  "scripts/",
			want: true,
		},
		{
			name: "local path without trailing slash",
			src:  "scripts/deploy.sh",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := config.IsDirectoryImport(tt.src)
			if got != tt.want {
				t.Errorf("IsDirectoryImport(%q) = %v, want %v", tt.src, got, tt.want)
			}
		})
	}
}

func Test_ImportFiles_LocalDir(t *testing.T) {
	// Create a source directory with multiple files in a nested structure
	srcDir, err := os.MkdirTemp("", "import-dir-src-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(srcDir)

	files := map[string]string{
		"deploy.sh":     "#!/bin/bash\necho deploy\n",
		"init.sh":       "#!/bin/bash\necho init\n",
		"sub/nested.sh": "#!/bin/bash\necho nested\n",
	}
	for relPath, content := range files {
		fullPath := filepath.Join(srcDir, relPath)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	projectDir, err := os.MkdirTemp("", "import-dir-project-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(projectDir)

	// Write eirctl config that imports the directory (trailing slash)
	cfgContent := fmt.Sprintf(`import:
  - src: %s/
    dest: scripts
`, srcDir)
	cfgPath := filepath.Join(projectDir, "eirctl.yaml")
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	cl := config.NewConfigLoader(config.NewConfig())
	cl.WithDir(projectDir)
	_, err = cl.Load(cfgPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All files should be in scripts/ preserving relative paths
	for relPath, expectedContent := range files {
		destPath := filepath.Join(projectDir, "scripts", relPath)
		content, err := os.ReadFile(destPath)
		if err != nil {
			t.Errorf("expected file at %s, got error: %v", destPath, err)
			continue
		}
		if string(content) != expectedContent {
			t.Errorf("file %s: got %q, want %q", relPath, string(content), expectedContent)
		}
	}
}

func Test_ImportFiles_LocalDirExplicitDest(t *testing.T) {
	srcDir, err := os.MkdirTemp("", "import-dir-dest-src-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(srcDir)

	if err := os.WriteFile(filepath.Join(srcDir, "build.sh"), []byte("#!/bin/bash\necho build\n"), 0644); err != nil {
		t.Fatal(err)
	}

	projectDir, err := os.MkdirTemp("", "import-dir-dest-project-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(projectDir)

	cfgContent := fmt.Sprintf(`import:
  - src: %s/
    dest: my-scripts
`, srcDir)
	cfgPath := filepath.Join(projectDir, "eirctl.yaml")
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	cl := config.NewConfigLoader(config.NewConfig())
	cl.WithDir(projectDir)
	_, err = cl.Load(cfgPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	destPath := filepath.Join(projectDir, "my-scripts", "build.sh")
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("expected file at %s, got error: %v", destPath, err)
	}
	if string(content) != "#!/bin/bash\necho build\n" {
		t.Errorf("got %q, want %q", string(content), "#!/bin/bash\necho build\n")
	}
}

func Test_ImportFiles_GitDir(t *testing.T) {
	// Create git repo with a scripts/ directory containing multiple files
	_, dir := createFilesystemTestRepo(t, map[string]string{
		"scripts/deploy.sh": "#!/bin/bash\necho deploy from git\n",
		"scripts/init.sh":   "#!/bin/bash\necho init from git\n",
		"other/unrelated":   "should not be imported",
	}, "")
	defer os.RemoveAll(dir)

	projectDir, err := os.MkdirTemp("", "import-gitdir-project-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(projectDir)

	cfgContent := fmt.Sprintf(`import:
  - src: git::file://%s//scripts/
    dest: scripts
`, dir)
	cfgPath := filepath.Join(projectDir, "eirctl.yaml")
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	cl := config.NewConfigLoader(config.NewConfig())
	cl.WithDir(projectDir)
	_, err = cl.Load(cfgPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// deploy.sh and init.sh should be in scripts/
	for _, name := range []string{"deploy.sh", "init.sh"} {
		destPath := filepath.Join(projectDir, "scripts", name)
		if _, err := os.Stat(destPath); os.IsNotExist(err) {
			t.Errorf("expected file %s to exist", destPath)
		}
	}

	// unrelated file from other/ should NOT be present
	unrelatedPath := filepath.Join(projectDir, "scripts", "unrelated")
	if _, err := os.Stat(unrelatedPath); !os.IsNotExist(err) {
		t.Errorf("did not expect %s to exist", unrelatedPath)
	}
}

func Test_ImportFiles_DirURLNotSupported(t *testing.T) {
	projectDir, err := os.MkdirTemp("", "import-dir-url-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(projectDir)

	cfgContent := `import:
  - src: https://example.com/scripts/
    dest: scripts
`
	cfgPath := filepath.Join(projectDir, "eirctl.yaml")
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	cl := config.NewConfigLoader(config.NewConfig())
	cl.WithDir(projectDir)
	_, err = cl.Load(cfgPath)
	if err == nil {
		t.Fatal("expected error for HTTP directory import, got nil")
	}
	if !errors.Is(err, config.ErrImportFileFailed) {
		t.Errorf("expected ErrImportFileFailed, got: %v", err)
	}
}

// sha256Hex computes the SHA-256 hex digest of content.
func sha256Hex(content []byte) string {
	h := sha256.Sum256(content)
	return hex.EncodeToString(h[:])
}

func Test_ImportFiles_HashMatch(t *testing.T) {
	projectDir, err := os.MkdirTemp("", "import-hash-match-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(projectDir)

	scriptContent := []byte("#!/bin/bash\necho secure\n")
	scriptFile := filepath.Join(projectDir, "secure.sh")
	if err := os.WriteFile(scriptFile, scriptContent, 0755); err != nil {
		t.Fatal(err)
	}

	hash := "sha256:" + sha256Hex(scriptContent)

	configContent := fmt.Sprintf(`
import:
  - src: %s
    dest: secure.sh
    hash: %s

tasks:
  task1:
    command:
      - echo hello
`, scriptFile, hash)

	configFile := filepath.Join(projectDir, "eirctl.yaml")
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	cl := config.NewConfigLoader(config.NewConfig())
	cl.WithDir(projectDir)
	_, err = cl.Load(configFile)
	if err != nil {
		t.Fatalf("expected no error for matching hash, got: %v", err)
	}

	destPath := filepath.Join(projectDir, "secure.sh")
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("expected file at %s, got error: %v", destPath, err)
	}
	if string(content) != string(scriptContent) {
		t.Errorf("got %q, wanted %q", string(content), string(scriptContent))
	}
}

func Test_ImportFiles_HashMismatch(t *testing.T) {
	projectDir, err := os.MkdirTemp("", "import-hash-mismatch-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(projectDir)

	scriptContent := []byte("#!/bin/bash\necho tampered\n")
	scriptFile := filepath.Join(projectDir, "tampered.sh")
	if err := os.WriteFile(scriptFile, scriptContent, 0755); err != nil {
		t.Fatal(err)
	}

	// Use a deliberately wrong hash
	wrongHash := "sha256:0000000000000000000000000000000000000000000000000000000000000000"

	configContent := fmt.Sprintf(`
import:
  - src: %s
    dest: tampered.sh
    hash: %s

tasks:
  task1:
    command:
      - echo hello
`, scriptFile, wrongHash)

	configFile := filepath.Join(projectDir, "eirctl.yaml")
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	cl := config.NewConfigLoader(config.NewConfig())
	cl.WithDir(projectDir)
	_, err = cl.Load(configFile)
	if err == nil {
		t.Fatal("expected error for hash mismatch, got nil")
	}
	if !errors.Is(err, config.ErrHashMismatch) {
		t.Errorf("expected ErrHashMismatch, got: %v", err)
	}
}

func Test_ImportFiles_UnsupportedHashAlgorithm(t *testing.T) {
	projectDir, err := os.MkdirTemp("", "import-hash-unsupported-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(projectDir)

	scriptContent := []byte("#!/bin/bash\necho algo\n")
	scriptFile := filepath.Join(projectDir, "algo.sh")
	if err := os.WriteFile(scriptFile, scriptContent, 0755); err != nil {
		t.Fatal(err)
	}

	configContent := fmt.Sprintf(`
import:
  - src: %s
    dest: algo.sh
    hash: md5:d41d8cd98f00b204e9800998ecf8427e

tasks:
  task1:
    command:
      - echo hello
`, scriptFile)

	configFile := filepath.Join(projectDir, "eirctl.yaml")
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	cl := config.NewConfigLoader(config.NewConfig())
	cl.WithDir(projectDir)
	_, err = cl.Load(configFile)
	if err == nil {
		t.Fatal("expected error for unsupported hash algorithm, got nil")
	}
	if !errors.Is(err, config.ErrUnsupportedHashAlgorithm) {
		t.Errorf("expected ErrUnsupportedHashAlgorithm, got: %v", err)
	}
}

func Test_ImportFiles_HashBadFormat(t *testing.T) {
	projectDir, err := os.MkdirTemp("", "import-hash-badformat-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(projectDir)

	scriptContent := []byte("#!/bin/bash\necho badformat\n")
	scriptFile := filepath.Join(projectDir, "bad.sh")
	if err := os.WriteFile(scriptFile, scriptContent, 0755); err != nil {
		t.Fatal(err)
	}

	configContent := fmt.Sprintf(`
import:
  - src: %s
    dest: bad.sh
    hash: nocolon

tasks:
  task1:
    command:
      - echo hello
`, scriptFile)

	configFile := filepath.Join(projectDir, "eirctl.yaml")
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	cl := config.NewConfigLoader(config.NewConfig())
	cl.WithDir(projectDir)
	_, err = cl.Load(configFile)
	if err == nil {
		t.Fatal("expected error for bad hash format, got nil")
	}
	if !errors.Is(err, config.ErrUnsupportedHashAlgorithm) {
		t.Errorf("expected ErrUnsupportedHashAlgorithm, got: %v", err)
	}
}

func Test_ImportFiles_DirectoryHashMatch(t *testing.T) {
	srcDir, err := os.MkdirTemp("", "import-dirhash-match-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(srcDir)

	files := map[string]string{
		"a.sh": "#!/bin/bash\necho a\n",
		"b.sh": "#!/bin/bash\necho b\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(srcDir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Compute canonical hash: sorted paths, "<path>\n<content>" concatenated
	h := sha256.New()
	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		h.Write([]byte(p))
		h.Write([]byte("\n"))
		h.Write([]byte(files[p]))
	}
	dirHash := "sha256:" + hex.EncodeToString(h.Sum(nil))

	projectDir, err := os.MkdirTemp("", "import-dirhash-project-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(projectDir)

	cfgContent := fmt.Sprintf(`import:
  - src: %s/
    dest: scripts
    hash: %s
`, srcDir, dirHash)
	cfgPath := filepath.Join(projectDir, "eirctl.yaml")
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	cl := config.NewConfigLoader(config.NewConfig())
	cl.WithDir(projectDir)
	_, err = cl.Load(cfgPath)
	if err != nil {
		t.Fatalf("expected no error for matching directory hash, got: %v", err)
	}
}

func Test_ImportFiles_DirectoryHashMismatch(t *testing.T) {
	srcDir, err := os.MkdirTemp("", "import-dirhash-mismatch-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(srcDir)

	if err := os.WriteFile(filepath.Join(srcDir, "a.sh"), []byte("echo a"), 0644); err != nil {
		t.Fatal(err)
	}

	projectDir, err := os.MkdirTemp("", "import-dirhash-mismatch-project-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(projectDir)

	cfgContent := fmt.Sprintf(`import:
  - src: %s/
    dest: scripts
    hash: sha256:0000000000000000000000000000000000000000000000000000000000000000
`, srcDir)
	cfgPath := filepath.Join(projectDir, "eirctl.yaml")
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	cl := config.NewConfigLoader(config.NewConfig())
	cl.WithDir(projectDir)
	_, err = cl.Load(cfgPath)
	if err == nil {
		t.Fatal("expected error for directory hash mismatch, got nil")
	}
	if !errors.Is(err, config.ErrHashMismatch) {
		t.Errorf("expected ErrHashMismatch, got: %v", err)
	}
}

func Test_ImportFiles_NoHashSkipsVerification(t *testing.T) {
	projectDir, err := os.MkdirTemp("", "import-nohash-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(projectDir)

	scriptContent := []byte("#!/bin/bash\necho nohash\n")
	scriptFile := filepath.Join(projectDir, "nohash.sh")
	if err := os.WriteFile(scriptFile, scriptContent, 0755); err != nil {
		t.Fatal(err)
	}

	// No hash field — should succeed without verification
	configContent := fmt.Sprintf(`
import:
  - src: %s
    dest: nohash.sh

tasks:
  task1:
    command:
      - echo hello
`, scriptFile)

	configFile := filepath.Join(projectDir, "eirctl.yaml")
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	cl := config.NewConfigLoader(config.NewConfig())
	cl.WithDir(projectDir)
	_, err = cl.Load(configFile)
	if err != nil {
		t.Fatalf("expected no error when hash is omitted, got: %v", err)
	}
}

func Test_UnifiedImport_MixedStringAndObject(t *testing.T) {
	// Create a project dir with a shared config and a script file
	projectDir, err := os.MkdirTemp("", "unified-import-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(projectDir)

	// Create a shared config to import as string
	sharedConfig := `
tasks:
  shared_task:
    command:
      - echo shared
`
	sharedFile := filepath.Join(projectDir, "shared.yaml")
	if err := os.WriteFile(sharedFile, []byte(sharedConfig), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a script to import as file
	scriptContent := []byte("#!/bin/bash\necho from unified\n")
	scriptFile := filepath.Join(projectDir, "deploy.sh")
	if err := os.WriteFile(scriptFile, scriptContent, 0755); err != nil {
		t.Fatal(err)
	}

	hash := "sha256:" + sha256Hex(scriptContent)

	// Unified import: string entry (config) + object entry (file import with hash)
	configContent := fmt.Sprintf(`
import:
  - shared.yaml
  - src: %s
    dest: deploy.sh
    hash: %s

tasks:
  main_task:
    command:
      - echo main
`, scriptFile, hash)

	configFile := filepath.Join(projectDir, "eirctl.yaml")
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	cl := config.NewConfigLoader(config.NewConfig())
	cl.WithDir(projectDir)
	cfg, err := cl.Load(configFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the shared config task was imported
	if _, ok := cfg.Tasks["shared_task"]; !ok {
		t.Error("expected shared_task from config import")
	}
	if _, ok := cfg.Tasks["main_task"]; !ok {
		t.Error("expected main_task from main config")
	}

	// Verify the file was written
	destPath := filepath.Join(projectDir, "deploy.sh")
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("expected file at %s, got error: %v", destPath, err)
	}
	if string(content) != string(scriptContent) {
		t.Errorf("got %q, wanted %q", string(content), string(scriptContent))
	}
}

func Test_UnifiedImport_ObjectConfigImportWithHash(t *testing.T) {
	// Object form in import without dest = config import, hash should be silently accepted
	// (hash verification for config imports is a follow-up)
	projectDir, err := os.MkdirTemp("", "unified-config-hash-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(projectDir)

	sharedConfig := `
tasks:
  hashed_task:
    command:
      - echo hashed
`
	sharedFile := filepath.Join(projectDir, "hashed.yaml")
	if err := os.WriteFile(sharedFile, []byte(sharedConfig), 0644); err != nil {
		t.Fatal(err)
	}

	// Object form without dest — treated as config import
	configContent := fmt.Sprintf(`
import:
  - src: %s

tasks:
  main_task:
    command:
      - echo main
`, sharedFile)

	configFile := filepath.Join(projectDir, "eirctl.yaml")
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	cl := config.NewConfigLoader(config.NewConfig())
	cl.WithDir(projectDir)
	cfg, err := cl.Load(configFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := cfg.Tasks["hashed_task"]; !ok {
		t.Error("expected hashed_task from config import via object form")
	}
}

func Test_UnifiedImport_FileImportHashMismatch(t *testing.T) {
	projectDir, err := os.MkdirTemp("", "unified-file-hash-mismatch-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(projectDir)

	scriptContent := []byte("#!/bin/bash\necho bad\n")
	scriptFile := filepath.Join(projectDir, "bad.sh")
	if err := os.WriteFile(scriptFile, scriptContent, 0755); err != nil {
		t.Fatal(err)
	}

	configContent := fmt.Sprintf(`
import:
  - src: %s
    dest: bad.sh
    hash: sha256:0000000000000000000000000000000000000000000000000000000000000000

tasks:
  task1:
    command:
      - echo hello
`, scriptFile)

	configFile := filepath.Join(projectDir, "eirctl.yaml")
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	cl := config.NewConfigLoader(config.NewConfig())
	cl.WithDir(projectDir)
	_, err = cl.Load(configFile)
	if err == nil {
		t.Fatal("expected error for hash mismatch in unified import")
	}
	if !errors.Is(err, config.ErrHashMismatch) {
		t.Errorf("expected ErrHashMismatch, got: %v", err)
	}
}

func Test_UnifiedImport_BackwardCompatStringOnly(t *testing.T) {
	// Ensure pure string imports still work (backward compatibility)
	projectDir, err := os.MkdirTemp("", "unified-compat-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(projectDir)

	sharedConfig := `
tasks:
  compat_task:
    command:
      - echo compat
`
	sharedFile := filepath.Join(projectDir, "compat.yaml")
	if err := os.WriteFile(sharedFile, []byte(sharedConfig), 0644); err != nil {
		t.Fatal(err)
	}

	configContent := `
import:
  - compat.yaml

tasks:
  main_task:
    command:
      - echo main
`

	configFile := filepath.Join(projectDir, "eirctl.yaml")
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	cl := config.NewConfigLoader(config.NewConfig())
	cl.WithDir(projectDir)
	cfg, err := cl.Load(configFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := cfg.Tasks["compat_task"]; !ok {
		t.Error("expected compat_task from string config import")
	}
	if _, ok := cfg.Tasks["main_task"]; !ok {
		t.Error("expected main_task from main config")
	}
}

func Test_ImportFiles_URL_Non200Status(t *testing.T) {
	testSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer testSrv.Close()

	projectDir, err := os.MkdirTemp("", "import-url-404-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(projectDir)

	configContent := fmt.Sprintf(`
import:
  - src: %s/scripts/deploy.sh
    dest: deploy.sh

tasks:
  task1:
    command:
      - echo hello
`, testSrv.URL)

	configFile := filepath.Join(projectDir, "eirctl.yaml")
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	cl := config.NewConfigLoader(config.NewConfig())
	cl.WithDir(projectDir)
	_, err = cl.Load(configFile)
	if err == nil {
		t.Fatal("expected error for non-200 HTTP response")
	}
	if !errors.Is(err, config.ErrImportFileFailed) {
		t.Errorf("expected ErrImportFileFailed, got: %v", err)
	}
}

func Test_ImportFiles_URL_ContentLengthTooLarge(t *testing.T) {
	testSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set Content-Length much larger than MaxImportFileSize (10MB)
		w.Header().Set("Content-Length", "999999999999")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("small body"))
	}))
	defer testSrv.Close()

	projectDir, err := os.MkdirTemp("", "import-url-toolarge-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(projectDir)

	configContent := fmt.Sprintf(`
import:
  - src: %s/bigfile.bin
    dest: bigfile.bin

tasks:
  task1:
    command:
      - echo hello
`, testSrv.URL)

	configFile := filepath.Join(projectDir, "eirctl.yaml")
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	cl := config.NewConfigLoader(config.NewConfig())
	cl.WithDir(projectDir)
	_, err = cl.Load(configFile)
	if err == nil {
		t.Fatal("expected error for oversized Content-Length")
	}
	if !strings.Contains(err.Error(), "remote file too large") {
		t.Errorf("expected 'remote file too large' error, got: %v", err)
	}
}

func Test_ImportFiles_LocalFileNotFound(t *testing.T) {
	projectDir, err := os.MkdirTemp("", "import-notfound-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(projectDir)

	configContent := `
import:
  - src: nonexistent-script.sh
    dest: deploy.sh

tasks:
  task1:
    command:
      - echo hello
`

	configFile := filepath.Join(projectDir, "eirctl.yaml")
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	cl := config.NewConfigLoader(config.NewConfig())
	cl.WithDir(projectDir)
	_, err = cl.Load(configFile)
	if err == nil {
		t.Fatal("expected error for missing local file")
	}
	if !errors.Is(err, config.ErrImportFileFailed) {
		t.Errorf("expected ErrImportFileFailed, got: %v", err)
	}
}

func Test_ImportFiles_LocalDirNotADirectory(t *testing.T) {
	projectDir, err := os.MkdirTemp("", "import-notadir-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(projectDir)

	// Create a regular file, but reference it with trailing slash (directory import)
	regularFile := filepath.Join(projectDir, "notadir")
	if err := os.WriteFile(regularFile, []byte("i am a file"), 0644); err != nil {
		t.Fatal(err)
	}

	// Use trailing slash to trigger directory import on a non-directory path
	configContent := fmt.Sprintf(`
import:
  - src: %s/
    dest: scripts/

tasks:
  task1:
    command:
      - echo hello
`, regularFile)

	configFile := filepath.Join(projectDir, "eirctl.yaml")
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	cl := config.NewConfigLoader(config.NewConfig())
	cl.WithDir(projectDir)
	_, err = cl.Load(configFile)
	if err == nil {
		t.Fatal("expected error for non-directory path with trailing slash")
	}
	if !errors.Is(err, config.ErrImportFileFailed) {
		t.Errorf("expected ErrImportFileFailed, got: %v", err)
	}
}

func Test_ImportFiles_LocalDirEmpty(t *testing.T) {
	projectDir, err := os.MkdirTemp("", "import-emptydir-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(projectDir)

	// Create an empty directory to import
	emptyDir := filepath.Join(projectDir, "empty")
	if err := os.MkdirAll(emptyDir, 0755); err != nil {
		t.Fatal(err)
	}

	configContent := fmt.Sprintf(`
import:
  - src: %s/
    dest: scripts/

tasks:
  task1:
    command:
      - echo hello
`, emptyDir)

	configFile := filepath.Join(projectDir, "eirctl.yaml")
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	cl := config.NewConfigLoader(config.NewConfig())
	cl.WithDir(projectDir)
	_, err = cl.Load(configFile)
	if err == nil {
		t.Fatal("expected error for empty directory import")
	}
	if !errors.Is(err, config.ErrImportFileFailed) {
		t.Errorf("expected ErrImportFileFailed, got: %v", err)
	}
}

func Test_ImportFiles_DirectoryHashBadFormat(t *testing.T) {
	projectDir, err := os.MkdirTemp("", "import-dirhash-badformat-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(projectDir)

	// Create a source directory with a file
	srcDir := filepath.Join(projectDir, "scripts")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "run.sh"), []byte("#!/bin/bash\necho run\n"), 0755); err != nil {
		t.Fatal(err)
	}

	configContent := fmt.Sprintf(`
import:
  - src: %s/
    dest: imported-scripts/
    hash: not-a-valid-hash-format

tasks:
  task1:
    command:
      - echo hello
`, srcDir)

	configFile := filepath.Join(projectDir, "eirctl.yaml")
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	cl := config.NewConfigLoader(config.NewConfig())
	cl.WithDir(projectDir)
	_, err = cl.Load(configFile)
	if err == nil {
		t.Fatal("expected error for bad hash format on directory import")
	}
	if !errors.Is(err, config.ErrUnsupportedHashAlgorithm) {
		t.Errorf("expected ErrUnsupportedHashAlgorithm, got: %v", err)
	}
}

func Test_ImportFiles_DirectoryHashUnsupportedAlgorithm(t *testing.T) {
	projectDir, err := os.MkdirTemp("", "import-dirhash-unsupported-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(projectDir)

	// Create a source directory with a file
	srcDir := filepath.Join(projectDir, "scripts")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "run.sh"), []byte("#!/bin/bash\necho run\n"), 0755); err != nil {
		t.Fatal(err)
	}

	configContent := fmt.Sprintf(`
import:
  - src: %s/
    dest: imported-scripts/
    hash: md5:d41d8cd98f00b204e9800998ecf8427e

tasks:
  task1:
    command:
      - echo hello
`, srcDir)

	configFile := filepath.Join(projectDir, "eirctl.yaml")
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	cl := config.NewConfigLoader(config.NewConfig())
	cl.WithDir(projectDir)
	_, err = cl.Load(configFile)
	if err == nil {
		t.Fatal("expected error for unsupported hash algorithm on directory import")
	}
	if !errors.Is(err, config.ErrUnsupportedHashAlgorithm) {
		t.Errorf("expected ErrUnsupportedHashAlgorithm, got: %v", err)
	}
}

func Test_ImportFiles_DirStatError(t *testing.T) {
	projectDir, err := os.MkdirTemp("", "import-dir-staterr-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(projectDir)

	configContent := `
import:
  - src: /nonexistent/path/that/does/not/exist/
    dest: scripts/

tasks:
  task1:
    command:
      - echo hello
`

	configFile := filepath.Join(projectDir, "eirctl.yaml")
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	cl := config.NewConfigLoader(config.NewConfig())
	cl.WithDir(projectDir)
	_, err = cl.Load(configFile)
	if err == nil {
		t.Fatal("expected error for nonexistent directory path")
	}
	if !errors.Is(err, config.ErrImportFileFailed) {
		t.Errorf("expected ErrImportFileFailed, got: %v", err)
	}
}
