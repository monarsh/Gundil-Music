//go:build !windows

package player

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
)

func ipcPath() string {
	path := filepath.Join(os.TempDir(), fmt.Sprintf("gundil-mpv-%d.sock", os.Getpid()))
	os.Remove(path)
	return path
}

func dialIPC(path string) (io.ReadWriteCloser, error) {
	return net.Dial("unix", path)
}
