// Package timerres pins the system timer to its finest period for the length
// of a run.
//
// Windows wakes a waiting thread on a timer whose period defaults to 15.625ms.
// The Go runtime raises that to 1ms while it has timers of its own to service
// and releases it again whenever the process goes briefly idle, so the period
// in force during a measurement is decided by unrelated activity on the
// machine. A measurement that straddles a release sees requests wait for the
// next 15.625ms tick: throughput falls by a factor of three or more, tail
// latency lands exactly on the tick, and CPU time per request does not move at
// all, because the work per request is unchanged and only the waiting grew.
//
// Left alone this makes the same framework measure anywhere between 40k and
// 180k req/s on one machine within a single run, which is a wider gap than any
// difference between the frameworks being compared.
package timerres

import (
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	winmm         = windows.NewLazySystemDLL("winmm.dll")
	procTimeBegin = winmm.NewProc("timeBeginPeriod")
	procTimeEnd   = winmm.NewProc("timeEndPeriod")

	ntdll          = windows.NewLazySystemDLL("ntdll.dll")
	procQueryTimer = ntdll.NewProc("NtQueryTimerResolution")
)

// Pin requests the finest timer period the system offers and returns a function
// that releases it. The period is a system-wide setting held at the finest
// value any process has asked for, so the orchestrator holding it also covers
// the server and load generator processes it spawns.
func Pin() func() {
	if r, _, _ := procTimeBegin.Call(1); r != 0 {
		return func() {}
	}
	return func() { procTimeEnd.Call(1) }
}

// Current reports the timer period currently in force. ok is false when it
// cannot be read.
func Current() (period time.Duration, ok bool) {
	var minRes, maxRes, curRes uint32
	r, _, _ := procQueryTimer.Call(
		uintptr(unsafe.Pointer(&minRes)),
		uintptr(unsafe.Pointer(&maxRes)),
		uintptr(unsafe.Pointer(&curRes)),
	)
	if r != 0 || curRes == 0 {
		return 0, false
	}
	return time.Duration(curRes) * 100 * time.Nanosecond, true
}
