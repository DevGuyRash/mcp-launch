//go:build !windows

package main

import (
    "os"
    "syscall"
    "time"
)

func newSysProcAttrForGroup() *syscall.SysProcAttr {
    return &syscall.SysProcAttr{Setpgid: true}
}

func killPIDOS(pid int) error {
    if pid <= 0 { return nil }
    pr, err := os.FindProcess(pid)
    if err == nil {
        _ = pr.Signal(syscall.SIGTERM)
        time.Sleep(300 * time.Millisecond)
    }
    return nil
}

func killProcessGroupOS(pid int) error {
    if pid <= 0 { return nil }
    _ = syscall.Kill(-pid, syscall.SIGTERM)
    time.Sleep(800 * time.Millisecond)
    _ = syscall.Kill(-pid, syscall.SIGKILL)
    return nil
}
