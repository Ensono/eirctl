// Package genci generates CI yaml definitions based on the
// eirctl pipeline nodes.
//
// This is a translation layer between eirctl concepts of tasks, pipelines and contexts into the world of CI tools yaml syntax.
// See a list of supported tools and overview [here](https://github.com/Ensono/eirctl/blob/master/docs/ci-generator.md).
//
//	Sample output in github
//	```yaml
//
// jobs:
//
//	```
package genci

import (
	"errors"
	"fmt"

	"github.com/Ensono/eirctl/internal/config"
	"github.com/Ensono/eirctl/scheduler"
	"gopkg.in/yaml.v3"
)

var (
	ErrImplementationNotExist   = errors.New("implementation does not exist")
	ErrFailedImplementationInit = errors.New("failed to initialise the implementation")
)

type CITarget string

const (
	GitlabCITarget    CITarget = "gitlab"
	GitHubCITarget    CITarget = "github"
	BitbucketCITarget CITarget = "bitbucket"
)

// strategy - selector
type GenCi struct {
	implTyp        CITarget
	implementation GenCiIface
	// CITargetOS sets the CI Runner node OS.
	// The default is linux
	CITargetOS string
	// CITargetArch sets the CI Runner node Architecture
	// The default is amd64
	CITargetArch string
	// conf            *config.Config
	// eirctlPipeline *scheduler.ExecutionGraph
}

type GenCiIface interface {
	Convert(pipeline *scheduler.ExecutionGraph) ([]byte, error)
}

type Opts func(*GenCi)

func New(implTyp CITarget, conf *config.Config, opts ...Opts) (*GenCi, error) {
	// Pass in the version from config or set default the current binary Version
	gci := &GenCi{
		implTyp: implTyp,
	}

	switch implTyp {
	case GitHubCITarget:
		gh, err := newGithubCiImpl(conf)
		if err != nil {
			return nil, fmt.Errorf("%w, %v", ErrFailedImplementationInit, err)
		}
		gci.implementation = gh
	// Add new cases here with their implementation
	// case GitlabCITarget:
	// 	gci.implementation = &DefualtCiImpl{}
	case BitbucketCITarget:
		bb, err := newBitbucketCIImpl(conf)
		if err != nil {
			return nil, fmt.Errorf("%w, %v", ErrFailedImplementationInit, err)
		}
		gci.implementation = bb
	default:
		return nil, fmt.Errorf("%s, %w", implTyp, ErrImplementationNotExist)
	}
	return gci, nil
}

func (g *GenCi) Convert(eirctlPipeline *scheduler.ExecutionGraph) ([]byte, error) {
	return g.implementation.Convert(eirctlPipeline)
}

type DefualtCiImpl struct{}

func (impl *DefualtCiImpl) Convert(pipeline *scheduler.ExecutionGraph) ([]byte, error) {
	return nil, nil
}

func extractGeneratorMetadata[T any](implTyp CITarget, generatorMeta map[string]any) (*T, error) {
	typ := new(T)
	if gh, found := generatorMeta[string(implTyp)]; found {
		b, err := yaml.Marshal(gh)
		if err != nil {
			return typ, err
		}
		if err := yaml.Unmarshal(b, typ); err != nil {
			return typ, err
		}
	}
	return typ, nil
}
