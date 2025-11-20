package runner_test

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/Ensono/eirctl/runner"
	"github.com/sirupsen/logrus"
)

func TestTerminalUtils_GetSize_default_unix(t *testing.T) {
	mterm := &mockTerminal{getSizeFn: func(_ int) (width int, height int, err error) {
		return 1, 1, nil
	}}

	stdin, _ := os.CreateTemp("", "stdin-*")
	stdout, _ := os.CreateTemp("", "stdout-*")
	defer os.Remove(stdin.Name())
	defer os.Remove(stdout.Name())

	mtu := runner.NewTerminalUtils(mterm, runner.WithCustomFD(stdin, stdout))
	tsize, fd := mtu.InitInteractiveTerminal()
	if tsize[0] != 1 && tsize[1] != 1 {
		t.Error("terminal size")
	}
	if fd > 0 {
		t.Errorf("fd err")
	}
}

func TestTerminalUtils_GetSize_fallback_on_stdin_uintptr(t *testing.T) {
	mterm := &mockTerminal{}
	mterm.getSizeFn = func(_ int) (width int, height int, err error) {
		// already called by the default unix and failed
		if mterm.getSizeCalled == 2 {
			return 1, 1, nil
		}
		return 1, 1, fmt.Errorf("force fallback")
	}
	stdin, _ := os.CreateTemp("", "stdin-*")
	stdout, _ := os.CreateTemp("", "stdout-*")
	defer os.Remove(stdin.Name())
	defer os.Remove(stdout.Name())

	mtu := runner.NewTerminalUtils(mterm, runner.WithCustomFD(stdin, stdout))
	tsize, fd := mtu.InitInteractiveTerminal()
	if tsize[0] != 1 && tsize[1] != 1 {
		t.Error("terminal size")
	}
	if fd < 3 {
		t.Errorf("got fd (%v) wanted higher than 3", fd)
	}
}

func TestTerminalUtils_GetSize_fallback_on_stdout_uintptr(t *testing.T) {
	mterm := &mockTerminal{}
	mterm.getSizeFn = func(_ int) (width int, height int, err error) {
		// called by default unix and stdint
		if mterm.getSizeCalled == 3 {
			return 1, 1, nil
		}
		return 1, 1, fmt.Errorf("force fallback")
	}
	stdin, _ := os.CreateTemp("", "stdin-*")
	stdout, _ := os.CreateTemp("", "stdout-*")
	defer os.Remove(stdin.Name())
	defer os.Remove(stdout.Name())

	mtu := runner.NewTerminalUtils(mterm, runner.WithCustomFD(stdin, stdout))
	tsize, fd := mtu.InitInteractiveTerminal()
	if tsize[0] != 1 && tsize[1] != 1 {
		t.Error("terminal size")
	}
	// this needs testing on windows as there are more channels there...
	if fd < 3 {
		t.Errorf("fd (%v) should be higher than 3", fd)
	}
}

type mockCmdOut struct {
	err      error
	outbytes []byte
}

func (m mockCmdOut) Output() ([]byte, error) {
	return m.outbytes, m.err
}

func TestTerminalUtils_GetSize_fallback_on_stty_check(t *testing.T) {
	mterm := &mockTerminal{}
	mterm.getSizeFn = func(_ int) (width int, height int, err error) {
		return 1, 1, fmt.Errorf("force fallback")
	}
	stdin, _ := os.CreateTemp("", "stdin-*")
	stdout, _ := os.CreateTemp("", "stdout-*")
	defer os.Remove(stdin.Name())
	defer os.Remove(stdout.Name())

	mtu := runner.NewTerminalUtils(mterm, runner.WithCustomFD(stdin, stdout), runner.WithCustomExecCmd(func(name string, arg ...string) runner.CmdOutputIface {
		return mockCmdOut{err: nil, outbytes: []byte(`100 200`)}
	}))

	tsize, fd := mtu.InitInteractiveTerminal()
	if tsize[0] != 100 && tsize[1] != 200 {
		t.Error("terminal size")
	}

	// this needs testing on windows as there are more channels there...
	if fd != -1 {
		t.Errorf("fd (%v) should be -1", fd)
	}
}

