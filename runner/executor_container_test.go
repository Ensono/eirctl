package runner_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Ensono/eirctl/output"
	"github.com/Ensono/eirctl/runner"
	"github.com/Ensono/eirctl/variables"
	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/term"
	"mvdan.cc/sh/v3/interp"
)

type mockContainerClient struct {
	close   func() error
	pull    func() (io.ReadCloser, error)
	create  func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error)
	start   func(ctx context.Context, containerID string, options container.StartOptions) error
	attach  func(ctx context.Context, container string, options container.AttachOptions) (types.HijackedResponse, error)
	wait    func(ctx context.Context, containerID string, condition container.WaitCondition) (<-chan container.WaitResponse, <-chan error)
	resize  func(ctx context.Context, containerID string, options container.ResizeOptions) error
	logs    func(ctx context.Context, containerID string, options container.LogsOptions) (io.ReadCloser, error)
	inspect func(ctx context.Context, containerID string) (container.InspectResponse, error)
	remove  func(ctx context.Context, containerID string, options container.RemoveOptions) error
	stop    func(ctx context.Context, containerID string, options container.StopOptions) error
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

func (mc mockContainerClient) ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error {
	return mc.remove(ctx, containerID, options)
}

func (mc mockContainerClient) ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error {
	return mc.stop(ctx, containerID, options)
}

func (mc mockContainerClient) ContainerWait(ctx context.Context, containerID string, condition container.WaitCondition) (<-chan container.WaitResponse, <-chan error) {
	return mc.wait(ctx, containerID, condition)
}

func (mc mockContainerClient) ContainerLogs(ctx context.Context, containerID string, options container.LogsOptions) (io.ReadCloser, error) {
	return mc.logs(ctx, containerID, options)
}

func (mc mockContainerClient) ContainerInspect(ctx context.Context, containerID string) (container.InspectResponse, error) {
	return mc.inspect(ctx, containerID)
}

func (mc mockContainerClient) ContainerAttach(ctx context.Context, container string, options container.AttachOptions) (types.HijackedResponse, error) {
	return mc.attach(ctx, container, options)
}

