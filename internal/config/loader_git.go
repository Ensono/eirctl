package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Ensono/eirctl/internal/utils"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/kevinburke/ssh_config"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v3"
)

const (
	gitPrefix              = "git::"
	gitPathSeparator       = "//"
	sshGitConnectionString = "ssh://%s@%s:%s/%s" // user@host:port/org/repo
	GitSshCommandVar       = "GIT_SSH_COMMAND"
)

var (
	// GitRegExp must begin with git::
	// must include a protocol to use
	// must have a repo url and path to file specified
	gitRegexp                    = regexp.MustCompile(`^git::(ssh|https?|file)://(.+?)//([^?]+)(?:\?ref=([^&]+))?$`)
	ErrIncorrectlyFormattedGit   = errors.New("incorrectly formatted git import, must satisfy this regex `^git::(ssh|https?|file)://(.+?)//([^?]+)(?:\\?ref=([^&]+))?$`")
	ErrGitTagBranchRevisionWrong = errors.New("tag or branch or revision was not found")
	ErrGitOperation              = errors.New("git operation failed")
)

type GitSource struct {
	repo      *git.Repository
	gcOpts    *git.CloneOptions
	yamlPath  string
	tag       string
	SshConfig SSHConfigAuth
}

type SSHConfigAuth struct {
	IdentityFile string
	ConfigFile   string
	User         string
	Port         string
	Hostname     string
}

func IsGit(raw string) bool {
	return strings.HasPrefix(raw, gitPrefix)
}

func NewGitSource(raw string) (*GitSource, error) {
	gs := &GitSource{gcOpts: &git.CloneOptions{
		// specifically set this here so that later it is a known value
		RemoteName: "origin",
		Depth:      0,
	}}

	gitImportParts := gitRegexp.FindStringSubmatch(raw)
	if len(gitImportParts) != 5 {
		return gs, fmt.Errorf("import %s, %w", raw, ErrIncorrectlyFormattedGit)
	}

	switch gitImportParts[1] {
	case "ssh":
		p1 := strings.Split(gitImportParts[2], "/")
		// auth using ssh_config
		auth, err := gs.getGitSSHAuth(p1[0])
		if err != nil {
			return nil, err
		}
		gs.gcOpts.URL = fmt.Sprintf(sshGitConnectionString, gs.SshConfig.User, gs.SshConfig.Hostname, gs.SshConfig.Port, strings.Join(p1[1:], "/"))
		gs.gcOpts.Auth = auth
	case "http", "https":
		gs.gcOpts.URL = "https://" + gitImportParts[2]
	case "file":
		gs.gcOpts.URL = gitImportParts[2]
	default:
		return nil, fmt.Errorf("must specify a protocol (ssh|https|file)\n%w", ErrIncorrectlyFormattedGit)
	}

	gs.yamlPath = gitImportParts[3]
	gs.tag = gitImportParts[4]

	return gs, nil
}

// Clone calls the git clone operation
func (gs *GitSource) Clone() (err error) {
	if gs.repo, err = git.Clone(memory.NewStorage(), nil, gs.gcOpts); err != nil {
		return err
	}
	return nil
}

func (gs *GitSource) YamlPath() string {
	return gs.yamlPath
}

func (gs *GitSource) GitCheckoutStr() string {
	return gs.gcOpts.URL
}

func (gs *GitSource) Tag() string {
	return gs.tag
}

func (gs *GitSource) WithRepo(repo *git.Repository) {
	gs.repo = repo
}

func (gs *GitSource) Config() (*ConfigDefinition, error) {
	commit, err := gs.getCommit(gs.repo)
	if err != nil {
		return nil, fmt.Errorf("%w\nerror: %v", ErrGitOperation, err)
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("%w\nerror: %v", ErrGitOperation, err)
	}
	file, err := tree.File(gs.yamlPath)
	if err != nil {
		return nil, fmt.Errorf("%w\nerror: %v", ErrGitOperation, err)
	}
	contents, err := file.Reader()
	if err != nil {
		return nil, fmt.Errorf("%w\nerror: %v", ErrGitOperation, err)
	}

	defer contents.Close()

	cm := &ConfigDefinition{}
	if err := yaml.NewDecoder(contents).Decode(&cm); err != nil {
		return nil, err
	}
	return cm, nil
}

type getCommitFunc func(r *git.Repository, tag string) (*object.Commit, error)

var getCommitFuncFallback []getCommitFunc = []getCommitFunc{
	func(r *git.Repository, tag string) (*object.Commit, error) {
		rev, err := r.ResolveRevision(plumbing.Revision(tag))
		if err != nil {
			return nil, fmt.Errorf("%w, gone through all fallbacks", ErrGitTagBranchRevisionWrong)
		}
		return r.CommitObject(plumbing.NewHashReference("", *rev).Hash())
	},
	func(r *git.Repository, tag string) (*object.Commit, error) {
		return r.CommitObject(plumbing.NewHash(tag))
	},
}

