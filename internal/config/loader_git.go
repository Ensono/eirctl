package config

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/Ensono/eirctl/internal/schema"
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
	"golang.org/x/crypto/ssh/knownhosts"
	"gopkg.in/yaml.v3"
	"mvdan.cc/sh/v3/shell"
)

const (
	gitPrefix              = "git::"
	sshGitConnectionString = "ssh://%s@%s:%s/%s" // user@host:port/org/repo
	remoteRef              = "origin"
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
	SshConfig *SSHConfigAuth
	repo      *git.Repository
	gcOpts    *git.CloneOptions
	yamlPath  string
	tag       string
	// entry is the file or directory options struct to check out and store as a file or parse as a ConfigDefinition
	entry schema.ImportEntry
}

type SSHConfigAuth struct {
	IdentityFile string
	ConfigFile   string
	User         string
	Port         string
	Hostname     string
	// The legacy string fields preserve API compatibility. Effective selections
	// are stored as ordered slices so quoted paths and repeated directives retain
	// their original boundaries and precedence.
	UserKnownHostsFile    string
	SystemKnownHostsFile  string
	UserKnownHostsFiles   []string
	SystemKnownHostsFiles []string
	StrictHostKeyChecking string
}

func IsGit(raw string) bool {
	return strings.HasPrefix(raw, gitPrefix)
}

// NewGitSource converts an import string of type git
// to a parseable git clone and checkout object
func NewGitSource(entry schema.ImportEntry) (*GitSource, error) {
	gs := &GitSource{
		gcOpts: &git.CloneOptions{
			// specifically set this here so that later it is a known value
			RemoteName: remoteRef,
			Depth:      0,
		},
		entry: entry,
	}

	gitImportParts := gitRegexp.FindStringSubmatch(entry.Src)

	logrus.Tracef("loader_git.NewGitSource: Git Import Parts: %+v", gitImportParts)

	if len(gitImportParts) != 5 {
		return gs, fmt.Errorf("import %s, %w", entry.Src, ErrIncorrectlyFormattedGit)
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

func (gs *GitSource) File() (io.ReadCloser, error) {
	tree, err := gs.tree()
	if err != nil {
		return nil, fmt.Errorf("%w\nError: %v", ErrGitOperation, err)
	}

	logrus.Trace("loader_git.File: tree.File")
	file, err := tree.File(gs.yamlPath)
	if err != nil {
		return nil, fmt.Errorf("%w\nError: %v", ErrGitOperation, err)
	}

	logrus.Trace("loader_git.File: file.Reader")
	contents, err := file.Reader()
	if err != nil {
		return nil, fmt.Errorf("%w\nFailed to read: %v", ErrGitOperation, err)
	}
	return contents, nil
}

func (gs *GitSource) Config() (*ConfigDefinition, error) {

	contents, err := gs.File()
	if err != nil {
		return nil, err
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
			tryRemote := fmt.Sprintf("refs/remotes/%s/%s", remoteRef, tag)
			logrus.Debugf("Failed to resolve '%s', trying, '%s'", tag, tryRemote)

			rev, err = r.ResolveRevision(plumbing.Revision(tryRemote))

			if err != nil {
				// TODO: This error never gets surfaced as it's ignored in `getCommit` below..?
				return nil, fmt.Errorf("%w, gone through all fallbacks", ErrGitTagBranchRevisionWrong)
			}
		}

		return r.CommitObject(*rev)
	},
	// TODO: Validate if this is needed
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

// tree grabs hold of a tree at the relevant commit_sha/branch/tag
//
// Abstracted away for future use of directory walking or plain file retrieval
func (gs *GitSource) tree() (*object.Tree, error) {
	logrus.Trace("loader_git.File: gs.getCommit")
	commit, err := gs.getCommit(gs.repo)
	if err != nil {
		return nil, fmt.Errorf("%w\nError: %v", ErrGitOperation, err)
	}

	logrus.Trace("loader_git.File: commit.Tree")
	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("%w\nFailed to get git tree: %v", ErrGitOperation, err)
	}
	return tree, nil
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

	hostKeyCallback, err := hostKeyCallback(sshConf)
	if err != nil {
		return nil, err
	}

	return &gitssh.PublicKeys{
		User:   sshConf.User,
		Signer: signer,
		HostKeyCallbackHelper: gitssh.HostKeyCallbackHelper{
			HostKeyCallback: hostKeyCallback,
		},
	}, nil
}

// hostKeyCallback verifies SSH server identity using configured OpenSSH known-host files.
// StrictHostKeyChecking=no is the sole compatibility opt-out and is deliberately noisy.
func hostKeyCallback(sshConf *SSHConfigAuth) (ssh.HostKeyCallback, error) {
	if strings.EqualFold(sshConf.StrictHostKeyChecking, "no") {
		logrus.Warn("SSH host-key verification is disabled by StrictHostKeyChecking=no")
		return ssh.InsecureIgnoreHostKey(), nil
	}

	if err := validateConfiguredKnownHostsFiles(sshConf); err != nil {
		return nil, err
	}
	paths := knownHostsFiles(sshConf)
	if len(paths) == 0 {
		return nil, fmt.Errorf("%w\nno readable known-hosts file is available for %s; configure UserKnownHostsFile or add the host key", ErrGitOperation, sshConf.Hostname)
	}
	callback, err := knownhosts.New(paths...)
	if err != nil {
		return nil, fmt.Errorf("%w\nfailed to load known-hosts files for %s: %v", ErrGitOperation, sshConf.Hostname, err)
	}
	effectivePort := sshConf.Port
	if effectivePort == "" {
		effectivePort = "22"
	}
	effectiveAddress := net.JoinHostPort(sshConf.Hostname, effectivePort)
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		if err := callback(hostname, remote, key); err != nil {
			return fmt.Errorf("%w: SSH host-key verification failed for %s; verify the effective host and port in the configured known-hosts file", err, effectiveAddress)
		}
		return nil
	}, nil
}

