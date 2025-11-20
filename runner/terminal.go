package runner

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
	"golang.org/x/term"
)

type Terminal interface {
	MakeRaw(fd int) (*term.State, error)
	Restore(fd int, state *term.State) error
	IsTerminal(fd int) bool
	GetSize(fd int) (width, height int, err error)
}

type realTerminal struct{}

// MakeRaw accepts a stdin fd pointer
func (t *realTerminal) MakeRaw(fd int) (*term.State, error) {
	return term.MakeRaw(fd)
}

// Restore accepts a stdin fd pointer
func (t *realTerminal) Restore(fd int, state *term.State) error {
	return term.Restore(fd, state)
}

// IsTerminal accepts a terminalFD
func (t *realTerminal) IsTerminal(fd int) bool {
	return term.IsTerminal(fd)
}

func (t *realTerminal) GetSize(fd int) (width, height int, err error) {
	return term.GetSize(fd)
}

type CmdOutputIface interface {
	Output() ([]byte, error)
}

// TerminalUtils provides some utilities over the terminals in differen OS's
//
// NOTE: we need to remove/rework the utils package
type TerminalUtils struct {
	term       Terminal
	terminalFd int
	stdInFd    FileFDIface
	stdOutFd   FileFDIface
	execCmd    func(name string, arg ...string) CmdOutputIface
}

type TerminalUtilsOpt func(*TerminalUtils)

type FileFDIface interface {
	Fd() uintptr
}

func WithCustomFD(stdin, stdout *os.File) TerminalUtilsOpt {
	return func(tu *TerminalUtils) {
		tu.stdInFd = stdin
		tu.stdOutFd = stdout
	}
}

func WithCustomExecCmd(execmd func(name string, arg ...string) CmdOutputIface) TerminalUtilsOpt {
	return func(tu *TerminalUtils) {
		tu.execCmd = execmd
	}
}

func NewTerminalUtils(term Terminal, opts ...TerminalUtilsOpt) *TerminalUtils {

	// setting up default terminalUtils
	tu := &TerminalUtils{
		term:     term,
		stdInFd:  os.Stdin,
		stdOutFd: os.Stdout,
		execCmd: func(name string, arg ...string) CmdOutputIface {
			return exec.Command("stty", "size")
		},
	}

	for _, fn := range opts {
		fn(tu)
	}

	return tu
}

// InitInteractiveTerminal gets the terminal size and file descriptor, through various methods or will
// fallback to 80x24 which is the standard.
//
// Returns an int with the width and height, and also the file descriptor.
//
// If the last fallback is reached the terminal file descriptor will be -1
// signalling failure. You can continue to size the terminal to a known value
// but you won't be able to use other terminal functions that rely on a valid
// file descriptor.
func (tu *TerminalUtils) InitInteractiveTerminal() ([2]int, int) {
	var terminalSize [2]int
	var err error
	tu.terminalFd = -1

	// Try fd zero, most *nix systems use this
	terminalSize, err = getTerminalSize(tu.term, "0", 0)
	if err == nil {
		tu.terminalFd = 0
		return terminalSize, 0
	}

	// Try stdin's fd next
	// Most articles say this is the method to use...
	terminalSize, err = getTerminalSize(tu.term, "stdin", tu.stdInFd.Fd())
	if err == nil {
		tu.terminalFd = int(tu.stdInFd.Fd())
		return terminalSize, int(tu.stdInFd.Fd())
	}

	// Try stdout, it seems a whole host of terminals respond to this
	// This also will return when stdin is a PIPE and so not a Terminal
	terminalSize, err = getTerminalSize(tu.term, "stdout", tu.stdOutFd.Fd())
	if err == nil {
		tu.terminalFd = int(tu.stdOutFd.Fd())
		return terminalSize, int(tu.stdOutFd.Fd())
	}

	// Last shot in the dark, try using `stty size`
	// If present on the system (such as through WSL) this works even in cmd
	terminalSize, err = getTerminalSizeFromSttyCommand(tu.execCmd)
	if err == nil {
		// File Descriptor will be -1
		return terminalSize, -1
	}

	// All fallbacks have failed, assume the standard terminal size and return
	// -1 as the file descriptor
	logrus.Warn("utils.getTerminalSize: All fallback methods have been exhausted, assuming a standard terminal size of 80x24.\nThis might result in weird behaviour.")
	return [2]int{80, 24}, -1
}

func (tu *TerminalUtils) UpdateSize(fd int) (width, height int, err error) {
	width, height, err = tu.term.GetSize(fd)

	if err != nil {
		logrus.Error("Terminal.UpdateSize: Failed to get size")
		return 0, 0, nil
	}

	return width, height, nil
}

func getTerminalSize(termix Terminal, fileDescriptorName string, fileDescriptor uintptr) ([2]int, error) {
	logrus.Tracef("utils.getTerminalSize: Trying %s: %d", fileDescriptorName, fileDescriptor)
	width, height, err := termix.GetSize(int(fileDescriptor))
	if err != nil {
		logrus.Tracef("utils.getTerminalSize: '%s': '%d' failed, error: %s", fileDescriptorName, fileDescriptor, err.Error())
		return [2]int{0, 0}, err
	}

	return [2]int{width, height}, nil
}

func getTerminalSizeFromSttyCommand(execCmd func(name string, arg ...string) CmdOutputIface) ([2]int, error) {
	logrus.Tracef("utils.getTerminalSizeFromSttyCommand Trying to execute 'stty size' as a final fallback")

	output, err := execCmd("stty", "size").Output()
	if err != nil {
		logrus.Tracef("utils.getTerminalSizeFromSttyCommand: command failed:\n\t- Output: %s\n\tErr: %s", output, err.Error())
		// this will still fallback to the default 80x24
		return [2]int{}, errors.New("utils.getTerminalSizeFromSttyCommand: failed to run command")
	}

	outputString := strings.Split(string(output), " ")

	if len(outputString) != 2 {
		logrus.Tracef("utils.getTerminalSizeFromSttyCommand: Expected two numbers as a response, got: %s", output)
		return [2]int{}, errors.New("utils.getTerminalSizeFromSttyCommand: Expected two integers back from stty")
	}

	width, err := strconv.Atoi(outputString[0])
	height, err2 := strconv.Atoi(outputString[1])

	if err != nil || err2 != nil {
		logrus.Tracef("utils.getTerminalSizeFromSttyCommand: Expected two numbers as a response, got:\n\t - Width: %d\n\r - Height: %d", width, height)

		return [2]int{}, errors.New("utils.getTerminalSizeFromSttyCommand: Width or Height is invalid")
	}

	return [2]int{width, height}, nil
}
