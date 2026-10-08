//go:build darwin || linux

package metrics

import (
	"runtime"
	"syscall"
)

func processUsage() (float64, uint64, bool) {
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		return 0, 0, false
	}
	cpu := float64(usage.Utime.Sec+usage.Stime.Sec) + float64(usage.Utime.Usec+usage.Stime.Usec)/1e6
	rss := uint64(usage.Maxrss)
	if runtime.GOOS == "linux" {
		rss *= 1024
	}
	return cpu, rss, true
}