func validateConfiguredKnownHostsFiles(sshConf *SSHConfigAuth) error {
	for _, configured := range []struct {
		label string
		paths []string
	}{
		{"UserKnownHostsFile", configuredKnownHosts(sshConf.UserKnownHostsFiles, sshConf.UserKnownHostsFile)},
		{"GlobalKnownHostsFile", configuredKnownHosts(sshConf.SystemKnownHostsFiles, sshConf.SystemKnownHostsFile)},
	} {
		for _, candidate := range configured.paths {
			path := filepath.Clean(utils.NormalizeHome(candidate))
			info, err := os.Stat(path)
			if err != nil || info.IsDir() {
				return fmt.Errorf("%w\nconfigured %s is not a readable known-hosts file: %s", ErrGitOperation, configured.label, path)
			}
		}
	}
	return nil
}

func configuredKnownHosts(paths []string, legacy string) []string {
	if len(paths) > 0 {
		return paths
	}
	if legacy == "" {
		return nil
	}
	fields, err := shell.Fields(legacy, nil)
	if err != nil {
		return []string{legacy}
	}
	return fields
}

var platformDefaultKnownHostsFiles = defaultPlatformKnownHostsFiles

func defaultPlatformKnownHostsFiles() []string {
	if runtime.GOOS == "windows" {
		programData := os.Getenv("ProgramData")
		if programData == "" {
			programData = `C:\\ProgramData`
		}
		return []string{filepath.Join(programData, "ssh", "ssh_known_hosts")}
	}
	return []string{"/etc/ssh/ssh_known_hosts", "/etc/ssh/ssh_known_hosts2"}
}

func knownHostsFiles(sshConf *SSHConfigAuth) []string {
	var candidates []string
	if configured := configuredKnownHosts(sshConf.UserKnownHostsFiles, sshConf.UserKnownHostsFile); len(configured) > 0 {
		candidates = append(candidates, configured...)
	} else {
		homeDir := utils.MustGetUserHomeDir()
		candidates = append(candidates,
			filepath.Join(homeDir, ".ssh", "known_hosts"),
			filepath.Join(homeDir, ".ssh", "known_hosts2"),
		)
	}
	if configured := configuredKnownHosts(sshConf.SystemKnownHostsFiles, sshConf.SystemKnownHostsFile); len(configured) > 0 {
		candidates = append(candidates, configured...)
	} else {
		candidates = append(candidates, platformDefaultKnownHostsFiles()...)
	}

	files := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		candidate = filepath.Clean(utils.NormalizeHome(candidate))
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			files = append(files, candidate)
		}
	}
	return files
}

