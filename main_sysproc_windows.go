//go:build windows

package main

import (
    "fmt"
    "os/exec"
    "syscall"

    winapi "golang.org/x/sys/windows"
)

func newSysProcAttrForGroup() *syscall.SysProcAttr {
    // Create a new process group so taskkill /T terminates the entire tree
    return &syscall.SysProcAttr{CreationFlags: winapi.CREATE_NEW_PROCESS_GROUP}
}

func killPIDOS(pid int) error {
    if pid <= 0 { return nil }
    return exec.Command("taskkill", "/PID", fmt.Sprint(pid), "/T", "/F").Run()
}

func killProcessGroupOS(pid int) error {
    if pid <= 0 { return nil }
    return exec.Command("taskkill", "/PID", fmt.Sprint(pid), "/T", "/F").Run()
}
