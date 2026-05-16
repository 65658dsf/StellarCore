//go:build unix

package server

import (
	"os"
	"syscall"
)

func platformRestartExecutor() RestartExecutor {
	executable, err := os.Executable()
	if err != nil {
		return func() error {
			return err
		}
	}
	args := append([]string{executable}, os.Args[1:]...)
	envs := os.Environ()

	return func() error {
		return syscall.Exec(executable, args, envs)
	}
}
