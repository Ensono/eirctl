package config_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
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

var oh = os.Getenv("HOME")

func createDummySshConf(t *testing.T) func() {
	t.Helper()
	tmpHomeDir, _ := os.MkdirTemp("", "ssh-conf-*")
	tmpSShNew := filepath.Join(tmpHomeDir, ".ssh")
	_ = os.Mkdir(tmpSShNew, 0777)
	sshConfFile, _ := os.Create(filepath.Join(tmpSShNew, "config"))
	_, _ = sshConfFile.Write([]byte(`
Host github.com
    Hostname ssh.github.com
    Port 443
    User git
`))
	sshIdFile, _ := os.Create(filepath.Join(tmpSShNew, "id_ed25519"))
	sshIdFile.Write([]byte(`-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
QyNTUxOQAAACCJBIEqsaMRDEzAD3jnx/aonMCQ3TNzJ9s9nE0Z9oAhYQAAAJA//SKQP/0i
kAAAAAtzc2gtZWQyNTUxOQAAACCJBIEqsaMRDEzAD3jnx/aonMCQ3TNzJ9s9nE0Z9oAhYQ
AAAECpHtGcC8b9PcJOr2CYYatl0UyZdgRG8+M6Rm/Z6ncY4IkEgSqxoxEMTMAPeOfH9qic
wJDdM3Mn2z2cTRn2gCFhAAAADXRlc3RAdGVzdC5jb20=
-----END OPENSSH PRIVATE KEY-----
`))
	os.Setenv("HOME", tmpHomeDir)
	return func() {
		os.RemoveAll(tmpHomeDir)
		os.Setenv("HOME", oh)
	}
}

