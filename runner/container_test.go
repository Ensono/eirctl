package runner_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/Ensono/eirctl/output"
	"github.com/Ensono/eirctl/runner"
	"github.com/Ensono/eirctl/variables"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/moby/term"
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

func (mc mockContainerClient) ContainerAttach(ctx context.Context, container string, options container.AttachOptions) (types.HijackedResponse, error) {
	return types.HijackedResponse{}, nil
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
}

func Test_ImagePull_AuthFunc(t *testing.T) {
	t.Parallel()
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

	t.Run("no auth files present", func(t *testing.T) {
		runner.DOCKER_CONFIG_FILE = "/unknown/config.json"
		runner.PODMAN_CONFIG_FILE = "/unknown/auth.json"
		gotFn := runner.AuthLookupFunc("private.io/alpine:3.21.3")
		_, err := gotFn(context.TODO())
		if err == nil {
			t.Fatalf("got %v, wanted err", err)
		}
		if !errors.Is(err, runner.ErrRegistryAuth) {
			t.Errorf("got '%v', wanted a %v", err, runner.ErrRegistryAuth)
		}
	})

	t.Run("read auth file error", func(t *testing.T) {
		runner.DOCKER_CONFIG_FILE = "/unknown/config.json"
		runner.PODMAN_CONFIG_FILE = "/unknown/auth.json"
		tmpRegFile, _ := os.CreateTemp(os.TempDir(), "auth-*")
		if err := os.WriteFile(tmpRegFile.Name(), []byte(`{"auths":{"private.io":{"auth":function(){}?}}}`), 0777); err != nil {
			t.Fatal(err)
		}
		os.Setenv(runner.REGISTRY_AUTH_FILE, tmpRegFile.Name())

		defer os.Unsetenv(runner.REGISTRY_AUTH_FILE)
		defer os.Remove(tmpRegFile.Name())

		gotFn := runner.AuthLookupFunc("private.io/alpine:3.21.3")
		_, err := gotFn(context.TODO())
		if err == nil {
			t.Fatalf("got %v, wanted err", err)
		}
		if !errors.Is(err, runner.ErrRegistryAuth) {
			t.Errorf("got '%v', wanted a %v", err, runner.ErrRegistryAuth)
		}
	})
}

type mockTerminal struct {
	makeRawCalled    bool
	restoreCalled    bool
	returnMakeRaw    *term.State
	returnMakeRawErr error
}

func (m *mockTerminal) MakeRaw(fd uintptr) (*term.State, error) {
	m.makeRawCalled = true
	return m.returnMakeRaw, m.returnMakeRawErr
}

func (m *mockTerminal) Restore(fd uintptr, state *term.State) error {
	m.restoreCalled = true
	return nil
}

func Test_Execute_shell(t *testing.T) {

	t.Run("alpine succeeds", func(t *testing.T) {

		configContext := &runner.ExecutionContext{
			Env: variables.FromMap(map[string]string{"FOO": "bar"}),
		}

		containerOpt := runner.NewContainerContext("alpine:3.21.3")
		containerOpt.ShellArgs = []string{"sh"}
		containerOpt.VolumesFromArgs([]string{"-v ${PWD}:/testdir"})

		// containerOpt
		execContext := runner.NewExecutionContext(nil, configContext.Dir, configContext.Env, configContext.Envfile,
			[]string{}, []string{}, []string{}, []string{},
			runner.WithContainerOpts(containerOpt))
		ce, err := runner.NewContainerExecutor(execContext)
		if err != nil {
			t.Fatal(err)
		}

		stdin := bytes.NewBufferString("echo hello\nexit\n")
		stdout := output.NewSafeWriter(new(bytes.Buffer))
		stderr := output.NewSafeWriter(&bytes.Buffer{})

		ce.Term = &mockTerminal{returnMakeRawErr: nil}

		if _, err := ce.Execute(context.TODO(), &runner.Job{
			Stdin:   io.NopCloser(stdin),
			Stdout:  stdout,
			Stderr:  stderr,
			Dir:     configContext.Dir,
			IsShell: true,
		}); err != nil {
			t.Fatal(err)
		}

		output := stdout.String()
		if !strings.Contains(output, "hello") {
			t.Errorf("expected output to contain 'hello', got: %q", output)
		}
	})
}
