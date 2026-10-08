package metrics

import (
	"runtime"
	"time"
)

type Recorder struct {
	start     time.Time
	cpuStart  float64
	supported bool
}

type Summary struct {
	ElapsedSeconds float64  `json:"elapsed_seconds"`
	CPUSeconds     *float64 `json:"cpu_seconds,omitempty"`
	CPUPercent     *float64 `json:"cpu_single_core_percent,omitempty"`
	PeakRSSBytes   uint64   `json:"process_peak_rss_bytes,omitempty"`
	HeapBytes      uint64   `json:"heap_bytes"`
}

// CPUSeconds includes all process threads, including SQLite and Go GC work.
func CPUSeconds() (float64, bool) {
	cpu, _, ok := processUsage()
	return cpu, ok
}

func Start() Recorder {
	cpu, _, ok := processUsage()
	return Recorder{start: time.Now(), cpuStart: cpu, supported: ok}
}

func (r Recorder) Finish() Summary {
	elapsed := time.Since(r.start).Seconds()
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	result := Summary{ElapsedSeconds: elapsed, HeapBytes: memory.HeapAlloc}
	cpu, peakRSS, ok := processUsage()
	if r.supported && ok {
		seconds := cpu - r.cpuStart
		percent := 0.0
		if elapsed > 0 {
			percent = 100 * seconds / elapsed
		}
		result.CPUSeconds = &seconds
		result.CPUPercent = &percent
		result.PeakRSSBytes = peakRSS
	}
	return result
}
