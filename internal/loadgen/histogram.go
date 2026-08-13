package loadgen

import "time"

// histogram is a fixed-resolution latency histogram: 1µs buckets below 1ms,
// 10µs below 10ms and 100µs up to 1s. Anything slower lands in the overflow
// bucket, and the exact maximum is tracked separately. Each worker owns one,
// so recording never touches a shared cache line.
const (
	fineBuckets   = 1000 // 0..1ms at 1µs
	mediumBuckets = 900  // 1ms..10ms at 10µs
	coarseBuckets = 9900 // 10ms..1s at 100µs
	totalBuckets  = fineBuckets + mediumBuckets + coarseBuckets + 1
	overflowIndex = totalBuckets - 1
)

type histogram struct {
	counts   []int64
	count    int64
	max      time.Duration
	requests int64
	errors   int64
	nonOK    int64
	bytesIn  int64
}

func newHistogram() *histogram {
	return &histogram{counts: make([]int64, totalBuckets)}
}

func bucketOf(us int64) int {
	switch {
	case us < 0:
		return 0
	case us < fineBuckets:
		return int(us)
	case us < 10_000:
		return fineBuckets + int((us-1000)/10)
	case us < 1_000_000:
		return fineBuckets + mediumBuckets + int((us-10_000)/100)
	default:
		return overflowIndex
	}
}

// bucketValue is the representative latency of a bucket index.
func bucketValue(i int) time.Duration {
	switch {
	case i < fineBuckets:
		return time.Duration(i) * time.Microsecond
	case i < fineBuckets+mediumBuckets:
		return time.Duration(1000+(i-fineBuckets)*10) * time.Microsecond
	case i < overflowIndex:
		return time.Duration(10_000+(i-fineBuckets-mediumBuckets)*100) * time.Microsecond
	default:
		return time.Second
	}
}

func (h *histogram) record(d time.Duration) {
	h.counts[bucketOf(int64(d/time.Microsecond))]++
	h.count++
	if d > h.max {
		h.max = d
	}
}

func (h *histogram) merge(other *histogram) {
	for i, c := range other.counts {
		h.counts[i] += c
	}
	h.count += other.count
	if other.max > h.max {
		h.max = other.max
	}
}

// quantile returns the latency at q (0..1) by walking the buckets.
func (h *histogram) quantile(q float64) time.Duration {
	if h.count == 0 {
		return 0
	}
	target := int64(float64(h.count) * q)
	var seen int64
	for i, c := range h.counts {
		seen += c
		if seen >= target {
			return bucketValue(i)
		}
	}
	return h.max
}
