// Package clock provides a monotonic clock fine-grained enough to time a
// single loopback request.
//
// On Windows time.Now() has a granularity of about half a millisecond, which is
// several times longer than the request being measured: latency percentiles
// taken from it collapse into "0ms" and "1ms". QueryPerformanceCounter is used
// instead, which ticks in the tens of nanoseconds.
package clock

import (
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32    = windows.NewLazySystemDLL("kernel32.dll")
	procCounter = kernel32.NewProc("QueryPerformanceCounter")
	procFreq    = kernel32.NewProc("QueryPerformanceFrequency")

	// nsPerTick is zero when the performance counter is unavailable, in which
	// case the coarse clock is used instead.
	nsPerTick float64
)

func init() {
	var freq int64
	r, _, _ := procFreq.Call(uintptr(unsafe.Pointer(&freq)))
	if r == 0 || freq == 0 {
		return
	}
	nsPerTick = float64(time.Second) / float64(freq)
}

// Now returns an opaque monotonic reading.
func Now() int64 {
	if nsPerTick == 0 {
		return time.Now().UnixNano()
	}
	var counter int64
	if r, _, _ := procCounter.Call(uintptr(unsafe.Pointer(&counter))); r == 0 {
		return time.Now().UnixNano()
	}
	return counter
}

// Since returns the time elapsed since a reading taken by Now.
func Since(start int64) time.Duration {
	if nsPerTick == 0 {
		return time.Duration(time.Now().UnixNano() - start)
	}
	return time.Duration(float64(Now()-start) * nsPerTick)
}

// Resolution reports the smallest interval the clock can distinguish.
func Resolution() time.Duration {
	if nsPerTick == 0 {
		return 500 * time.Microsecond
	}
	return time.Duration(nsPerTick)
}
