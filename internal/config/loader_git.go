package config

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

const (
	gitPrefix           = "git::"
	gitPathSeparator    = "//"
	gitConnectionString = "git@%s:%s"
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
	// git storage for testing
	repo           *git.Repository
	gitCheckoutStr string
	yamlPath       string
	tag            string
}

func IsGit(raw string) bool {
	return strings.HasPrefix(raw, gitPrefix)
}

func NewGitSource(raw string) (*GitSource, error) {
	gs := &GitSource{}
	logrus.Debugf("git path: %s", raw)
	gitImportParts := gitRegexp.FindStringSubmatch(raw)
	if len(gitImportParts) != 5 {
		return gs, fmt.Errorf("import %s, %w", raw, ErrIncorrectlyFormattedGit)
	}

	switch gitImportParts[1] {
	case "ssh":
		p1 := strings.Split(gitImportParts[2], "/")
		gs.gitCheckoutStr = fmt.Sprintf(gitConnectionString, p1[0], strings.Join(p1[1:], "/"))
	case "http", "https":
		gs.gitCheckoutStr = "https://" + gitImportParts[2]
	case "file":
		gs.gitCheckoutStr = gitImportParts[2]
	default:
		return nil, fmt.Errorf("must specify a protocol (ssh|https|file)\n%w", ErrIncorrectlyFormattedGit)
	}

	gs.yamlPath = gitImportParts[3]
	gs.tag = gitImportParts[4]

	return gs, nil
}

// Clone calls
func (gs *GitSource) Clone() error {
	// default opts
	gcOpts := &git.CloneOptions{
		URL:        gs.gitCheckoutStr,
		RemoteName: "origin", // specifically set this here so that later it is a known value
		Depth:      0,
	}
	var err error
	if gs.repo, err = git.Clone(memory.NewStorage(), nil, gcOpts); err != nil {
		return err
	}
	return nil
}

func (gs *GitSource) YamlPath() string {
	return gs.yamlPath
}

func (gs *GitSource) GitCheckoutStr() string {
	return gs.gitCheckoutStr
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
