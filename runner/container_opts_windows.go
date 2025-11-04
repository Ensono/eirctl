//go:build windows
// +build windows

package runner

import (
	"context"
	"time"
	"unsafe"

	"os"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"
)

var (
	modkernel32                    = windows.NewLazySystemDLL("kernel32.dll")
	procGetConsoleScreenBufferInfo = modkernel32.NewProc("GetConsoleScreenBufferInfo")
)

type winSigWinch struct{}

func (w winSigWinch) Signal()        {}
func (w winSigWinch) String() string { return "WINDOWS_SIGWINCH" }

type coord struct {
	X int16
	Y int16
}

func platformPullOptions(ctx context.Context, containerConf *container.Config) (image.PullOptions, error) {
	pullOpts := image.PullOptions{}

	ra, err := AuthLookupFunc(containerConf)(ctx)
	if err != nil {
		return image.PullOptions{}, err
	}
	pullOpts.RegistryAuth = ra

	return pullOpts, nil
}

func platformContainerConfig(containerContext *ContainerContext, cEnv []string, cmd []string, wd string, term Terminal, tty, attachStdin bool) (*container.Config, *container.HostConfig) {
	containerPorts, hostPorts := containerContext.Ports()

	containerConfig := &container.Config{
		Image:       containerContext.Image,
		Entrypoint:  containerContext.Entrypoint,
		Env:         cEnv,
		Cmd:         cmd,
		Tty:         tty, // TODO: TTY along with StdIn will require switching off stream multiplexer
		AttachStdin: attachStdin,
		// OpenStdin: ,
		// WorkingDir in a container will always be /eirctl
		// will append any job specified paths to the default working
		WorkingDir:   wd,
		User:         containerContext.User(),
		ExposedPorts: containerPorts,
	}

	hostConfig := &container.HostConfig{
		Mounts:       []mount.Mount{},
		UsernsMode:   container.UsernsMode(containerContext.userns),
		PortBindings: hostPorts,
		AutoRemove:   true,
	}

	// Only mount of type bind can be used on windows
	for _, volume := range containerContext.BindMounts() {
		hostConfig.Mounts = append(hostConfig.Mounts, mount.Mount{
			// TODO: enable additional mount types
			// e.g. `image` for built container volume inspection
			Type:   mount.TypeBind, // current default is bind
			Source: volume.SourcePath,
			Target: volume.TargetPath,
			// FIXME: allow a more comprehensive list of options.
			//
			// Perhaps struct embedding from the docker/api/types package in the context definition.
			// BindOptions:   &mount.BindOptions{},
			// VolumeOptions: &mount.VolumeOptions{},
			// Consistency:   mount.ConsistencyDefault,
			// TmpfsOptions:  &mount.TmpfsOptions{},
		})
	}

	// Debug config
	logrus.Debugf("ContainerConfig: %+v", containerConfig)
	logrus.Debugf("HostConfig: %+v", hostConfig)
	return containerConfig, hostConfig
}

func mutateShellContainerConfig(containerConfig *container.Config) {
	containerConfig.Tty = true
	containerConfig.OpenStdin = true
	containerConfig.AttachStdin = true
	containerConfig.AttachStdout = true
	containerConfig.AttachStderr = true
	containerConfig.Cmd = []string{containerConfig.Cmd[0]}

	logrus.Debugf("Shell Mutated Windows ContainerConfig: %+v", containerConfig)
}

// resizeSignal polls for console size changes without consuming stdin input
func resizeSignal(term Terminal) chan os.Signal {
	ch := make(chan os.Signal, 1)

	go func() {
		defer close(ch)
		hOut := windows.Handle(term.GetTerminalFd())
		winSigWinch := &winSigWinch{}

		var prevSize coord

		for {
			var info windows.ConsoleScreenBufferInfo

			// NOTE: I'm not sure why, but this call fails when using it
			// directly from the 'windows' package...
			// This is a workaround to import the DLL call it using FFI instead...
			r1, _, err := procGetConsoleScreenBufferInfo.Call(
				uintptr(hOut),
				uintptr(unsafe.Pointer(&info)),
			)

			// NOTE: Unlike most terminal calls: non-zero means success, zero means failure
			// See: https://learn.microsoft.com/en-us/windows/console/getconsolescreenbufferinfo
			if r1 == 0 {
				logrus.Tracef("resizeSignal(windows) failed 'GetConsoleScreenBufferInfo': %s", err.Error())
			} else {
				// Store off-by-one width and height as this is calculated every 300ms...
				width := info.Window.Right - info.Window.Left
				height := info.Window.Bottom - info.Window.Top
				curSize := coord{X: width, Y: height}

				if curSize != prevSize {
					prevSize = curSize
					logrus.Tracef("resizeSignal(windows): Terminal resized to: %dx%d, Sending '%s' to the channel", width+1, height+1, winSigWinch.String())
					select {
					case ch <- winSigWinch:
					default:
					}
				}
			}

			time.Sleep(300 * time.Millisecond)
		}
	}()

	return ch
}
