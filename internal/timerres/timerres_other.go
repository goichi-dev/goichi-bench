//go:build !windows

package timerres

// Everywhere except Windows a waiting thread is woken by a high-resolution
// timer already, so there is nothing to pin.

import "time"

// Pin is a no-op outside Windows.
func Pin() func() { return func() {} }

// Current reports the timer period currently in force. ok is false when the
// platform has no single such period to report.
func Current() (period time.Duration, ok bool) { return 0, false }
