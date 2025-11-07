package cmdutils

import (
	"io"
	"os"
)

type OsFSOpsIface interface {
	Rename(oldpath string, newpath string) error
	WriteFile(name string, data []byte, perm os.FileMode) error
	Create(name string) (io.Writer, error)
}

// OsFsOps is a concrete implementation of the above iface
type OsFsOps struct {
}

func (o OsFsOps) Rename(oldpath string, newpath string) error {
	return os.Rename(oldpath, newpath)
}

func (o OsFsOps) WriteFile(name string, data []byte, perm os.FileMode) error {
	return os.WriteFile(name, data, perm)
}

func (o OsFsOps) Create(name string) (io.Writer, error) {
	return os.Create(name)
}
