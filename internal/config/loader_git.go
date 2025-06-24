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
		Depth:      1,
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

func (gs *GitSource) getCommit(r *git.Repository) (*object.Commit, error) {
	// If tag or branch was specified, check out the correct commit
	if gs.tag != "" {
		// Try as branch first
		// currently hardcoding to "origin"
		// but it shouldn't cause an issuse as the in-memory clone is controlled
		ref, err := r.Reference(plumbing.NewRemoteReferenceName("origin", gs.tag), true)
		if err != nil {
			// Fallback to tag
			ref, err = r.Reference(plumbing.NewTagReferenceName(gs.tag), true)
			if err != nil {
				// Try as revision (commit or tag/branch fallback)
				rev, err := r.ResolveRevision(plumbing.Revision(gs.tag))
				if err != nil {
					return nil, fmt.Errorf("%w, gone through all fallbacks.", ErrGitTagBranchRevisionWrong)
				}
				return r.CommitObject(plumbing.NewHashReference("", *rev).Hash())
			}
			// ref exists based on tag - let's make sure it's not a complex tag with an annotation and hash reference
			return resolveToCommit(r, ref.Hash())
			// if tag, err := r.TagObject(ref.Hash()); err == nil {
			// 	// de-reference the commit from tag
			// 	return tag.Commit()
			// }
		}
		return r.CommitObject(ref.Hash())
	}
	// use default current head
	// Get file content from HEAD commit - as a default commit
	ref, err := r.Head()
	if err != nil {
		return nil, fmt.Errorf("get HEAD: %w", err)
	}
	commit, err := r.CommitObject(ref.Hash())
	if err != nil {
		return nil, fmt.Errorf("get HEAD commit: %w", err)
	}

	return commit, nil
}

func resolveToCommit(repo *git.Repository, hash plumbing.Hash) (*object.Commit, error) {
	// Try directly as a commit
	if commit, err := repo.CommitObject(hash); err == nil {
		return commit, nil
	}

	// Try as annotated tag
	tagObj, err := repo.TagObject(hash)
	if err != nil {
		return nil, fmt.Errorf("not a commit or tag: %w", err)
	}

	// Recursively resolve the tag's target
	target := tagObj.Target
	switch tagObj.TargetType {
	case plumbing.CommitObject:
		return repo.CommitObject(target)
	case plumbing.TagObject:
		// Nested tag (tag of tag): recurse
		return resolveToCommit(repo, target)
	default:
		return nil, fmt.Errorf("unsupported target type: %v", tagObj.TargetType)
	}
}
