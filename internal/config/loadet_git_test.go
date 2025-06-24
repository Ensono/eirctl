package config_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/Ensono/eirctl/internal/config"
	"github.com/Ensono/eirctl/internal/utils"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
	"gopkg.in/yaml.v3"
)

// helper to create an in-memory test git repo
func createTestRepo(t *testing.T, files map[string]string, branch string, refName string) *git.Repository {
	t.Helper()

	storer := memory.NewStorage()
	fs := memfs.New()

	repo, err := git.Init(storer, fs)
	if err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("failed to get worktree: %v", err)
	}

	for path, content := range files {
		f, err := fs.Create(path)
		if err != nil {
			t.Fatalf("failed to create file: %v", err)
		}
		if _, err := f.Write([]byte(content)); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}
		f.Close()

		_, err = wt.Add(path)
		if err != nil {
			t.Fatalf("failed to add file: %v", err)
		}
	}

	commit, err := wt.Commit("initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "tester",
			Email: "tester@example.com",
		},
	})
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	// Optionally create a branch reference
	if branch != "" && branch != "main" {
		ref := plumbing.NewHashReference(plumbing.NewBranchReferenceName(branch), commit)
		err = repo.Storer.SetReference(ref)
		if err != nil {
			t.Fatalf("set branch ref: %v", err)
		}
	}
	return repo
}

func Test_NewGitSource_ValidInput(t *testing.T) {
	ttests := map[string]struct {
		rawString             string
		wantCheckoutStr       string
		wantTag, wantYamlPath string
	}{
		"ssh without ref": {
			rawString:       "git::ssh://github.com/example/repo//config.yaml",
			wantCheckoutStr: "git@github.com:example/repo",
			wantTag:         "",
			wantYamlPath:    "config.yaml",
		},
		"https without ref": {
			rawString:       "git::https://github.com/example/repo//config.yaml",
			wantCheckoutStr: "https://github.com/example/repo",
			wantTag:         "",
			wantYamlPath:    "config.yaml",
		},
		"file without ref": {
			// it is the user's responsibility to add the preceeding slash if the path is absolute
			rawString:       "git::file:///path/to/repo//config.yaml",
			wantCheckoutStr: "/path/to/repo",
			wantTag:         "",
			wantYamlPath:    "config.yaml",
		},
		"ssh with ref": {
			rawString:       "git::ssh://github.com/example/repo//config.yaml?ref=v1.0.1",
			wantCheckoutStr: "git@github.com:example/repo",
			wantTag:         "v1.0.1",
			wantYamlPath:    "config.yaml",
		},
	}
	for name, tt := range ttests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			gs, err := config.NewGitSource(tt.rawString)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gs.Tag() != tt.wantTag {
				t.Errorf("Tag got '%s', wanted: %s", gs.Tag(), tt.wantTag)
			}
			if gs.GitCheckoutStr() != tt.wantCheckoutStr {
				t.Errorf("GitCheckoutStr got '%s', wanted: %s", gs.GitCheckoutStr(), tt.wantCheckoutStr)
			}
			if gs.YamlPath() != tt.wantYamlPath {
				t.Errorf("YamlPath got '%s', wanted: %s", gs.YamlPath(), tt.wantYamlPath)
			}
		})
	}
}
func TestGitSource_Config_FromHead(t *testing.T) {
	dummyConfig := &config.ConfigDefinition{
		Contexts: map[string]*config.ContextDefinition{
			"foo": {Container: &utils.Container{Name: "bar.io/qux"}},
		},
	}
	cfgWriter := &bytes.Buffer{}
	_ = yaml.NewEncoder(cfgWriter).Encode(dummyConfig)

	repo := createTestRepo(t, map[string]string{
		"config.yaml": cfgWriter.String(),
	}, "", "")

	gs, err := config.NewGitSource("git::ssh://bar.org//config.yaml")
	if err != nil {
		t.Fatalf("NewGitSource error: %v", err)
	}
	gs.WithRepo(repo)

	cfg, err := gs.Config()
	if err != nil {
		t.Fatalf("Config error: %v", err)
	}

	val, ok := cfg.Contexts["foo"]
	if !ok {
		t.Errorf("expected context 'foo', got '%v'", cfg.Contexts)
	}
	if val.Container != nil && val.Container.Name != "bar.io/qux" {
		t.Error("expected Container.Name 'bar.io/qux', got nil")
	}
}

