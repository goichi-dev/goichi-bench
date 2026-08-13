//go:build !windows && !linux

package procstat

import "time"

// CPU is the processor time a process has consumed since it started.
type CPU struct {
	User time.Duration
	Sys  time.Duration
}

// Total is the combined user and system time.
func (c CPU) Total() time.Duration { return c.User + c.Sys }

// Read is unimplemented on this platform; the CPU-per-request columns are left
// empty rather than filled with a guess.
func Read(int) (CPU, bool) { return CPU{}, false }
