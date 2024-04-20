package files

import (
	"fmt"
	"os"
	"path/filepath"
)

type Writer struct {
	dst     string   // Name of destination file
	tmp     *os.File // Temporary file to which data is written.
	tmpName string   // Name of temporary file.
	err     error
}

func NewWriter(file string) *Writer {
	w := &Writer{dst: file}
	dir, base := filepath.Dir(file), filepath.Base(file)
	w.tmp, w.err = os.CreateTemp(dir, base+".tmp*")
	if w.err == nil {
		w.tmpName = w.tmp.Name()
	}

	return w
}

func (w *Writer) Write(p []byte) (n int, err error) {
	if w.err != nil {
		return 0, w.err
	}

	if w.tmp == nil {
		return 0, fmt.Errorf("file %s already cleaned up", w.dst)
	}

	n, err = w.tmp.Write(p)
	if err != nil {
		w.err = err

	}

	return n, err
}

func (w *Writer) Close() error {
	if w.err != nil {
		return w.err
	}
	if w.tmp == nil {
		return fmt.Errorf("file %s already cleaned up", w.dst)
	}
	err := w.tmp.Close()
	w.tmp = nil
	if err != nil {
		_ = os.Remove(w.tmpName)
		return err
	}

	if err := os.Rename(w.tmpName, w.dst); err != nil {
		_ = os.Remove(w.tmpName)
		return err
	}

	return nil
}

func (w *Writer) Cleanup() {
	if w.tmp == nil {
		return
	}
	_ = w.tmp.Close()
	w.tmp = nil
	_ = os.Remove(w.tmpName)
}