// parseGitSshCommandEnv looks for the conventional GIT_SSH_COMMAND variable
// if set it will use the values for identity and/or config file
func parseGitSshCommandEnv() *SSHConfigAuth {
	sshConf := &SSHConfigAuth{}
	gsc := os.Getenv(GitSshCommandVar)
	args, err := shell.Fields(gsc, nil)
	if err != nil {
		logrus.Debugf("unable to parse %s: %v", GitSshCommandVar, err)
		return sshConf
	}
	if len(args) < 1 {
		return sshConf
	}
	// use posix flag package for these flags
	pflagSet := pflag.NewFlagSet("gitsshcommand", pflag.ContinueOnError)
	identity := pflagSet.StringP("identity", "i", "", "identity file - i.e. the private key to use")
	confFile := pflagSet.StringP("file", "F", "", "config file override")
	optionParam := pflagSet.StringToStringP("", "o", nil, "options parameter")

	_ = pflagSet.Parse(args)

	sshConf.IdentityFile = *identity
	sshConf.ConfigFile = *confFile

	for key, v := range *optionParam {
		// Keep this as switch in case we want to introduce additional parameters from the `-oParam=Val` option set in SSH.
		switch strings.ToLower(key) {
		case "hostname":
			sshConf.Hostname = v
		case "port":
			sshConf.Port = v
		case "userknownhostsfile":
			sshConf.UserKnownHostsFile = v
			sshConf.UserKnownHostsFiles = append(sshConf.UserKnownHostsFiles, v)
		case "systemknownhostsfile":
			sshConf.SystemKnownHostsFile = v
			sshConf.SystemKnownHostsFiles = append(sshConf.SystemKnownHostsFiles, v)
		case "stricthostkeychecking":
			sshConf.StrictHostKeyChecking = v
		default:
			logrus.Debugf("option: %s, currently not supported with GIT_SSH_COMMAND", key)
		}
	}
	// pflag's StringToString map retains only the last repeated -o option. Keep
	// all known-host directives in command order for OpenSSH-compatible trust
	// file precedence.
	sshConf.UserKnownHostsFiles = sshOptionValues(args, "userknownhostsfile")
	if len(sshConf.UserKnownHostsFiles) > 0 {
		sshConf.UserKnownHostsFile = sshConf.UserKnownHostsFiles[0]
	}
	sshConf.SystemKnownHostsFiles = sshOptionValues(args, "systemknownhostsfile")
	if len(sshConf.SystemKnownHostsFiles) > 0 {
		sshConf.SystemKnownHostsFile = sshConf.SystemKnownHostsFiles[0]
	}

	return sshConf
}

func sshOptionValues(args []string, option string) []string {
	var values []string
	for index := 0; index < len(args); index++ {
		argument := args[index]
		if argument == "-o" {
			index++
			if index >= len(args) {
				break
			}
			argument = args[index]
		} else if strings.HasPrefix(argument, "-o") {
			argument = strings.TrimPrefix(argument, "-o")
		} else {
			continue
		}
		key, value, found := strings.Cut(argument, "=")
		if found && strings.EqualFold(key, option) {
			values = append(values, value)
		}
	}
	return values
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
	if len(sshConfig.UserKnownHostsFiles) == 0 && sshConfig.UserKnownHostsFile == "" {
		sshConfig.UserKnownHostsFiles, _ = fileSSHCfg.GetAll(hostname, "UserKnownHostsFile")
		if len(sshConfig.UserKnownHostsFiles) > 0 {
			sshConfig.UserKnownHostsFile = sshConfig.UserKnownHostsFiles[0]
		}
	}
	if len(sshConfig.SystemKnownHostsFiles) == 0 && sshConfig.SystemKnownHostsFile == "" {
		sshConfig.SystemKnownHostsFiles, _ = fileSSHCfg.GetAll(hostname, "GlobalKnownHostsFile")
		if len(sshConfig.SystemKnownHostsFiles) > 0 {
			sshConfig.SystemKnownHostsFile = sshConfig.SystemKnownHostsFiles[0]
		}
	}
	if sshConfig.StrictHostKeyChecking == "" {
		sshConfig.StrictHostKeyChecking, _ = fileSSHCfg.Get(hostname, "StrictHostKeyChecking")
	}
	return nil
}
