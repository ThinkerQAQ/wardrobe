//go:build windows

package main

import (
	"os/exec"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Closing the private Job Object kills the Tunnel child process tree even
// when the local supervisor is terminated unexpectedly.
func mcpLaunchChild(cmd *exec.Cmd) (func() error, error) {
	mcpHideProcess(cmd)
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, err
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		windows.CloseHandle(job)
		return nil, err
	}
	if err = cmd.Start(); err != nil {
		windows.CloseHandle(job)
		return nil, err
	}
	handle, openErr := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
	if openErr == nil {
		openErr = windows.AssignProcessToJobObject(job, handle)
		windows.CloseHandle(handle)
	}
	if openErr != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		windows.CloseHandle(job)
		return nil, openErr
	}
	var once sync.Once
	var closed error
	return func() error { once.Do(func() { closed = windows.CloseHandle(job) }); return closed }, nil
}
