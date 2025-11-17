//go:build !windows
// +build !windows

package utils

import (
	"errors"
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/net/context"
)

type Result struct {
	output string
	err    error
}

// 执行shell命令，可设置执行超时时间
func ExecShell(ctx context.Context, command string) (string, error) {
	cmd := exec.Command("/bin/bash", "-c", command)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	// 设置工作目录为用户家目录，避免 getcwd 错误
	if homeDir, err := os.UserHomeDir(); err == nil {
		cmd.Dir = homeDir
	} else {
		cmd.Dir = "/tmp"
	}
	resultChan := make(chan Result)
	go func() {
		output, err := cmd.CombinedOutput()
		resultChan <- Result{string(output), err}
	}()
	select {
	case <-ctx.Done():
		if cmd.Process.Pid > 0 {
			syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		return "", errors.New("timeout killed")
	case result := <-resultChan:
		return result.output, result.err
	}
}
