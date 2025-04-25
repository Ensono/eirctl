package runner

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"runtime"
	"strings"
	"time"

	"github.com/Ensono/eirctl/internal/utils"
	"github.com/Ensono/eirctl/variables"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	"github.com/docker/docker/pkg/stdcopy"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
	"golang.org/x/term"
	"mvdan.cc/sh/v3/interp"
)

var (
	ErrImagePull                        = errors.New("failed to pull container image")
	ErrRegistryAuth                     = errors.New("failed to auth to registry")
	ErrContainerCreate                  = errors.New("failed to create container")
	ErrContainerStart                   = errors.New("failed to start container")
	ErrContainerWait                    = errors.New("failed to wait for container")
	ErrContainerLogs                    = errors.New("failed to get container logs")
	ErrContainerExecCmd                 = errors.New("failed to run cmd in container")
	ErrContainerMultiplexedStdoutStream = errors.New("failed to de-muiltiplex the stream")
)

// ContainerExecutorIface interface used by this implementation
type ContainerExecutorIface interface {
	Close() error
	ImagePull(ctx context.Context, refStr string, options image.PullOptions) (io.ReadCloser, error)
	ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, platform *ocispec.Platform, containerName string) (container.CreateResponse, error)
	ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error
	ContainerWait(ctx context.Context, containerID string, condition container.WaitCondition) (<-chan container.WaitResponse, <-chan error)
	ContainerLogs(ctx context.Context, containerID string, options container.LogsOptions) (io.ReadCloser, error)
	ContainerInspect(ctx context.Context, containerID string) (container.InspectResponse, error)
	// Shell
	ContainerAttach(ctx context.Context, container string, options container.AttachOptions) (types.HijackedResponse, error)
	ContainerResize(ctx context.Context, containerID string, options container.ResizeOptions) error
}

type Terminal interface {
	MakeRaw(fd int) (*term.State, error)
	Restore(fd int, state *term.State) error
	IsTerminal(fd int) bool
	GetSize(fd int) (width, height int, err error)
}

type realTerminal struct{}

func (t realTerminal) MakeRaw(fd int) (*term.State, error) {
	return term.MakeRaw(fd)
	// return term.MakeRaw(uintptr(fd))
}

func (t realTerminal) Restore(fd int, state *term.State) error {
	return term.Restore(fd, state)
	// return term.RestoreTerminal(uintptr(fd), state)
}

func (t realTerminal) IsTerminal(fd int) bool {
	return term.IsTerminal(fd)
}

func (t realTerminal) GetSize(fd int) (width, height int, err error) {
	return term.GetSize(fd)
}

type ContainerExecutor struct {
	// containerClient
	cc          ContainerExecutorIface
	execContext *ExecutionContext
	Term        Terminal
}

type ContainerOpts func(*ContainerExecutor)

// NewContainerExecutor initialises an OCI compliant client
//
// It implicitely creates it from `env` any missing vars required to initialise it,
// will be flagged in the error response.
func NewContainerExecutor(execContext *ExecutionContext, opts ...ContainerOpts) (*ContainerExecutor, error) {
	// NOTE: potentially check env vars are set here
	// also cover it in tests to ensure errors are handled correctly
	// os.Setenv("DOCKER_HOST", "unix:///var/run/docker.sock")
	c, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	ce := &ContainerExecutor{
		cc:          c,
		execContext: execContext,
		Term:        realTerminal{},
	}

	for _, opt := range opts {
		opt(ce)
	}

	return ce, nil
}

func WithContainerClient(client ContainerExecutorIface) ContainerOpts {
	return func(ce *ContainerExecutor) {
		ce.cc = client
	}
}

func (e *ContainerExecutor) WithReset(doReset bool) {}

// Execute executes given job with provided context
// Returns job output
func (e *ContainerExecutor) Execute(ctx context.Context, job *Job) ([]byte, error) {
	defer e.cc.Close()

	containerContext := e.execContext.Container()
	cmd := containerContext.ShellArgs
	cmd = append(cmd, job.Command)
	tty, attachStdin := false, false
	remoteDir := ""
	if e.execContext.Dir != job.Dir {
		remoteDir = job.Dir
	}
	// everything in the container is relative to the `/eirctl` directory
	wd := path.Join("/eirctl", remoteDir)
	// adding the opiniated PWD into the Container Env as per the wd variable
	cEnv := utils.ConvertEnv(utils.ConvertToMapOfStrings(
		job.Env.Merge(variables.FromMap(map[string]string{"PWD": wd})).
			Merge(variables.FromMap(containerContext.envOverride)).
			Map()))

	containerConfig, hostConfig := platformContainerConfig(containerContext, cEnv, cmd, wd, tty, attachStdin)

	if job.IsShell && job.Stdin != nil {
		return e.shell(ctx, containerConfig, hostConfig, job)
	}
	return e.execute(ctx, containerConfig, hostConfig, job)
}

