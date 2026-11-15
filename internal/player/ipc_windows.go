//go:build windows

package player

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/Microsoft/go-winio"
)

func ipcPath() string {
	return fmt.Sprintf(`\\.\pipe\gundil-mpv-%d`, os.Getpid())
}

func dialIPC(path string) (io.ReadWriteCloser, error) {
	timeout := 2 * time.Second
	return winio.DialPipe(path, &timeout)
}
