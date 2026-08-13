// Package procstat reads the CPU time consumed by another process.
//
// It exists because throughput on a loopback interface is often capped by the
// operating system rather than by the server under test: once every framework
// saturates the same network path they all report the same requests per
// second. CPU time per request keeps measuring the framework itself even then.
package procstat

import (
	"time"

	"golang.org/x/sys/windows"
)

// CPU is the processor time a process has consumed since it started.
type CPU struct {
	User time.Duration
	Sys  time.Duration
}

// Total is the combined user and system time.
func (c CPU) Total() time.Duration { return c.User + c.Sys }

// Read returns the CPU time consumed so far by the given process.
func Read(pid int) (CPU, bool) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return CPU{}, false
	}
	defer windows.CloseHandle(h)

	var creation, exit, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(h, &creation, &exit, &kernel, &user); err != nil {
		return CPU{}, false
	}
	return CPU{
		User: filetimeDuration(user),
		Sys:  filetimeDuration(kernel),
	}, true
}

// filetimeDuration converts a FILETIME interval, which counts 100ns ticks.
func filetimeDuration(ft windows.Filetime) time.Duration {
	ticks := int64(ft.HighDateTime)<<32 | int64(ft.LowDateTime)
	return time.Duration(ticks) * 100 * time.Nanosecond
}
