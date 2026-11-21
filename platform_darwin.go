package main

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

func openURL(url string) error     { return native("open", url).Run() }
func openFolder(dir string) error  { return native("open", dir).Run() }
func revealFile(path string) error { return native("open", "-R", path).Run() }

func chooseFolder() (string, error) {
	out, err := native("osascript", "-e", `POSIX path of (choose folder with prompt "Choose where to save")`).Output()
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return "", nil
	}
	return strings.TrimSuffix(string(out), "\n"), err
}

func freeBytes(path string) (uint64, error) {
	var s syscall.Statfs_t
	err := syscall.Statfs(path, &s)
	return s.Bavail * uint64(s.Bsize), err
}

func procAttr() *syscall.SysProcAttr { return &syscall.SysProcAttr{Setpgid: true} }
func killTree(pid int) error         { return syscall.Kill(-pid, syscall.SIGKILL) }
func lockInstance(f *os.File) (bool, error) {
	err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) {
		return false, nil
	}
	return err == nil, err
}
