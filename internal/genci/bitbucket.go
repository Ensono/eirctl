package genci

import (
	"github.com/Ensono/eirctl/internal/config"
	"github.com/Ensono/eirctl/scheduler"
)

type bitbucketCIImpl struct {
}

func newBitbucketCIImpl(conf *config.Config) (*bitbucketCIImpl, error) {
	impl := &bitbucketCIImpl{}
	if conf.Generate != nil && conf.Generate.Version != "" {
		// impl.eirctlVersion = conf.Generate.Version
	}
	return impl, nil
}

func (impl *bitbucketCIImpl) Convert(pipeline *scheduler.ExecutionGraph) ([]byte, error) {
	return []byte{}, nil
}