func TestTerminalUtils_GetSize_fallback_on_stty_check_fails_on_output(t *testing.T) {
	mterm := &mockTerminal{}
	mterm.getSizeFn = func(_ int) (width int, height int, err error) {
		return 1, 1, fmt.Errorf("force fallback")
	}
	stdin, _ := os.CreateTemp("", "stdin-*")
	stdout, _ := os.CreateTemp("", "stdout-*")
	defer os.Remove(stdin.Name())
	defer os.Remove(stdout.Name())

	mtu := runner.NewTerminalUtils(mterm, runner.WithCustomFD(stdin, stdout), runner.WithCustomExecCmd(func(name string, arg ...string) runner.CmdOutputIface {
		return mockCmdOut{err: fmt.Errorf("unable to get output"), outbytes: nil}
	}))

	tsize, fd := mtu.InitInteractiveTerminal()
	if tsize[0] != 80 && tsize[1] != 24 {
		t.Error("terminal size error")
	}

	// this needs testing on windows as there are more channels there...
	if fd != -1 {
		t.Errorf("fd (%v) should be -1", fd)
	}
}

func TestTerminalUtils_GetSize_fallback_on_stty_check_fails_on_int_convert(t *testing.T) {

	logrus.SetOutput(&bytes.Buffer{})

	mterm := &mockTerminal{}
	mterm.getSizeFn = func(_ int) (width int, height int, err error) {
		return 1, 1, fmt.Errorf("force fallback")
	}
	stdin, _ := os.CreateTemp("", "stdin-*")
	stdout, _ := os.CreateTemp("", "stdout-*")
	defer os.Remove(stdin.Name())
	defer os.Remove(stdout.Name())

	mtu := runner.NewTerminalUtils(mterm, runner.WithCustomFD(stdin, stdout), runner.WithCustomExecCmd(func(name string, arg ...string) runner.CmdOutputIface {
		return mockCmdOut{err: nil, outbytes: []byte(`100 xyz`)}
	}))

	tsize, fd := mtu.InitInteractiveTerminal()
	if tsize[0] != 80 && tsize[1] != 24 {
		t.Error("terminal size error")
	}

	// this needs testing on windows as there are more channels there...
	if fd != -1 {
		t.Errorf("fd (%v) should be -1", fd)
	}
}

func TestTerminalUtils_GetSize_fallback_on_stty_check_fails_on_split(t *testing.T) {

	logrus.SetOutput(&bytes.Buffer{})

	mterm := &mockTerminal{}
	mterm.getSizeFn = func(_ int) (width int, height int, err error) {
		return 1, 1, fmt.Errorf("force fallback")
	}

	stdin, _ := os.CreateTemp("", "stdin-*")
	stdout, _ := os.CreateTemp("", "stdout-*")
	defer os.Remove(stdin.Name())
	defer os.Remove(stdout.Name())

	mtu := runner.NewTerminalUtils(mterm, runner.WithCustomFD(stdin, stdout), runner.WithCustomExecCmd(func(name string, arg ...string) runner.CmdOutputIface {
		return mockCmdOut{err: nil, outbytes: []byte(`100`)}
	}))

	tsize, fd := mtu.InitInteractiveTerminal()
	if tsize[0] != 80 && tsize[1] != 24 {
		t.Error("terminal size error")
	}

	if mterm.getSizeCalled != 3 {
		t.Errorf("getSize called (%v) expected 3", mterm.getSizeCalled)
	}
	// this needs testing on windows as there are more channels there...
	if fd != -1 {
		t.Errorf("fd (%v) should be -1", fd)
	}
}