func (mc mockContainerClient) ContainerResize(ctx context.Context, containerID string, options container.ResizeOptions) error {
	return mc.resize(ctx, containerID, options)
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
		cc := runner.NewContainerContext("public.io/container:tag2")
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

		if err := nce.PullImage(context.TODO(), &container.Config{Image: cc.Image}); err != nil {
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
		containerConf := &container.Config{
			Image: "private.io/alpine:3.21.3",
			Env:   []string{fmt.Sprintf("%s=%s", runner.REGISTRY_AUTH_FILE, tmpRegFile.Name())}}

		defer os.Remove(tmpRegFile.Name())

		gotFn := runner.AuthLookupFunc(containerConf)
		got, err := gotFn(context.TODO())
		if err != nil {
			t.Fatal(err)
		}
		if got == "" {
			t.Error("got '', wanted a token")
		}
	})

	t.Run("DOCKER_CONFIG use private registry - authFunc run", func(t *testing.T) {
		// originalEnv := os.Environ()
		tmpRegFile, _ := os.Create(path.Join(os.TempDir(), "config.json"))
		_, err := tmpRegFile.Write([]byte(`{"auths":{"private.io":{"auth":"dXNlcm5hbWU6cGFzc3dvcmQxCg=="}}}`))
		if err != nil {
			t.Fatal(err)
		}

		containerConf := &container.Config{
			Image: "private.io/alpine:3.21.3",
			Env:   []string{fmt.Sprintf("%s=%s", runner.DOCKER_CONFIG, path.Dir(tmpRegFile.Name()))}}

		defer os.Remove(tmpRegFile.Name())

		gotFn := runner.AuthLookupFunc(containerConf)
		got, err := gotFn(context.TODO())
		if err != nil {
			t.Fatal(err)
		}
		if got == "" {
			t.Error("got '', wanted a token")
		}
	})

	t.Run("is public registry - authFunc run", func(t *testing.T) {
		// originalEnv := os.Environ()
		tmpRegFile, _ := os.CreateTemp(os.TempDir(), "auth-*")
		if err := os.WriteFile(tmpRegFile.Name(), []byte(`{"auths":{"private.io":{"auth":"dXNlcm5hbWU6cGFzc3dvcmQxCg=="}}}`), 0777); err != nil {
			t.Fatal(err)
		}

		containerConf := &container.Config{
			Image: "public.io/alpine:3.21.3",
			Env:   []string{fmt.Sprintf("%s=%s", runner.REGISTRY_AUTH_FILE, tmpRegFile.Name())}}

		defer os.Remove(tmpRegFile.Name())

		gotFn := runner.AuthLookupFunc(containerConf)
		got, err := gotFn(context.TODO())
		if err != nil {
			t.Fatal(err)
		}
		if got != "" {
			t.Errorf("got %s, wanted ''", got)
		}
	})

	t.Run("use private registry idtoken - authFunc run", func(t *testing.T) {
		// originalEnv := os.Environ()
		tmpRegFile, _ := os.CreateTemp(os.TempDir(), "auth-*")
		if err := os.WriteFile(tmpRegFile.Name(), []byte(`{"auths":{"private.io":{"auth":"eyJwYXlsb2FkIjoic29tZXRvaXViaGdmZHM/RERmZHN1amJmZy9kc2ZnZCIsInZlcnNpb24iOiIyIiwiZXhwaXJhdGlvbiI6MTc0MjI4MjE2Nn0K"}}}`), 0777); err != nil {
			t.Fatal(err)
		}

		containerConf := &container.Config{
			Image: "private.io/alpine:3.21.3",
			Env:   []string{fmt.Sprintf("%s=%s", runner.REGISTRY_AUTH_FILE, tmpRegFile.Name())}}

		defer os.Remove(tmpRegFile.Name())

		gotFn := runner.AuthLookupFunc(containerConf)
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
		containerConf := &container.Config{
			Image: "private.io/alpine:3.21.3",
			Env:   []string{}}

		defer os.Remove(tmpRegFile.Name())

		gotFn := runner.AuthLookupFunc(containerConf)
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
		runner.CONTAINER_CONFIG_FILE = "/unknown/auth.json"
		containerConf := &container.Config{
			Image: "private.io/alpine:3.21.3",
			Env:   []string{}}

		gotFn := runner.AuthLookupFunc(containerConf)
		_, err := gotFn(context.TODO())
		if err != nil {
			t.Fatalf("got %v, wanted <nil>", err)
		}
	})

	t.Run("read auth file error", func(t *testing.T) {
		runner.DOCKER_CONFIG_FILE = "/unknown/config.json"
		runner.CONTAINER_CONFIG_FILE = "/unknown/auth.json"
		tmpRegFile, _ := os.CreateTemp(os.TempDir(), "auth-*")
		if err := os.WriteFile(tmpRegFile.Name(), []byte(`{"auths":{"private.io":{"auth":function(){}?}}}`), 0777); err != nil {
			t.Fatal(err)
		}

		containerConf := &container.Config{
			Image: "private.io/alpine:3.21.3",
			Env:   []string{fmt.Sprintf("%s=%s", runner.REGISTRY_AUTH_FILE, tmpRegFile.Name())}}

		defer os.Remove(tmpRegFile.Name())

		gotFn := runner.AuthLookupFunc(containerConf)
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
	getSizeCalled    int
	getSizeFn        func(fd int) (width, height int, err error)
}

// func (m *mockTerminal) GetTerminalFd() int {
// 	return 0
// }

func (m *mockTerminal) MakeRaw(fd int) (*term.State, error) {
	m.makeRawCalled = true
	return m.returnMakeRaw, m.returnMakeRawErr
}

func (m *mockTerminal) Restore(fd int, state *term.State) error {
	m.restoreCalled = true
	return nil
}

func (m *mockTerminal) IsTerminal(fd int) bool {
	return true
}

func (m *mockTerminal) GetSize(fd int) (width, height int, err error) {
	m.getSizeCalled++
	if m.getSizeFn != nil {
		return m.getSizeFn(fd)
	}
	return 1, 1, nil
}

func (m *mockTerminal) UpdateSize() (width, height int, err error) {
	return 2, 2, nil
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

func mockContainerClientHelper(t *testing.T, respCh <-chan container.WaitResponse, errCh <-chan error, outStreamer io.Reader, conn *MockConn) mockContainerClient {
	t.Helper()
	return mockContainerClient{
		create: func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error) {
			return container.CreateResponse{ID: "created0-123"}, nil
		},
		pull: func() (io.ReadCloser, error) {
			mr := mockReaderCloser{bytes.NewReader([]byte(`done`))}
			return mr, nil
		},
		start: func(ctx context.Context, containerID string, options container.StartOptions) error {
			return nil
		},
		wait: func(ctx context.Context, containerID string, condition container.WaitCondition) (<-chan container.WaitResponse, <-chan error) {
			return respCh, errCh
		},
		logs: func(ctx context.Context, containerID string, options container.LogsOptions) (io.ReadCloser, error) {
			return io.NopCloser(outStreamer), nil
		},
		inspect: func(ctx context.Context, containerID string) (container.InspectResponse, error) {
			resp := container.InspectResponse{&container.ContainerJSONBase{}, []container.MountPoint{}, &container.Config{}, &container.NetworkSettings{}, &ocispec.Descriptor{}}
			resp.State = &container.State{ExitCode: 0}
			resp.Image = "container:foo"
			resp.Config = &container.Config{Cmd: []string{"pwd"}}
			return resp, nil
		},
		close: func() error {
			return nil
		},
		resize: func(ctx context.Context, containerID string, options container.ResizeOptions) error {
			return nil
		},
		remove: func(ctx context.Context, containerID string, options container.RemoveOptions) error {
			return nil
		},
		stop: func(ctx context.Context, containerID string, options container.StopOptions) error {
			return nil
		},
		attach: func(ctx context.Context, container string, options container.AttachOptions) (types.HijackedResponse, error) {
			if conn == nil {
				t.Fatal("must provide a conn for shell commands")
			}
			return types.NewHijackedResponse(conn, ""), nil
		},
	}
}

func mockClientHelper(t *testing.T, mcc mockContainerClient) (*runner.ContainerExecutor, *runner.ExecutionContext, func()) {
	t.Helper()
	configContext := &runner.ExecutionContext{
		Env: variables.FromMap(map[string]string{"FOO": "bar"}),
	}

	containerOpt := runner.NewContainerContext("container:foo-3.21.3")
	containerOpt.ShellArgs = []string{"sh"}
	containerOpt.ParseContainerArgs([]string{"-v ${PWD}:/testdir"})

	// containerOpt
	execContext := runner.NewExecutionContext(nil, configContext.Dir, configContext.Env, configContext.Envfile,
		[]string{}, []string{}, []string{}, []string{},
		runner.WithContainerOpts(containerOpt))

	ce, err := runner.NewContainerExecutor(execContext, runner.WithContainerClient(mcc))
	if err != nil {
		t.Fatal(err)
	}
	stdin, _ := os.CreateTemp("", "stdin-*")
	stdout, _ := os.CreateTemp("", "stdout-*")
	ce.WithTerminalUtils(runner.NewTerminalUtils(&mockTerminal{}, runner.WithCustomFD(stdin, stdout)))
	return ce, configContext, func() {
		defer os.Remove(stdin.Name())
		defer os.Remove(stdout.Name())
	}
}

func Test_ContainerExecutor_shell(t *testing.T) {
	t.Run("correctly gets output", func(t *testing.T) {
		t.Parallel()
		stdin := bytes.NewBufferString("")
		stdout := output.NewSafeWriter(new(bytes.Buffer))
		stderr := output.NewSafeWriter(&bytes.Buffer{})
		respCh := make(chan container.WaitResponse)
		errCh := make(chan error)
		conn := NewMockConn()

		mcc := mockContainerClientHelper(t, respCh, errCh, &bytes.Reader{}, conn)

		ce, configContext, cleanup := mockClientHelper(t, mcc)
		defer cleanup()

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

	t.Run("fails to start container", func(t *testing.T) {
		t.Parallel()
		stdin := bytes.NewBufferString("")
		stdout := output.NewSafeWriter(new(bytes.Buffer))
		stderr := output.NewSafeWriter(&bytes.Buffer{})
		respCh := make(chan container.WaitResponse)
		errCh := make(chan error)
		conn := NewMockConn()

		mcc := mockContainerClientHelper(t, respCh, errCh, &bytes.Reader{}, conn)
		mcc.start = func(ctx context.Context, containerID string, options container.StartOptions) error {
			return fmt.Errorf("fail")
		}

		ce, configContext, cleanup := mockClientHelper(t, mcc)
		defer cleanup()

		go func() {
			_, _ = conn.Write([]byte("hello\n"))
			time.Sleep(500 * time.Millisecond)
			respCh <- container.WaitResponse{Error: nil, StatusCode: 0}
		}()

		_, err := ce.Execute(context.TODO(), &runner.Job{
			Stdin:   io.NopCloser(stdin),
			Stdout:  stdout,
			Stderr:  stderr,
			Dir:     configContext.Dir,
			IsShell: true,
		})
		if err == nil {
			t.Fatal(err)
		}
		if !errors.Is(err, runner.ErrContainerStart) {
			t.Errorf("got wrong type of error (%v), wanted %v", err, runner.ErrContainerStart)
		}
	})

	t.Run("fails to attach container", func(t *testing.T) {
		t.Parallel()
		stdin := bytes.NewBufferString("")
		stdout := output.NewSafeWriter(new(bytes.Buffer))
		stderr := output.NewSafeWriter(&bytes.Buffer{})
		respCh := make(chan container.WaitResponse)
		errCh := make(chan error)
		conn := NewMockConn()

		mcc := mockContainerClientHelper(t, respCh, errCh, &bytes.Reader{}, conn)
		mcc.attach = func(ctx context.Context, container string, options container.AttachOptions) (types.HijackedResponse, error) {
			return types.HijackedResponse{}, fmt.Errorf("faile")
		}

		ce, configContext, cleanup := mockClientHelper(t, mcc)
		defer cleanup()

		go func() {
			_, _ = conn.Write([]byte("hello\n"))
			time.Sleep(500 * time.Millisecond)
			respCh <- container.WaitResponse{Error: nil, StatusCode: 0}
		}()

		_, err := ce.Execute(context.TODO(), &runner.Job{
			Stdin:   io.NopCloser(stdin),
			Stdout:  stdout,
			Stderr:  stderr,
			Dir:     configContext.Dir,
			IsShell: true,
		})
		if err == nil {
			t.Fatal(err)
		}
		if !errors.Is(err, runner.ErrContainerAttach) {
			t.Errorf("got wrong type of error (%v), wanted %v", err, runner.ErrContainerAttach)
		}
	})

}

type errWriter struct {
	resp int
	err  error
}

func (ew *errWriter) Write(p []byte) (int, error) {
	return ew.resp, ew.err
}

// multiplexFrame builds one Docker multiplexed frame.
// streamType: 1=stdout, 2=stderr
// payload: the raw data to send.
//
// Frame layout:  1 byte streamType | 3 bytes zero | 4 bytes BE length | payload
func multiplexFrame(streamType byte, payload []byte) []byte {
	// 8‐byte header
	header := make([]byte, 8)
	header[0] = streamType // 1 for stdout
	// header[1..3] are already zero
	binary.BigEndian.PutUint32(header[4:], uint32(len(payload)))
	return append(header, payload...)
}

type safeReaderWriter struct {
	LogsReader *io.PipeReader
	LogsWriter *io.PipeWriter
	mu         sync.Mutex
}

func (s *safeReaderWriter) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.LogsWriter.Write(p)
}

func Test_ContainerExecutor_Execute(t *testing.T) {
	t.Run("succeeds with image in cache", func(t *testing.T) {
		t.Parallel()
		respCh := make(chan container.WaitResponse)
		errCh := make(chan error)
		pr, pw := io.Pipe()
		outStreamer := &safeReaderWriter{mu: sync.Mutex{}, LogsReader: pr, LogsWriter: pw}

		cmdOut := []string{"/eirctl", "hello, iteration 1", "hello, iteration 2", "hello, iteration 3", "hello, iteration 4", "hello, iteration 5", "hello, iteration 6", "hello, iteration 7", "hello, iteration 8", "hello, iteration 9", "hello, iteration 10"}

		mcc := mockContainerClientHelper(t, respCh, errCh, outStreamer.LogsReader, nil)

		ce, _, cleanup := mockClientHelper(t, mcc)
		defer cleanup()

		so, se := output.NewSafeWriter(&bytes.Buffer{}), output.NewSafeWriter(&bytes.Buffer{})
		go func() {
			time.Sleep(500 * time.Millisecond)
			respCh <- container.WaitResponse{Error: nil, StatusCode: 0}
		}()
		go func() {
			for _, v := range cmdOut {
				outStreamer.Write(multiplexFrame(1, fmt.Append([]byte(v), "\n")))
			}
			outStreamer.Write([]byte(`\r\n`))
		}()

		_, err := ce.Execute(context.TODO(), &runner.Job{Command: `pwd
for i in $(seq 1 10); do echo "hello, iteration $i"; done`,
			Env:    variables.NewVariables(),
			Vars:   variables.NewVariables(),
			Stdout: so,
			Stderr: se,
			Stdin:  nil,
		})
		if err != nil {
			t.Fatal(err)
		}

		if len(se.String()) > 0 {
			t.Errorf("got error %v, expected nil\n\n", se.String())
		}
		if len(so.String()) == 0 {
			t.Errorf("got (%s) no output, expected stdout\n\n", so.String())
		}
		want := fmt.Sprintf("%s\n", strings.Join(cmdOut, "\n"))
		if so.String() != want {
			t.Errorf("outputs do not match\n\tgot: %s\n\twanted:  %s", so.String(), want)
		}
	})

	t.Run("succeeds but is cancelled from user", func(t *testing.T) {
		t.Parallel()
		respCh := make(chan container.WaitResponse)
		errCh := make(chan error)
		pr, pw := io.Pipe()
		outStreamer := &safeReaderWriter{mu: sync.Mutex{}, LogsReader: pr, LogsWriter: pw}

		cmdOut := []string{"/eirctl", "hello, iteration 1", "hello, iteration 2", "hello, iteration 3", "hello, iteration 4", "hello, iteration 5", "hello, iteration 6", "hello, iteration 7", "hello, iteration 8", "hello, iteration 9", "hello, iteration 10"}

		mcc := mockContainerClientHelper(t, respCh, errCh, outStreamer.LogsReader, nil)

		ce, _, cleanup := mockClientHelper(t, mcc)
		defer cleanup()

		so, se := output.NewSafeWriter(&bytes.Buffer{}), output.NewSafeWriter(&bytes.Buffer{})
		cancelCtx, cancel := context.WithCancel(context.Background())
		go func() {
			cancel()
		}()
		go func() {
			time.Sleep(500 * time.Millisecond)
			respCh <- container.WaitResponse{Error: nil, StatusCode: 0}
		}()
		go func() {
			for _, v := range cmdOut {
				outStreamer.Write(multiplexFrame(1, fmt.Append([]byte(v), "\n")))
			}
			outStreamer.Write([]byte(`\r\n`))
		}()

		_, err := ce.Execute(cancelCtx, &runner.Job{Command: `pwd
for i in $(seq 1 10); do echo "hello, iteration $i"; done`,
			Env:    variables.NewVariables(),
			Vars:   variables.NewVariables(),
			Stdout: so,
			Stderr: se,
			Stdin:  nil,
		})
		if err != nil {
			t.Fatal(err)
		}

		if len(se.String()) > 0 {
			t.Errorf("got error %v, expected nil\n\n", se.String())
		}
	})

	t.Run("error on image not found and pull errored", func(t *testing.T) {
		t.Parallel()
		respCh := make(chan container.WaitResponse)
		errCh := make(chan error)
		mcc := mockContainerClientHelper(t, respCh, errCh, nil, nil)

		mcc.create = func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error) {
			return container.CreateResponse{ID: "created0-123"}, errdefs.ErrNotFound
		}
		mcc.pull = func() (io.ReadCloser, error) {
			return nil, fmt.Errorf("unable to pull")
		}
		mcc.close = func() error {
			return nil
		}
		ce, _, cleanup := mockClientHelper(t, mcc)
		defer cleanup()

		so, se := output.NewSafeWriter(&bytes.Buffer{}), output.NewSafeWriter(&bytes.Buffer{})
		_, err := ce.Execute(context.TODO(), &runner.Job{Command: `unknown --version`,
			Env:    variables.NewVariables(),
			Vars:   variables.NewVariables(),
			Stdout: so,
			Stderr: se,
		})

		if err == nil {
			t.Fatalf("got %v, wanted error", err)
		}
		if !errors.Is(err, runner.ErrContainerCreate) {
			t.Errorf("got %v, wanted %T", err, runner.ErrContainerCreate)
		}
	})
	t.Run("error on image create", func(t *testing.T) {

		t.Parallel()
		respCh := make(chan container.WaitResponse)
		errCh := make(chan error)
		mcc := mockContainerClientHelper(t, respCh, errCh, nil, nil)

		mcc.create = func(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error) {
			return container.CreateResponse{}, fmt.Errorf("unable to create")
		}
		mcc.close = func() error {
			return nil
		}

		ce, _, cleanup := mockClientHelper(t, mcc)
		defer cleanup()

		so, se := output.NewSafeWriter(&bytes.Buffer{}), output.NewSafeWriter(&bytes.Buffer{})
		_, err := ce.Execute(context.TODO(), &runner.Job{Command: `unknown --version`,
			Env:    variables.NewVariables(),
			Vars:   variables.NewVariables(),
			Stdout: so,
			Stderr: se,
		})

		if err == nil {
			t.Fatalf("got %v, wanted error", err)
		}
		if !errors.Is(err, runner.ErrContainerCreate) {
			t.Errorf("got %v, wanted %T", err, runner.ErrContainerCreate)
		}
	})
	t.Run("fails to start container", func(t *testing.T) {
		t.Parallel()
		respCh := make(chan container.WaitResponse)
		errCh := make(chan error)
		pr, pw := io.Pipe()
		outStreamer := &safeReaderWriter{mu: sync.Mutex{}, LogsReader: pr, LogsWriter: pw}

		mcc := mockContainerClientHelper(t, respCh, errCh, outStreamer.LogsReader, nil)

		mcc.start = func(ctx context.Context, containerID string, options container.StartOptions) error {
			return fmt.Errorf("failed to start container")
		}

		ce, _, cleanup := mockClientHelper(t, mcc)
		defer cleanup()

		_, err := ce.Execute(context.TODO(), &runner.Job{Command: `unknown --version`,
			Env:    variables.NewVariables(),
			Vars:   variables.NewVariables(),
			Stdout: io.Discard,
			Stderr: io.Discard,
		})

		if err == nil {
			t.Fatalf("got %v, wanted error", err)
		}
		if !errors.Is(err, runner.ErrContainerStart) {
			t.Fatal("incorrect type of error ")
		}
	})

	t.Run("fails to stream logs", func(t *testing.T) {
		t.Parallel()
		respCh := make(chan container.WaitResponse)
		errCh := make(chan error)
		pr, pw := io.Pipe()
		outStreamer := &safeReaderWriter{mu: sync.Mutex{}, LogsReader: pr, LogsWriter: pw}
		cmdOut := []string{"/eirctl", "hello, iteration 1", "hello, iteration 2", "hello, iteration 3", "hello, iteration 4", "hello, iteration 5", "hello, iteration 6", "hello, iteration 7", "hello, iteration 8", "hello, iteration 9", "hello, iteration 10"}

		mcc := mockContainerClientHelper(t, respCh, errCh, outStreamer.LogsReader, nil)

		mcc.logs = func(ctx context.Context, containerID string, options container.LogsOptions) (io.ReadCloser, error) {
			return nil, fmt.Errorf("failed")
		}

		ce, _, cleanup := mockClientHelper(t, mcc)
		defer cleanup()
		so, se := output.NewSafeWriter(&bytes.Buffer{}), output.NewSafeWriter(&bytes.Buffer{})

		go func() {
			time.Sleep(500 * time.Millisecond)
			respCh <- container.WaitResponse{Error: nil, StatusCode: 0}
		}()
		go func() {
			for _, v := range cmdOut {
				outStreamer.Write(multiplexFrame(1, fmt.Append([]byte(v), "\n")))
			}
			outStreamer.Write([]byte(`\r\n`))
		}()

		_, err := ce.Execute(context.TODO(), &runner.Job{Command: `pwd
for i in $(seq 1 10); do echo "hello, iteration $i"; done`,
			Env:    variables.NewVariables(),
			Vars:   variables.NewVariables(),
			Stdout: so,
			Stderr: se,
			Stdin:  nil,
		})

		if err == nil {
			t.Fatalf("got %v, wanted error", err)
		}
		if !errors.Is(err, runner.ErrContainerLogs) {
			t.Fatal("incorrect type of error ")
		}
	})

	t.Run("incorrect writer throws on multiplexing error", func(t *testing.T) {
		t.Parallel()
		respCh := make(chan container.WaitResponse)
		errCh := make(chan error)
		pr, pw := io.Pipe()
		outStreamer := &safeReaderWriter{mu: sync.Mutex{}, LogsReader: pr, LogsWriter: pw}

		mcc := mockContainerClientHelper(t, respCh, errCh, outStreamer.LogsReader, nil)

		ce, _, cleanup := mockClientHelper(t, mcc)
		defer cleanup()
		ew := &errWriter{
			err:  fmt.Errorf("throw here"),
			resp: 10,
		}
		go func() {
			outStreamer.Write(multiplexFrame(1, fmt.Append([]byte(`hello`), "\n")))
			outStreamer.LogsReader.Close()
		}()

		_, err := ce.Execute(context.TODO(), &runner.Job{Command: `unknown --version`,
			Env:    variables.NewVariables(),
			Vars:   variables.NewVariables(),
			Stdout: io.Discard,
			Stderr: ew,
		})

		if err == nil {
			t.Fatalf("got %v, wanted error", err)
		}
		if !errors.Is(err, runner.ErrContainerMultiplexedStdoutStream) {
			t.Fatal("incorrect type of error ")
		}
	})
	t.Run("succeeds with autoremove running and image not found", func(t *testing.T) {
		t.Parallel()
		respCh := make(chan container.WaitResponse)
		errCh := make(chan error)
		pr, pw := io.Pipe()
		outStreamer := &safeReaderWriter{mu: sync.Mutex{}, LogsReader: pr, LogsWriter: pw}

		cmdOut := []string{"/eirctl", "hello, iteration 1", "hello, iteration 2", "hello, iteration 3", "hello, iteration 4", "hello, iteration 5", "hello, iteration 6", "hello, iteration 7", "hello, iteration 8", "hello, iteration 9", "hello, iteration 10"}

		mcc := mockContainerClientHelper(t, respCh, errCh, outStreamer.LogsReader, nil)
		mcc.inspect = func(ctx context.Context, containerID string) (container.InspectResponse, error) {
			return container.InspectResponse{}, errdefs.ErrNotFound
		}

		ce, _, cleanup := mockClientHelper(t, mcc)
		defer cleanup()

		so, se := output.NewSafeWriter(&bytes.Buffer{}), output.NewSafeWriter(&bytes.Buffer{})
		go func() {
			time.Sleep(500 * time.Millisecond)
			respCh <- container.WaitResponse{Error: nil, StatusCode: 0}
		}()

		go func() {
			for _, v := range cmdOut {
				outStreamer.Write(multiplexFrame(1, fmt.Append([]byte(v), "\n")))
			}
			outStreamer.Write([]byte(`\r\n`))
		}()

		_, err := ce.Execute(context.TODO(), &runner.Job{Command: `pwd
for i in $(seq 1 10); do echo "hello, iteration $i"; done`,
			Env:    variables.NewVariables(),
			Vars:   variables.NewVariables(),
			Stdout: so,
			Stderr: se,
			Stdin:  nil,
		})
		if err != nil {
			t.Fatal(err)
		}

		if len(se.String()) > 0 {
			t.Errorf("got error %v, expected nil\n\n", se.String())
		}
		if len(so.String()) == 0 {
			t.Errorf("got (%s) no output, expected stdout\n\n", so.String())
		}
		want := fmt.Sprintf("%s\n", strings.Join(cmdOut, "\n"))
		if so.String() != want {
			t.Errorf("outputs do not match\n\tgot: %s\n\twanted:  %s", so.String(), want)
		}
	})
	t.Run("fails with insepct on exit status", func(t *testing.T) {
		t.Parallel()
		respCh := make(chan container.WaitResponse)
		errCh := make(chan error)
		pr, pw := io.Pipe()
		outStreamer := &safeReaderWriter{mu: sync.Mutex{}, LogsReader: pr, LogsWriter: pw}

		cmdOut := []string{"/eirctl", "hello, iteration 1", "hello, iteration 2", "hello, iteration 3", "hello, iteration 4", "hello, iteration 5", "hello, iteration 6", "hello, iteration 7", "hello, iteration 8", "hello, iteration 9", "hello, iteration 10"}

		mcc := mockContainerClientHelper(t, respCh, errCh, outStreamer.LogsReader, nil)
		mcc.inspect = func(ctx context.Context, containerID string) (container.InspectResponse, error) {
			return container.InspectResponse{}, fmt.Errorf("unable to inspect")
		}
		ce, _, cleanup := mockClientHelper(t, mcc)
		defer cleanup()

		so, se := output.NewSafeWriter(&bytes.Buffer{}), output.NewSafeWriter(&bytes.Buffer{})
		go func() {
			time.Sleep(500 * time.Millisecond)
			respCh <- container.WaitResponse{Error: nil, StatusCode: 0}
		}()

		go func() {
			for _, v := range cmdOut {
				outStreamer.Write(multiplexFrame(1, fmt.Append([]byte(v), "\n")))
			}
			outStreamer.Write([]byte(`\r\n`))
		}()

		_, err := ce.Execute(context.TODO(), &runner.Job{Command: `pwd
for i in $(seq 1 10); do echo "hello, iteration $i"; done`,
			Env:    variables.NewVariables(),
			Vars:   variables.NewVariables(),
			Stdout: so,
			Stderr: se,
			Stdin:  nil,
		})
		if err == nil {
			t.Fatal(err)
		}
		code, isExit := runner.IsExitStatus(err)
		if !isExit {
			t.Errorf("incorrect err type, got %v, wanted: %v", err, interp.ExitStatus(125))
		}
		if code != 125 {
			t.Errorf("got %d, wanted: 125", code)
		}
	})
	t.Run("fails with non 0 exit code", func(t *testing.T) {
		t.Parallel()
		respCh := make(chan container.WaitResponse)
		errCh := make(chan error)
		pr, pw := io.Pipe()
		outStreamer := &safeReaderWriter{mu: sync.Mutex{}, LogsReader: pr, LogsWriter: pw}

		cmdOut := []string{"/eirctl", "hello, iteration 1", "hello, iteration 2", "hello, iteration 3", "hello, iteration 4", "hello, iteration 5", "hello, iteration 6", "hello, iteration 7", "hello, iteration 8", "hello, iteration 9", "hello, iteration 10"}

		mcc := mockContainerClientHelper(t, respCh, errCh, outStreamer.LogsReader, nil)
		mcc.inspect = func(ctx context.Context, containerID string) (container.InspectResponse, error) {
			resp := container.InspectResponse{
				ContainerJSONBase: &container.ContainerJSONBase{State: &container.State{ExitCode: 1}},
				Config:            &container.Config{Image: "foo/bar", Cmd: []string{"fail me"}},
			}
			return resp, nil
		}

		ce, _, cleanup := mockClientHelper(t, mcc)
		defer cleanup()

		so, se := output.NewSafeWriter(&bytes.Buffer{}), output.NewSafeWriter(&bytes.Buffer{})
		go func() {
			time.Sleep(500 * time.Millisecond)
			respCh <- container.WaitResponse{Error: nil, StatusCode: 0}
		}()

		go func() {
			for _, v := range cmdOut {
				outStreamer.Write(multiplexFrame(1, fmt.Append([]byte(v), "\n")))
			}
			outStreamer.Write([]byte(`\r\n`))
		}()

		_, err := ce.Execute(context.TODO(), &runner.Job{Command: `pwd
for i in $(seq 1 10); do echo "hello, iteration $i"; done`,
			Env:    variables.NewVariables(),
			Vars:   variables.NewVariables(),
			Stdout: so,
			Stderr: se,
			Stdin:  nil,
		})
		if err == nil {
			t.Fatal(err)
		}
		code, isExit := runner.IsExitStatus(err)
		if !isExit {
			t.Errorf("incorrect err type, got %v, wanted: %v", err, interp.ExitStatus(125))
		}
		if code != 1 {
			t.Errorf("got %d, wanted: 1", code)
		}
	})
}

func Test_ContainerExecutor_Execute_Cancelled_CleanUp(t *testing.T) {
	t.Parallel()
	respCh := make(chan container.WaitResponse)
	errCh := make(chan error)
	pr, pw := io.Pipe()
	outStreamer := &safeReaderWriter{mu: sync.Mutex{}, LogsReader: pr, LogsWriter: pw}

	cmdOut := []string{"/eirctl", "hello, iteration 1", "hello, iteration 2", "hello, iteration 3", "hello, iteration 4", "hello, iteration 5", "hello, iteration 6", "hello, iteration 7", "hello, iteration 8", "hello, iteration 9", "hello, iteration 10"}

	so, se := output.NewSafeWriter(&bytes.Buffer{}), output.NewSafeWriter(&bytes.Buffer{})
	cancelCtx, cancel := context.WithCancel(context.Background())
	go func() {
		cancel()
	}()
	go func() {
		time.Sleep(500 * time.Millisecond)
		respCh <- container.WaitResponse{Error: nil, StatusCode: 0}
	}()
	go func() {
		for _, v := range cmdOut {
			outStreamer.Write(multiplexFrame(1, fmt.Append([]byte(v), "\n")))
		}
		outStreamer.Write([]byte(`\r\n`))
	}()
	mcc := mockContainerClientHelper(t, respCh, errCh, outStreamer.LogsReader, nil)

	rmCalled, stopCalled := 0, 0
	// ensure ctx is not cancelled - but parent context is cancelled and hence performing clean up
	mcc.remove = func(ctx context.Context, containerID string, options container.RemoveOptions) error {
		rmCalled++
		if cancelCtx.Err() == nil {
			t.Error("parent context should be cancelled")
		}
		if ctx.Err() != nil {
			t.Error("current context should be active")
		}
		return nil
	}
	mcc.stop = func(ctx context.Context, containerID string, options container.StopOptions) error {
		stopCalled++
		if cancelCtx.Err() == nil {
			t.Error("parent context should be cancelled")
		}
		if ctx.Err() != nil {
			t.Error("current context should be active")
		}
		return nil
	}

	ce, _, cleanup := mockClientHelper(t, mcc)
	defer cleanup()

	_, err := ce.Execute(cancelCtx, &runner.Job{Command: `pwd
for i in $(seq 1 10); do echo "hello, iteration $i"; done`,
		Env:    variables.NewVariables(),
		Vars:   variables.NewVariables(),
		Stdout: so,
		Stderr: se,
		Stdin:  nil,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(se.String()) > 0 {
		t.Errorf("got error %v, expected nil\n\n", se.String())
	}
	if rmCalled+stopCalled != 2 {
		t.Errorf("stop and remove functions were not called")
	}

}
