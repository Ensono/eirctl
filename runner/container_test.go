package runner_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"

	"github.com/Ensono/eirctl/runner"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type mockContainerClient struct {
	close func() error
	pull  func() (io.ReadCloser, error)
}

func (mc mockContainerClient) Close() error {
	return mc.close()
}

func (mc mockContainerClient) ImagePull(ctx context.Context, refStr string, options image.PullOptions) (io.ReadCloser, error) {
	return mc.pull()
}

func (mc mockContainerClient) ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error) {
	return container.CreateResponse{}, nil
}

func (mc mockContainerClient) ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error {
	return nil
}

func (mc mockContainerClient) ContainerWait(ctx context.Context, containerID string, condition container.WaitCondition) (<-chan container.WaitResponse, <-chan error) {
	return nil, nil

}
func (mc mockContainerClient) ContainerLogs(ctx context.Context, containerID string, options container.LogsOptions) (io.ReadCloser, error) {
	return nil, nil
}

func (mc mockContainerClient) ContainerInspect(ctx context.Context, containerID string) (container.InspectResponse, error) {
	return container.InspectResponse{}, nil
}

type mockReaderCloser struct {
	io.Reader
}

func (m mockReaderCloser) Close() error {
	return nil
}

// ErrUnauthorized
type mockUnauthorizedErr struct{ message string }

func (mu mockUnauthorizedErr) Error() string {
	return mu.message
}

func (mu mockUnauthorizedErr) Unauthorized() {

}

func Test_ImagePull(t *testing.T) {
	t.Parallel()
	t.Run("no auth required authFunc", func(t *testing.T) {
		cc := runner.NewContainerContext("docker.io/alpine:3.21.3")
		mcc := mockContainerClient{
			pull: func() (io.ReadCloser, error) {
				mr := mockReaderCloser{bytes.NewReader([]byte(`done`))}
				return mr, nil
			},
		}
		execContext := runner.NewExecutionContext(nil, "", nil, nil, []string{}, []string{}, []string{}, []string{},
			runner.WithContainerOpts(cc))

		nce, err := runner.NewContainerExecutor(execContext, runner.WithContainerClient(mcc))
		if err != nil {
			t.Fatal(err)
		}

		if err := nce.PullImage(context.TODO(), cc.Image, io.Discard); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("use private registry - authFunc run", func(t *testing.T) {
		// originalEnv := os.Environ()
		tmpRegFile, _ := os.CreateTemp(os.TempDir(), "auth-*")
		if err := os.WriteFile(tmpRegFile.Name(), []byte(`{"auths":{"private.io":{"auth":"dXNlcm5hbWU6cGFzc3dvcmQxCg=="}}}`), 0777); err != nil {
			t.Fatal(err)
		}
		os.Setenv(runner.REGISTRY_AUTH_FILE, tmpRegFile.Name())

		defer os.Unsetenv(runner.REGISTRY_AUTH_FILE)
		defer os.Remove(tmpRegFile.Name())

		gotFn := runner.AuthLookupFunc("private.io/alpine:3.21.3")
		got, err := gotFn(context.TODO())
		if err != nil {
			t.Fatal(err)
		}
		if got == "" {
			t.Error("got '', wanted a token")
		}
	})

	t.Run("use private registry idtoken - authFunc run", func(t *testing.T) {
		// originalEnv := os.Environ()
		tmpRegFile, _ := os.CreateTemp(os.TempDir(), "auth-*")
		if err := os.WriteFile(tmpRegFile.Name(), []byte(`{"auths":{"private.io":{"auth":"eyJwYXlsb2FkIjoic29tZXRvaXViaGdmZHM/RERmZHN1amJmZy9kc2ZnZCIsInZlcnNpb24iOiIyIiwiZXhwaXJhdGlvbiI6MTc0MjI4MjE2Nn0K"}}}`), 0777); err != nil {
			t.Fatal(err)
		}
		os.Setenv(runner.REGISTRY_AUTH_FILE, tmpRegFile.Name())

		defer os.Unsetenv(runner.REGISTRY_AUTH_FILE)
		defer os.Remove(tmpRegFile.Name())

		gotFn := runner.AuthLookupFunc("private.io/alpine:3.21.3")
		got, err := gotFn(context.TODO())
		if err != nil {
			t.Fatal(err)
		}
		if got == "" {
			t.Error("got '', wanted a token")
		}
	})

	t.Run("REGISTRY_AUTH_FILE not set - authFunc run", func(t *testing.T) {
		tmpRegFile, _ := os.CreateTemp(os.TempDir(), "auth-*")
		if err := os.WriteFile(tmpRegFile.Name(), []byte(`{"auths":{"private.io":{"auth":"dXNlcm5hbWU6cGFzc3dvcmQxCg=="}}}`), 0777); err != nil {
			t.Fatal(err)
		}

		defer os.Remove(tmpRegFile.Name())

		gotFn := runner.AuthLookupFunc("private.io/alpine:3.21.3")
		got, err := gotFn(context.TODO())
		if err != nil {
			t.Fatal(err)
		}
		if got != "" {
			t.Errorf("got '%s', wanted a `''`", got)
		}
	})
}
