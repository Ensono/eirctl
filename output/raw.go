package output

import (
	"io"

	"github.com/Ensono/eirctl/task"
)

// rawOutputDecorator sets up the writer
// most commonly this will be a bytes.Buffer which is not concurrency safe
// mu property locks it from multiple writes
type rawOutputDecorator struct {
	w *SafeWriter
	t *task.Task
}

func newRawOutputWriter(t *task.Task, w io.Writer) *rawOutputDecorator {
	return &rawOutputDecorator{w: NewSafeWriter(w), t: t}
}

func (d *rawOutputDecorator) WriteHeader() error {
	return nil
}

func (d *rawOutputDecorator) Write(b []byte) (int, error) {
	d.t.HandleOutputCapture(b)
	return d.w.Write(b)
}

func (d *rawOutputDecorator) WriteFooter() error {
	return nil
}
