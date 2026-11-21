package main

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

func openURL(url string) error { return native("rundll32", "url.dll,FileProtocolHandler", url).Run() }
func explorer(arg string) error {
	err := native("explorer", arg).Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return nil
	}
	return err
}
func openFolder(dir string) error  { return explorer(dir) }
func revealFile(path string) error { return explorer("/select," + path) }

func chooseFolder() (string, error) {
	script := `[Console]::OutputEncoding = [Text.UTF8Encoding]::new($false)
Add-Type -AssemblyName System.Windows.Forms
$picker = New-Object System.Windows.Forms.FolderBrowserDialog
$picker.Description = 'Choose where to save'
try {
  if ($picker.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) {
    [Console]::Write($picker.SelectedPath)
  }
} finally {
  $picker.Dispose()
}`
	out, err := native("powershell", "-NoProfile", "-STA", "-Command", script).Output()
	return strings.TrimRight(string(out), "\r\n"), err
}

var kernel = syscall.NewLazyDLL("kernel32.dll")

func freeBytes(path string) (uint64, error) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	var available uint64
	ok, _, err := kernel.NewProc("GetDiskFreeSpaceExW").Call(uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&available)), 0, 0)
	if ok == 0 {
		return 0, err
	}
	return available, nil
}

func procAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
}
func killTree(pid int) error { return native("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid)).Run() }
func lockInstance(f *os.File) (bool, error) {
	var position syscall.Overlapped
	position.OffsetHigh = 1
	ok, _, err := kernel.NewProc("LockFileEx").Call(f.Fd(), 3, 0, 1, 0, uintptr(unsafe.Pointer(&position)))
	if ok != 0 {
		return true, nil
	}
	if errors.Is(err, syscall.Errno(33)) {
		return false, nil
	}
	return false, err
}