func (gs *GitSource) getCommit(r *git.Repository) (*object.Commit, error) {
	// If tag or branch was specified, check out the correct commit
	if gs.tag != "" {
		for _, fn := range getCommitFuncFallback {
			if c, e := fn(r, gs.tag); e == nil && c != nil {
				return c, nil
			}
		}
		return nil, fmt.Errorf("%w, gone through all fallbacks", ErrGitTagBranchRevisionWrong)
	}
	// use default current head
	// Get file content from HEAD commit - as a default commit
	ref, err := r.Head()
	if err != nil {
		return nil, fmt.Errorf("get HEAD: %w", err)
	}
	return r.CommitObject(ref.Hash())
}

// parseGitSshCommandEnv looks for the conventional GIT_SSH_COMMAND variable
// if set it will use the values for identity and/or config file
func parseGitSshCommandEnv() (string, string) {
	flagSet := pflag.NewFlagSet("gitsshcommand", pflag.ContinueOnError)
	_ = flagSet.StringP("identity", "i", "", "identity file - i.e. the private key to use")
	_ = flagSet.StringP("file", "F", "", "config file override")
	gsc := os.Getenv(GitSshCommandVar)
	if err := flagSet.Parse(strings.Fields(gsc)); err != nil {
		logrus.Debugf("%s: %s", GitSshCommandVar, gsc)
	}
	identityFile, _ := flagSet.GetString("identity")
	configFile, _ := flagSet.GetString("file")
	return identityFile, configFile
}

func getSshConfigFile() (identityFile, configFile string) {
	identityFile, configFile = parseGitSshCommandEnv()
	homeDir := filepath.Join(utils.MustGetUserHomeDir())
	if configFile == "" {
		// try default location if not specified via GIT_SSH_COMMAND
		for _, cf := range []string{
			filepath.Join(homeDir, ".ssh", "config"),
			filepath.Join("/", "etc", "ssh", "ssh_config"),
		} {
			if utils.FileExists(cf) {
				configFile = cf
				break
			}
		}
	}
	if identityFile == "" {
		// try default location if not specified via GIT_SSH_COMMAND
		for _, idf := range []string{
			filepath.Join(homeDir, ".ssh", "id_rsa"),
			filepath.Join(homeDir, ".ssh", "id_ed25519"),
		} {
			if utils.FileExists(idf) {
				identityFile = idf
				break
			}
		}
	}
	return identityFile, configFile
}

// processSSHConfig extracts the relevant info from a config dile
// Git Auth
func processSSHConfig(sshCfg *ssh_config.Config, hostname string, defaultIdentityFile string) (SSHConfigAuth, error) {
	sc := SSHConfigAuth{}
	sc.User, _ = sshCfg.Get(hostname, "User")
	sc.Port, _ = sshCfg.Get(hostname, "Port")
	sc.Hostname, _ = sshCfg.Get(hostname, "Hostname")
	sc.IdentityFile, _ = sshCfg.Get(hostname, "IdentityFile")

	if sc.Port == "" {
		sc.Port = ssh_config.Default("Port")
	}
	if sc.User == "" {
		sc.User = "git"
	}
	if sc.Hostname == "" {
		sc.Hostname = hostname
	}
	if sc.IdentityFile == "" {
		if defaultIdentityFile == "" {
			return sc, fmt.Errorf("%w\nfailed to identify a default identity file for host", ErrGitOperation)
		}
		sc.IdentityFile = defaultIdentityFile
	}
	return sc, nil
}

func (gs *GitSource) getGitSSHAuth(host string) (*gitssh.PublicKeys, error) {
	identityFile, cfgFile := getSshConfigFile()
	if identityFile == "" && cfgFile == "" {
		return nil, fmt.Errorf("%w\nneither default identity files nor a ssh_config were found at the desired locations", ErrGitOperation)
	}

	cfg := &ssh_config.Config{}
	if cfgFile != "" {
		f, err := os.Open(cfgFile)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		cfg, err = ssh_config.Decode(f)
		if err != nil {
			return nil, err
		}
	}

	sc, err := processSSHConfig(cfg, host, identityFile)
	if err != nil {
		return nil, err
	}
	sc.ConfigFile = cfgFile
	gs.SshConfig = sc

	key, err := os.ReadFile(utils.NormalizeHome(sc.IdentityFile))
	if err != nil {
		return nil, fmt.Errorf("%w\nfailed to read identityFile: %v", ErrGitOperation, err)
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("%w\nfailed to parse identityFile: %v", ErrGitOperation, err)
	}
	return &gitssh.PublicKeys{
		User:   sc.User,
		Signer: signer,
		HostKeyCallbackHelper: gitssh.HostKeyCallbackHelper{
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		},
	}, nil
}
