//go:build linux

package procstat

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// CPU is the processor time a process has consumed since it started.
type CPU struct {
	User time.Duration
	Sys  time.Duration
}

// Total is the combined user and system time.
func (c CPU) Total() time.Duration { return c.User + c.Sys }

// clockTick is the kernel's USER_HZ. It is 100 on every mainstream Linux
// build; reading it properly needs cgo, and being wrong here only scales a
// diagnostic number.
const clockTick = 100

// Read returns the CPU time consumed so far by the given process, from
// /proc/<pid>/stat.
func Read(pid int) (CPU, bool) {
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return CPU{}, false
	}
	// The executable name is parenthesised and may contain spaces, so fields
	// are counted from the closing parenthesis.
	i := strings.LastIndexByte(string(b), ')')
	if i < 0 {
		return CPU{}, false
	}
	fields := strings.Fields(string(b)[i+1:])
	// After the state field, utime is field 11 and stime field 12.
	if len(fields) < 13 {
		return CPU{}, false
	}
	utime, err1 := strconv.ParseInt(fields[11], 10, 64)
	stime, err2 := strconv.ParseInt(fields[12], 10, 64)
	if err1 != nil || err2 != nil {
		return CPU{}, false
	}
	perTick := time.Second / clockTick
	return CPU{
		User: time.Duration(utime) * perTick,
		Sys:  time.Duration(stime) * perTick,
	}, true
}