func (e *ContainerExecutor) createContainer(ctx context.Context, containerConfig *container.Config, hostConfig *container.HostConfig, job *Job) (container.CreateResponse, error) {

	resp, err := e.cc.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "")
	if err != nil {
		if errdefs.IsNotFound(err) {
			if err := e.PullImage(ctx, containerConfig.Image, job.Stdout); err != nil {
				return container.CreateResponse{}, err
			}
			// Image pulled now create container
			return e.createContainer(ctx, containerConfig, hostConfig, job)
		}
		return container.CreateResponse{}, fmt.Errorf("%v\n%w", err, ErrContainerCreate)
	}
	return resp, nil
}

func (e *ContainerExecutor) execute(ctx context.Context, containerConfig *container.Config, hostConfig *container.HostConfig, job *Job) ([]byte, error) {

	logrus.Debugf("%+v", containerConfig)

	// createdContainer
	createdContainer, err := e.createContainer(ctx, containerConfig, hostConfig, job)
	if err != nil {
		return nil, fmt.Errorf("%v\n%w", err, ErrContainerCreate)
	}

	if err := e.cc.ContainerStart(ctx, createdContainer.ID, container.StartOptions{}); err != nil {
		return nil, fmt.Errorf("%v\n%w", err, ErrContainerStart)
	}

	// streamLogs
	errExecCh := make(chan error)
	doneReadingCh := make(chan struct{})
	if err = e.streamLogs(ctx, createdContainer.ID, errExecCh, doneReadingCh, job); err != nil {
		return nil, err
	}

	statusWaitCh, errWaitCh := e.cc.ContainerWait(ctx, createdContainer.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errWaitCh:
		if err != nil {
			return nil, fmt.Errorf("%v\n%w", err, ErrContainerWait)
		}
	case err := <-errExecCh:
		if err != nil {
			return nil, err
		}
	case <-statusWaitCh:
		// even though the container is technically finished
		// in some case the buffer might still be copied in to.
		// we need to make sur this blocks for maximum of 5 sec
		// or until it is done reading
		// now wait up to 1s for the drain to complete
		timer := time.NewTimer(1 * time.Second)
		defer timer.Stop()
		select {
		case <-doneReadingCh:
			logrus.Debug("fully drained logs after exit")
		case <-timer.C:
			logrus.Debug("timed out waiting on buffer to drain completely...")
		}
	}
	return []byte{}, e.checkExitStatus(ctx, createdContainer.ID)
}

// shell runs the interactive mode for a given context
func (e *ContainerExecutor) shell(ctx context.Context, containerConfig *container.Config, hostConfig *container.HostConfig, job *Job) ([]byte, error) {

	mutateShellContainerConfig(containerConfig)

	hostConfig.AutoRemove = true

	logrus.Debugf("creating with config %+v", containerConfig)

	if runtime.GOOS == "windows" && containerConfig.Cmd[0] == "pwsh" {
		containerConfig.Cmd = append(containerConfig.Cmd, "-NoLogo", "-NoProfile",
			"-Command", "Remove-Module PSReadLine; pwsh")
	}

	// createdContainer
	createdContainer, err := e.createContainer(ctx, containerConfig, hostConfig, job)
	if err != nil {
		return nil, err
	}

	// Attach to container stdio
	attachedResp, err := e.cc.ContainerAttach(ctx, createdContainer.ID, container.AttachOptions{
		Stream: true,
		Stdin:  true,
		Stdout: true,
		Stderr: true,
		Logs:   false,
	})
	if err != nil {
		return nil, err
	}
	defer attachedResp.Close()

	// Set terminal to raw mode
	fd := int(os.Stdin.Fd())
	if e.Term.IsTerminal(fd) {
		logrus.Debug("Making Terminal Raw")
		oldState, err := e.Term.MakeRaw(fd)
		if err != nil {
			return nil, err
		}
		// defer e.Term.Restore(fd, oldState)
		defer func() {
			_ = e.Term.Restore(fd, oldState)
		}()
	}

	e.resizeShellTTY(ctx, fd, createdContainer.ID)

	// Start container with a defined shell
	if err := e.cc.ContainerStart(ctx, createdContainer.ID, container.StartOptions{}); err != nil {
		return nil, fmt.Errorf("%v\n%w", err, ErrContainerStart)
	}

	sigCh := resizeSignal()
	go func() {
		for range sigCh {
			logrus.Debug("terminal window resized")
			e.resizeShellTTY(ctx, fd, createdContainer.ID)
		}
	}()

	// Start copying stdin -> container Connection
	go func() {
		if _, err := io.Copy(attachedResp.Conn, job.Stdin); err != nil {
			logrus.Debug(err)
		}
	}()

	// container stdout and terminal/host -> stdout
	go func() {
		if _, err := io.Copy(job.Stdout, attachedResp.Conn); err != nil {
			logrus.Debug(err)
		}
	}()

	statusWaitCh, errWaitCh := e.cc.ContainerWait(ctx, createdContainer.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errWaitCh:
		if err != nil {
			return nil, fmt.Errorf("%v\n%w", err, ErrContainerWait)
		}
	case <-statusWaitCh:
		logrus.Debug("exiting container...")
	}
	return []byte{}, nil
}

