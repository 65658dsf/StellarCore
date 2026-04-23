//go:build !unix

package server

func platformRestartExecutor() RestartExecutor {
	return nil
}