func Test_NewGitSource_ValidInput(t *testing.T) {
	ttests := map[string]struct {
		rawString             string
		wantCheckoutStr       string
		wantTag, wantYamlPath string
	}{
		"ssh without ref": {
			rawString:       "git::ssh://github.com/example/repo//config.yaml",
			wantCheckoutStr: "ssh://git@ssh.github.com:443/example/repo",
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
			wantCheckoutStr: "ssh://git@ssh.github.com:443/example/repo",
			wantTag:         "v1.0.1",
			wantYamlPath:    "config.yaml",
		},
	}
	for name, tt := range ttests {
		t.Run(name, func(t *testing.T) {
			cleanUp := createDummySshConf(t)
			defer cleanUp()
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

func Test_NewGitSource_ValidInput_withSSH_COMMAND(t *testing.T) {
	ttests := map[string]struct {
		rawString             string
		wantCheckoutStr       string
		wantTag, wantYamlPath string
		cleanUp               func() (string, string, func())
	}{
		"use default ssh config": {
			rawString:       "git::ssh://github.com/example/repo//config.yaml",
			wantCheckoutStr: "ssh://git@ssh.github.com:443/example/repo",
			wantTag:         "",
			wantYamlPath:    "config.yaml",
			cleanUp: func() (string, string, func()) {
				return "", "", func() {}
			},
		},
		"specify identity over GIT_SSH_COMMAND": {
			rawString:       "git::ssh://github.com/example/repo//config.yaml",
			wantCheckoutStr: "ssh://git@ssh.github.com:443/example/repo",
			wantTag:         "",
			wantYamlPath:    "config.yaml",
			cleanUp: func() (string, string, func()) {
				tempFile, _ := os.CreateTemp("", "id-rando-*")
				tempFile.Write([]byte(`-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
QyNTUxOQAAACCJBIEqsaMRDEzAD3jnx/aonMCQ3TNzJ9s9nE0Z9oAhYQAAAJA//SKQP/0i
kAAAAAtzc2gtZWQyNTUxOQAAACCJBIEqsaMRDEzAD3jnx/aonMCQ3TNzJ9s9nE0Z9oAhYQ
AAAECpHtGcC8b9PcJOr2CYYatl0UyZdgRG8+M6Rm/Z6ncY4IkEgSqxoxEMTMAPeOfH9qic
wJDdM3Mn2z2cTRn2gCFhAAAADXRlc3RAdGVzdC5jb20=
-----END OPENSSH PRIVATE KEY-----
		`))
				os.Setenv(config.GitSshCommandVar, "ssh -i "+tempFile.Name())
				return tempFile.Name(), "", func() {
					os.Unsetenv(config.GitSshCommandVar)
					os.Remove(tempFile.Name())
				}
			},
		},
		"specify config over GIT_SSH_COMMAND": {
			rawString:       "git::ssh://github.com/example/repo//config.yaml",
			wantCheckoutStr: "ssh://git@ssh.github.com:4443/example/repo",
			wantTag:         "",
			wantYamlPath:    "config.yaml",
			cleanUp: func() (string, string, func()) {
				tempFile, _ := os.CreateTemp("", "config-rando-*")
				tempFile.Write([]byte(`Host github.com
		Hostname ssh.github.com
		Port 4443
		`))
				os.Setenv(config.GitSshCommandVar, "ssh -F "+tempFile.Name())
				return "", tempFile.Name(), func() {
					os.Unsetenv(config.GitSshCommandVar)
					os.Remove(tempFile.Name())
				}
			},
		},
		"specify identiy and config over GIT_SSH_COMMAND": {
			rawString:       "git::ssh://github.com/example/repo//config.yaml",
			wantCheckoutStr: "ssh://git@ssh.github.com:443/example/repo",
			wantTag:         "",
			wantYamlPath:    "config.yaml",
			cleanUp: func() (string, string, func()) {
				sshConfFile, _ := os.CreateTemp("", "config-rando-*")
				sshConfFile.Write([]byte(`Host github.com
		Port 443
		Hostname ssh.github.com
		`))
				sshIdFile, _ := os.CreateTemp("", "id-rando-*")
				sshIdFile.Write([]byte(`-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
QyNTUxOQAAACCJBIEqsaMRDEzAD3jnx/aonMCQ3TNzJ9s9nE0Z9oAhYQAAAJA//SKQP/0i
kAAAAAtzc2gtZWQyNTUxOQAAACCJBIEqsaMRDEzAD3jnx/aonMCQ3TNzJ9s9nE0Z9oAhYQ
AAAECpHtGcC8b9PcJOr2CYYatl0UyZdgRG8+M6Rm/Z6ncY4IkEgSqxoxEMTMAPeOfH9qic
wJDdM3Mn2z2cTRn2gCFhAAAADXRlc3RAdGVzdC5jb20=
-----END OPENSSH PRIVATE KEY-----
		`))
				os.Setenv(config.GitSshCommandVar, "ssh -F "+sshConfFile.Name()+" -i "+sshIdFile.Name())
				return sshIdFile.Name(), sshConfFile.Name(), func() {
					os.Unsetenv(config.GitSshCommandVar)
					os.Remove(sshConfFile.Name())
					os.Remove(sshIdFile.Name())
				}
			},
		},
	}
	for name, tt := range ttests {
		t.Run(name, func(t *testing.T) {
			cleanUp := createDummySshConf(t)
			defer cleanUp()
			idfile, configFile, setupClean := tt.cleanUp()
			defer setupClean()

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
			if idfile != "" && gs.SshConfig.IdentityFile != idfile {
				t.Errorf("identity file incorrect, got %s\nwanted %s", gs.SshConfig.IdentityFile, idfile)
			}
			if configFile != "" && gs.SshConfig.ConfigFile != configFile {
				t.Errorf("config file incorrect, got %s\nwanted %s", gs.SshConfig.IdentityFile, configFile)
			}
		})
	}
}

func Test_NewGitSource_ValidInput_withSSH_COMMAND_hostname_port(t *testing.T) {
	cleanUp := createDummySshConf(t)
	defer cleanUp()
	setup := func() func() {
		os.Setenv(config.GitSshCommandVar, "ssh -oHostname=altssh.github.org -oPort=443")
		return func() {
			os.Unsetenv(config.GitSshCommandVar)
		}
	}
	setupClean := setup()
	defer setupClean()

	gs, err := config.NewGitSource("git::ssh://github.com/example/repo//config.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gs.SshConfig.Hostname != "altssh.github.org" {
		t.Errorf("got %s, wanted: altssh.github.org", gs.SshConfig.Hostname)
	}
}

func TestGitSource_Config_FromHead(t *testing.T) {
	cleanUp := createDummySshConf(t)
	defer cleanUp()
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
		// "use ssh over git public": {"git::ssh://github.com/Ensono/eirctl.git//shared/security/scaning.yaml"},
		"use https over git public":               {"git::https://github.com/Ensono/eirctl.git//shared/security/eirctl.yaml"},
		"use straight git protocol and with .git": {"git::https://github.com/Ensono/eirctl.git//shared/security/eirctl.yaml"},
		"ref with branch":                         {"git::https://github.com/Ensono/eirctl//shared/security/eirctl.yaml?ref=main"},
		"ref with complex tag":                    {"git::https://github.com/Ensono/eirctl//shared/security/scaning.yaml?ref=0.7.0"},
		"ref with sha1":                           {"git::https://github.com/Ensono/eirctl//shared/security/scaning.yaml?ref=2f7bd79dc9b88a328f536bd97b8358c37d7abb9e"},
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

func Test_LoaderGit_SSHKeySigner(t *testing.T) {

	keyWithPassphrase := []byte(`-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAACmFlczI1Ni1jYmMAAAAGYmNyeXB0AAAAGAAAABDzGKF3uX
G1gXALZKFd6Ir4AAAAEAAAAAEAAAAzAAAAC3NzaC1lZDI1NTE5AAAAIDne4/teO42zTDdj
NwxUMNpbfmp/dxgU4ZNkC3ydgcugAAAAoJ3J/oA7+iqVOz0CIUUk9ufdP1VP4jDf2um+0s
Sgs7x6Gpyjq67Ps7wLRdSmxr/G5b+Z8dRGFYS/wUCQEe3whwuImvLyPwWjXLzkAyMzc01f
ywBGSrHnvP82ppenc2HuTI+E05Xc02i6JVyI1ShiekQL5twoqtR6pEBZnD17UonIx7cRzZ
gbDGyT3bXMQtagvCwoW+/oMTKXiZP5jCJpEO8=
-----END OPENSSH PRIVATE KEY-----`)
	keyWithoutPassphrase := []byte(`-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
QyNTUxOQAAACCJBIEqsaMRDEzAD3jnx/aonMCQ3TNzJ9s9nE0Z9oAhYQAAAJA//SKQP/0i
kAAAAAtzc2gtZWQyNTUxOQAAACCJBIEqsaMRDEzAD3jnx/aonMCQ3TNzJ9s9nE0Z9oAhYQ
AAAECpHtGcC8b9PcJOr2CYYatl0UyZdgRG8+M6Rm/Z6ncY4IkEgSqxoxEMTMAPeOfH9qic
wJDdM3Mn2z2cTRn2gCFhAAAADXRlc3RAdGVzdC5jb20=
-----END OPENSSH PRIVATE KEY-----`)
	t.Run("correctly uses passphrase", func(t *testing.T) {
		os.Setenv(config.GitSshPassphrase, "password")
		defer os.Unsetenv(config.GitSshPassphrase)
		signer, err := config.SSHKeySigner(keyWithPassphrase)
		if err != nil {
			t.Error(err)
		}
		if signer.PublicKey().Type() != "ssh-ed25519" {
			t.Errorf("incorrectly deduced public key - got: %v, wanted 'ssh-ed25519'", signer.PublicKey().Type())
		}
	})

	t.Run("fails with wrong passphrase", func(t *testing.T) {
		os.Setenv(config.GitSshPassphrase, "wrong")
		defer os.Unsetenv(config.GitSshPassphrase)
		_, err := config.SSHKeySigner(keyWithPassphrase)
		if err == nil {
			t.Error(err)
		}
	})

	t.Run("correctly uses key without passphrase", func(t *testing.T) {
		signer, err := config.SSHKeySigner(keyWithoutPassphrase)
		if err != nil {
			t.Error(err)
		}
		if signer.PublicKey().Type() != "ssh-ed25519" {
			t.Errorf("incorrectly deduced public key - got: %v, wanted 'ssh-ed25519'", signer.PublicKey().Type())
		}
	})

	t.Run("fails on no passphrase provided for the key", func(t *testing.T) {
		_, err := config.SSHKeySigner(keyWithPassphrase)
		if err == nil {
			t.Fatal("failed to error out on missing passphrase")
		}
		if !errors.Is(err, config.ErrGitOperation) {
			t.Fatalf("incorrect error type, got %v, wanted %v", err, config.ErrGitOperation)
		}
	})
}
