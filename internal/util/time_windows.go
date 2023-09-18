// this file can replace with windows.* package calls in a future golang version.  see:
// https://go-review.googlesource.com/c/sys/+/515915/4/windows/zsyscall_windows.go#4027
package util

import (
	"syscall"
)

var timeBeginPeriod uintptr
var timeEndPeriod uintptr

func init() {
	winmm, err := syscall.LoadLibrary("winmm.dll")
	if err != nil {
		return
	}
	timeBeginPeriod, _ = syscall.GetProcAddress(winmm, "timeBeginPeriod")
	timeEndPeriod, _ = syscall.GetProcAddress(winmm, "timeEndPeriod")
}

func TimeBeginPeriod(period uint32) error {
	r1, _, err := syscall.SyscallN(timeBeginPeriod, uintptr(period))
	if r1 != 0 {
		return err
	}
	return nil
}

func TimeEndPeriod(period uint32) error {
	r1, _, err := syscall.SyscallN(timeEndPeriod, uintptr(period))
	if r1 != 0 {
		return err
	}
	return nil
}
