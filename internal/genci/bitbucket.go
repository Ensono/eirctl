package genci

import (
	"github.com/Ensono/eirctl/internal/config"
	"github.com/Ensono/eirctl/scheduler"
)

type bitbucketCIImpl struct {
}

func newBitbucketCIImpl(_ *config.Config) (*bitbucketCIImpl, error) {
	impl := &bitbucketCIImpl{}
	return impl, nil
}

func (impl *bitbucketCIImpl) Convert(pipeline *scheduler.ExecutionGraph) ([]byte, error) {
	return []byte{}, nil
}
