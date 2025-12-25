//go:build windows
// +build windows

package utils

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"syscall"
	"time"
)

type Result struct {
	output string
	err    error
}

// 执行shell命令，可设置执行超时时间
// 改进：即使超时或被取消，也会返回已产生的输出
func ExecShell(ctx context.Context, command string) (string, error) {
	// 清理可能存在的 HTML 实体编码,防止 &quot; 等导致命令执行失败
	// 例如: del &quot;C:\file.txt&quot; -> del "C:\file.txt"
	command = CleanHTMLEntities(command)

	// 使用 cmd.exe，通过 CmdLine 直接传递完整命令行，绕过 Go 的参数转义
	cmd := exec.Command("cmd")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
		CmdLine:    `cmd /c "` + command + `"`,
	}
	// 设置工作目录为用户家目录
	if homeDir, err := os.UserHomeDir(); err == nil {
		cmd.Dir = homeDir
	} else {
		cmd.Dir = os.TempDir()
	}

	// 使用管道实时捕获输出
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}

	// 用于收集输出
	var outputBuffer bytes.Buffer
	var wg sync.WaitGroup

	// 启动命令
	if err := cmd.Start(); err != nil {
		return "", err
	}

	// 实时读取 stdout 和 stderr
	var mu sync.Mutex
	wg.Add(2)
	go func() {
		defer wg.Done()
		buf := make([]byte, 1024)
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				mu.Lock()
				outputBuffer.Write(buf[:n])
				mu.Unlock()
			}
			if err != nil {
				break
			}
		}
	}()
	go func() {
		defer wg.Done()
		buf := make([]byte, 1024)
		for {
			n, err := stderr.Read(buf)
			if n > 0 {
				mu.Lock()
				outputBuffer.Write(buf[:n])
				mu.Unlock()
			}
			if err != nil {
				break
			}
		}
	}()

	// 等待命令完成或超时
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-ctx.Done():
		// 超时或被取消，尝试终止进程
		if cmd.Process != nil && cmd.Process.Pid > 0 {
			// Windows 下先尝试正常终止
			cmd.Process.Kill()

			// 等待 2 秒，看进程是否退出
			timer := time.NewTimer(2 * time.Second)
			select {
			case <-done:
				timer.Stop()
			case <-timer.C:
				// 强制杀死进程树
				exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(cmd.Process.Pid)).Run()
				<-done
			}
		}

		// 等待 IO 读取完成
		wg.Wait()

		// 返回已捕获的输出（转换编码）和错误信息
		mu.Lock()
		output := outputBuffer.String()
		mu.Unlock()
		return ConvertEncoding(output), errors.New("timeout killed")

	case err := <-done:
		// 命令正常完成
		wg.Wait()
		mu.Lock()
		output := outputBuffer.String()
		mu.Unlock()
		return ConvertEncoding(output), err
	}
}

func ConvertEncoding(outputGBK string) string {
	// windows平台编码为gbk，需转换为utf8才能入库
	outputUTF8, ok := GBK2UTF8(outputGBK)
	if ok {
		return outputUTF8
	}

	return outputGBK
}
