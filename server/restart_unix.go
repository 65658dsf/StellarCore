//go:build unix

package server

import (
	"os"
	"syscall"
)

func platformRestartExecutor() RestartExecutor {
	return func() error {
		executable, err := os.Executable()
		if err != nil {
			return err
		}
		return syscall.Exec(executable, append([]string{executable}, os.Args[1:]...), os.Environ())
	}
}
