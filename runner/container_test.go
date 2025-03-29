package runner_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"

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
	close  func() error
	pull   func() (io.ReadCloser, error)
	create func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error)
	start  func(ctx context.Context, containerID string, options container.StartOptions) error
	attach func(ctx context.Context, container string, options container.AttachOptions) (types.HijackedResponse, error)
	wait   func(ctx context.Context, containerID string, condition container.WaitCondition) (<-chan container.WaitResponse, <-chan error)
}

func (mc mockContainerClient) Close() error {
	return mc.close()
}

func (mc mockContainerClient) ImagePull(ctx context.Context, refStr string, options image.PullOptions) (io.ReadCloser, error) {
	return mc.pull()
}

func (mc mockContainerClient) ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error) {
	return mc.create(ctx, config, hostConfig, networkingConfig, platform, containerName)
}

func (mc mockContainerClient) ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error {
	return mc.start(ctx, containerID, options)
}

func (mc mockContainerClient) ContainerWait(ctx context.Context, containerID string, condition container.WaitCondition) (<-chan container.WaitResponse, <-chan error) {
	return mc.wait(ctx, containerID, condition)

}
func (mc mockContainerClient) ContainerLogs(ctx context.Context, containerID string, options container.LogsOptions) (io.ReadCloser, error) {
	return nil, nil
}

func (mc mockContainerClient) ContainerInspect(ctx context.Context, containerID string) (container.InspectResponse, error) {
	return container.InspectResponse{}, nil
}

func (mc mockContainerClient) ContainerAttach(ctx context.Context, container string, options container.AttachOptions) (types.HijackedResponse, error) {
	return mc.attach(ctx, container, options)
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

type MockConn struct {
	reader *io.PipeReader
	writer *io.PipeWriter
	logBuf *bytes.Buffer

	Closed bool
}

func NewMockConn() *MockConn {
	r, w := io.Pipe()
	return &MockConn{
		reader: r,
		writer: w,
		logBuf: new(bytes.Buffer),
	}
}

func (m *MockConn) Read(b []byte) (int, error) {
	return m.reader.Read(b)
}

func (m *MockConn) Write(b []byte) (int, error) {
	n, err := m.writer.Write(b)
	// Optionally log/write to buffer
	m.logBuf.Write(b)
	return n, err
}

func (m *MockConn) Close() error {
	_ = m.reader.Close()
	_ = m.writer.Close()
	return nil
}

func (m *MockConn) LocalAddr() net.Addr {
	return &net.IPAddr{IP: net.ParseIP("127.0.0.1")}
}

func (m *MockConn) RemoteAddr() net.Addr {
	return &net.IPAddr{IP: net.ParseIP("127.0.0.2")}
}

func (m *MockConn) SetDeadline(t time.Time) error      { return nil }
func (m *MockConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *MockConn) SetWriteDeadline(t time.Time) error { return nil }

func Test_Execute_shell(t *testing.T) {

	t.Run("correctly gets output", func(t *testing.T) {

		configContext := &runner.ExecutionContext{
			Env: variables.FromMap(map[string]string{"FOO": "bar"}),
		}

		containerOpt := runner.NewContainerContext("alpine:3.21.3")
		containerOpt.ShellArgs = []string{"sh"}
		containerOpt.VolumesFromArgs([]string{"-v ${PWD}:/testdir"})
		conn := NewMockConn()

		stdin := bytes.NewBufferString("")
		stdout := output.NewSafeWriter(new(bytes.Buffer))
		stderr := output.NewSafeWriter(&bytes.Buffer{})

		respCh := make(chan container.WaitResponse)
		errCh := make(chan error)

		mcc := mockContainerClient{
			pull: func() (io.ReadCloser, error) {
				mr := mockReaderCloser{bytes.NewReader([]byte(`done`))}
				return mr, nil
			},
			create: func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error) {
				return container.CreateResponse{ID: "12354"}, nil
			},
			attach: func(ctx context.Context, container string, options container.AttachOptions) (types.HijackedResponse, error) {
				return types.NewHijackedResponse(conn, ""), nil
			},
			start: func(ctx context.Context, containerID string, options container.StartOptions) error {
				return nil
			},
			wait: func(ctx context.Context, containerID string, condition container.WaitCondition) (<-chan container.WaitResponse, <-chan error) {
				return respCh, errCh

			},
			close: func() error {
				return nil
			},
		}

		// containerOpt
		execContext := runner.NewExecutionContext(nil, configContext.Dir, configContext.Env, configContext.Envfile,
			[]string{}, []string{}, []string{}, []string{},
			runner.WithContainerOpts(containerOpt))

		ce, err := runner.NewContainerExecutor(execContext, runner.WithContainerClient(mcc))
		if err != nil {
			t.Fatal(err)
		}

		ce.Term = &mockTerminal{returnMakeRawErr: nil}

		go func() {
			_, _ = conn.Write([]byte("hello\n"))
			time.Sleep(500 * time.Millisecond)
			respCh <- container.WaitResponse{Error: nil, StatusCode: 0}
		}()

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
			t.Errorf("got: %q, wanted output to contain 'hello'", output)
		}
	})
}