func (e *ContainerExecutor) resizeShellTTY(ctx context.Context, fd int, containerId string) {
	if e.Term.IsTerminal(fd) {
		width, height, err := e.Term.GetSize(fd)
		if err != nil {
			logrus.Debugf("failed to get terminal size: %v", err)
			return
		}
		_ = e.cc.ContainerResize(ctx, containerId, container.ResizeOptions{
			Height: uint(height),
			Width:  uint(width),
		})
	}
}

// Container pull images - all contexts that have a container property
func (e *ContainerExecutor) PullImage(ctx context.Context, name string, dstOutput io.Writer) error {
	logrus.Debugf("pulling image: %s", name)

	pullOpts, err := platformPullOptions(ctx, name)
	if err != nil {
		return err
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	reader, err := e.cc.ImagePull(timeoutCtx, name, pullOpts)
	if err != nil {
		return fmt.Errorf("%v\n%w", err, ErrImagePull)
	}

	defer reader.Close()
	// container.ImagePull is asynchronous.
	// The reader needs to be read completely for the pull operation to complete.
	// If stdout is not required, consider using io.Discard instead of os.Stdout.
	// Debug log pull image output
	b := &bytes.Buffer{}
	if _, err := io.Copy(b, reader); err != nil {
		return err
	}
	logrus.Debug(b.String())
	return nil
}

func (e *ContainerExecutor) streamLogs(ctx context.Context, containerId string, errCh chan<- error, doneReadingCh chan<- struct{}, job *Job) error {
	out, err := e.cc.ContainerLogs(ctx, containerId, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Timestamps: false,
		Follow:     true,
	})
	if err != nil {
		return fmt.Errorf("%v\n%w", err, ErrContainerLogs)
	}

	go func() {
		defer out.Close()
		reader := bufio.NewReader(out)
		for {
			if _, err := stdcopy.StdCopy(job.Stdout, job.Stderr, reader); err != nil {
				// Handle EOF (when logs stop)
				// or reading from a closed socket after close
				if errors.Is(err, io.EOF) ||
					strings.Contains(err.Error(), "use of closed network connection") ||
					errors.Is(err, http.ErrBodyReadAfterClose) ||
					strings.Contains(err.Error(), "read on closed response body") {
					logrus.Infof("exiting on reader closed %v", err)
					break
				}
				logrus.Errorf("stream buffer exiting on %s", err)
				errCh <- fmt.Errorf("%w: %v", ErrContainerMultiplexedStdoutStream, err)
				return
			}
		}
		doneReadingCh <- struct{}{}
	}()

	return nil
}

func (e *ContainerExecutor) checkExitStatus(ctx context.Context, containerId string) error {
	resp, err := e.cc.ContainerInspect(ctx, containerId)
	if err != nil {
		logrus.Debugf("%v: %v", ErrContainerLogs, err)
		return interp.NewExitStatus(125)
	}
	if resp.State.ExitCode != 0 {
		logrus.Debugf("container image (%s) command %v failed with non-zero exit code", resp.Image, resp.Config.Cmd)
		return interp.NewExitStatus(uint8(resp.State.ExitCode))
	}
	return nil
}
