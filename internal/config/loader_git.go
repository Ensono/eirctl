package config

import (
	"errors"
	"flag"
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
	GitSshPassphrase       = "GIT_SSH_PASSPHRASE"
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
	SshConfig *SSHConfigAuth
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

// NewGitSource converts an import string of type git
// to a parseable git clone and checkout object
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

func SSHKeySigner(key []byte) (ssh.Signer, error) {
	if passphrase, found := os.LookupEnv(GitSshPassphrase); found {
		signer, err := ssh.ParsePrivateKeyWithPassphrase(key, []byte(passphrase))
		if err != nil {
			return nil, fmt.Errorf("%w\nfailed to parse identityFile with passprhase: %v", ErrGitOperation, err)
		}
		return signer, nil
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("%w\nfailed to parse identityFile: %v", ErrGitOperation, err)
	}

	return signer, nil
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

func (gs *GitSource) getGitSSHAuth(host string) (*gitssh.PublicKeys, error) {

	sshDefaultConf := parseDefaultSshConfigFilePaths()
	// values supplied via the GIT_SSH_COMMAND have global precedence
	sshConf := parseGitSshCommandEnv()
	// IF not specified via ENV overrides which have higher priority - fallback to default paths
	if sshConf.ConfigFile == "" {
		sshConf.ConfigFile = sshDefaultConf.ConfigFile
	}

	sshCfgFile := &ssh_config.Config{}
	if sshConf.ConfigFile != "" {
		f, err := os.Open(sshConf.ConfigFile)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		sshCfgFile, err = ssh_config.Decode(f)
		if err != nil {
			return nil, err
		}
	}

	if err := processSSHConfig(sshCfgFile, sshConf, host); err != nil {
		return nil, err
	}

	if sshConf.IdentityFile == "" {
		sshConf.IdentityFile = sshDefaultConf.IdentityFile
		if sshConf.IdentityFile == "" {
			return nil, fmt.Errorf("%w\nfailed to identify a default identity file for host (%s)", ErrGitOperation, host)
		}
	}

	gs.SshConfig = sshConf

	key, err := os.ReadFile(utils.NormalizeHome(sshConf.IdentityFile))
	if err != nil {
		return nil, fmt.Errorf("%w\nfailed to read identityFile: %v", ErrGitOperation, err)
	}

	signer, err := SSHKeySigner(key)

	if err != nil {
		return nil, err
	}

	return &gitssh.PublicKeys{
		User:   sshDefaultConf.User,
		Signer: signer,
		HostKeyCallbackHelper: gitssh.HostKeyCallbackHelper{
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		},
	}, nil
}

// parseGitSshCommandEnv looks for the conventional GIT_SSH_COMMAND variable
// if set it will use the values for identity and/or config file
func parseGitSshCommandEnv() *SSHConfigAuth {
	sshConf := &SSHConfigAuth{}
	gsc := os.Getenv(GitSshCommandVar)
	args := strings.Fields(gsc)
	if len(args) < 1 {
		return sshConf
	}
	// use posix flag package for these flags
	pflagSet := pflag.NewFlagSet("gitsshcommand", pflag.ContinueOnError)
	_ = pflagSet.StringP("identity", "i", "", "identity file - i.e. the private key to use")
	_ = pflagSet.StringP("file", "F", "", "config file override")

	if err := pflagSet.Parse(args); err != nil {
		logrus.Debugf("%s: %s\nerror: %v", GitSshCommandVar, gsc, err)
	}
	sshConf.IdentityFile, _ = pflagSet.GetString("identity")
	sshConf.ConfigFile, _ = pflagSet.GetString("file")

	// use default flag package to parse these flags
	flagSet := flag.NewFlagSet("nativeSSHGitCommand", flag.ContinueOnError)
	// hostname := flag.String("oHostname", "", "")
	hostname := flagSet.String("oHostname", "", "global hostname overrride")
	port := flagSet.String("oPort", "", "global port overrride")

	if err := flagSet.Parse(args[1:]); err != nil {
		logrus.Debugf("%s: %s\nnative flag parse error: %v", GitSshCommandVar, gsc, err)
	}

	sshConf.Hostname = *hostname
	sshConf.Port = *port

	return sshConf
}

// parseDefaultSshConfigFilePaths parses GIT_SSH_COMMAND and sets the default paths for:
//
//	configFile
//	identityFile
//
// if none were provided
func parseDefaultSshConfigFilePaths() *SSHConfigAuth {
	defaultFilePaths := &SSHConfigAuth{}
	homeDir := filepath.Join(utils.MustGetUserHomeDir())
	// try default location if not specified via GIT_SSH_COMMAND
	if defaultFilePaths.ConfigFile == "" {
		for _, cf := range []string{
			filepath.Join(homeDir, ".ssh", "config"),
			filepath.Join("/", "etc", "ssh", "ssh_config"),
		} {
			if utils.FileExists(cf) {
				defaultFilePaths.ConfigFile = cf
				break
			}
		}
	}
	// try default location if not specified via GIT_SSH_COMMAND
	if defaultFilePaths.IdentityFile == "" {
		for _, idf := range []string{
			filepath.Join(homeDir, ".ssh", "id_rsa"),
			filepath.Join(homeDir, ".ssh", "id_ed25519"),
		} {
			if utils.FileExists(idf) {
				defaultFilePaths.IdentityFile = idf
				break
			}
		}
	}
	return defaultFilePaths
}

// processSSHConfig extracts the relevant info from a config file, merging with
func processSSHConfig(fileSSHCfg *ssh_config.Config, sshConfig *SSHConfigAuth, hostname string) error {

	if sshConfig.Port == "" {
		filePort, _ := fileSSHCfg.Get(hostname, "Port")
		sshConfig.Port = filePort
		if sshConfig.Port == "" {
			sshConfig.Port = "22"
		}
	}
	if sshConfig.User == "" {
		fileUser, _ := fileSSHCfg.Get(hostname, "User")
		sshConfig.User = fileUser
		if sshConfig.User == "" {
			sshConfig.User = "git"
		}
	}

	if sshConfig.Hostname == "" {
		fileHostname, _ := fileSSHCfg.Get(hostname, "Hostname")
		sshConfig.Hostname = fileHostname
		if sshConfig.Hostname == "" {
			sshConfig.Hostname = hostname
		}
	}

	if sshConfig.IdentityFile == "" {
		fileIdentityFile, _ := fileSSHCfg.Get(hostname, "IdentityFile")
		sshConfig.IdentityFile = fileIdentityFile
	}
	return nil
}
