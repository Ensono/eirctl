package output

import (
	"bufio"
	"fmt"
	"io"

	"github.com/Ensono/eirctl/task"
)

type prefixedOutputDecorator struct {
	t *task.Task
	w *SafeWriter
}

func NewPrefixedOutputWriter(t *task.Task, w io.Writer) *prefixedOutputDecorator {
	return &prefixedOutputDecorator{
		t: t,
		w: NewSafeWriter(w),
	}
}

func (d *prefixedOutputDecorator) Write(p []byte) (int, error) {
	n := len(p)
	for {
		// use ScanLines for an easier newlint and empty output management
		advance, line, err := bufio.ScanLines(p, true)
		// All errors should hardstop
		if err != nil {
			return 0, err
		}
		// go to next stream once no tokens left
		if advance == 0 {
			break
		}
		// scan line for TASK_OUTPUT_
		d.t.HandleOutputCapture(line)
		if _, err := d.w.Write(fmt.Appendf(nil, "\x1b[36m%s\x1b[0m: %s\r\n", d.t.Name, line)); err != nil {
			return 0, err
		}
		p = p[advance:]
	}
	return n, nil
}

func (d *prefixedOutputDecorator) WriteHeader() error {
	_, err := d.w.Write(fmt.Appendf(nil, "\x1b[36m[INFO]\x1b[0m: Running task %s...\n", d.t.Name))
	return err
}

func (d *prefixedOutputDecorator) WriteFooter() error {
	_, err := d.w.Write(fmt.Appendf(nil, "\x1b[36m[INFO]\x1b[0m: %s finished. Duration %s\n", d.t.Name, d.t.Duration()))
	return err
}
