package runner

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

type termIface interface {
	GetSize(fd int) (width int, height int, err error)
}

type CmdOutputIface interface {
	Output() ([]byte, error)
}

// TerminalUtils provides some utilities over the terminals in differen OS's
//
// NOTE: we need to remove/rework the utils package
type TerminalUtils struct {
	term     termIface
	stdInFd  FileFDIface
	stdOutFd FileFDIface
	execCmd  func(name string, arg ...string) CmdOutputIface
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

func NewTerminalUtils(term termIface, opts ...TerminalUtilsOpt) *TerminalUtils {

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

// GetTerminalSize gets the terminal size and file descriptor, through various methods or will
// fallback to 80x24 which is the standard.
//
// Returns an int with the width and height, and also the file descriptor.
//
// If the last fallback is reached the terminal file descriptor will be -1
// signalling failure. You can continue to size the terminal to a known value
// but you won't be able to use other terminal functions that rely on a valid
// file descriptor.
func (tu *TerminalUtils) GetTerminalSize() ([2]int, int) {
	var terminalSize [2]int
	var err error

	// Try fd zero, most *nix systems use this
	terminalSize, err = getTerminalSize(tu.term, "0", 0)
	if err == nil {
		return terminalSize, 0
	}

	// Try stdin's fd next
	// Most articles say this is the method to use...
	terminalSize, err = getTerminalSize(tu.term, "stdin", tu.stdInFd.Fd())
	if err == nil {
		return terminalSize, int(tu.stdInFd.Fd())
	}

	// Try stdout, it seems a whole host of terminals respond to this
	// This also will return when stdin is a PIPE and so not a Terminal
	terminalSize, err = getTerminalSize(tu.term, "stdout", tu.stdOutFd.Fd())
	if err == nil {
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

func getTerminalSize(termix termIface, fileDescriptorName string, fileDescriptor uintptr) ([2]int, error) {
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