func TestGitSource_Config_FromBranch(t *testing.T) {
	dummyConfig := &config.ConfigDefinition{
		Contexts: map[string]*config.ContextDefinition{
			"foo": {Container: &utils.Container{Name: "bar.io/qux"}},
		},
	}
	cfgWriter := &bytes.Buffer{}
	_ = yaml.NewEncoder(cfgWriter).Encode(dummyConfig)

	repo := createTestRepo(t, map[string]string{
		"my.yaml": cfgWriter.String(),
	}, "feature/foo", "")

	gs, err := config.NewGitSource("git::https://example.com/org/repo//my.yaml?ref=feature/foo")
	if err != nil {
		t.Fatalf("NewGitSource error: %v", err)
	}
	gs.WithRepo(repo)

	cfg, err := gs.Config()
	if err != nil {
		t.Fatalf("Config error: %v", err)
	}
	val, ok := cfg.Contexts["foo"]
	if !ok {
		t.Errorf("expected context 'foo', got '%v'", cfg.Contexts)
	}
	if val.Container != nil && val.Container.Name != "bar.io/qux" {
		t.Error("expected Container.Name 'bar.io/qux', got nil")
	}
}

func TestNewGitSource_InvalidFormat(t *testing.T) {
	_, err := config.NewGitSource("invalid-url-format")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, config.ErrIncorrectlyFormattedGit) {
		t.Errorf("expected %v, got: %v", config.ErrIncorrectlyFormattedGit, err)
	}
}

func TestGitSource_Config_FileNotFound(t *testing.T) {
	dummyConfig := &config.ConfigDefinition{
		Contexts: map[string]*config.ContextDefinition{
			"foo": {Container: &utils.Container{Name: "bar.io/qux"}},
		},
	}
	cfgWriter := &bytes.Buffer{}
	_ = yaml.NewEncoder(cfgWriter).Encode(dummyConfig)

	repo := createTestRepo(t, map[string]string{
		"my.yaml": cfgWriter.String(),
	}, "", "")

	gs, err := config.NewGitSource("git::file://path/on/disk/repo//missing.yaml")
	if err != nil {
		t.Fatalf("NewGitSource error: %v", err)
	}
	gs.WithRepo(repo)

	_, err = gs.Config()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, config.ErrGitOperation) && !strings.Contains(err.Error(), "file not found") {
		t.Errorf("expected file not found error, got: %v", err)
	}
}

func Test_LoaderGit_Integration(t *testing.T) {
	ttests := map[string]struct {
		rawStr string
	}{
		"use https over git public":               {"git::https://github.com/Ensono/eirctl.git//shared/security/scaning.yaml"},
		"use straight git protocol and with .git": {"git::https://github.com/Ensono/eirctl.git//shared/security/scaning.yaml"},
		"ref with branch":                         {"git::https://github.com/Ensono/eirctl//shared/security/scaning.yaml?ref=main"},
		"ref with complex tag":                    {"git::https://github.com/Ensono/eirctl//shared/security/scaning.yaml?ref=0.7.0"},
	}

	for name, tt := range ttests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			gs, err := config.NewGitSource(tt.rawStr)
			if err != nil {
				t.Fatal(err)
			}
			if err := gs.Clone(); err != nil {
				t.Fatal(err)
			}
			cfg, err := gs.Config()
			if err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(gs.GitCheckoutStr(), "https://github.com/Ensono/eirctl") {
				t.Errorf("got %s, wanted the string to begin with`https://github.com/Ensono/eirctl`", gs.GitCheckoutStr())
			}
			if cfg == nil {
				t.Fatal("empty contents received")
			}
		})
	}
}
