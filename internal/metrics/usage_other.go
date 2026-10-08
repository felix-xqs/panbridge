//go:build !darwin && !linux && !windows

package metrics

func processUsage() (float64, uint64, bool) { return 0, 0, false }
