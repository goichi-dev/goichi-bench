//go:build !windows

package clock

// Everywhere except Windows the runtime's own monotonic clock is already fine
// enough to time a single loopback request.

import "time"

var base = time.Now()

// Now returns an opaque monotonic reading.
func Now() int64 { return int64(time.Since(base)) }

// Since returns the time elapsed since a reading taken by Now.
func Since(start int64) time.Duration { return time.Duration(Now() - start) }

// Resolution reports the smallest interval the clock can distinguish.
func Resolution() time.Duration { return time.Nanosecond }
