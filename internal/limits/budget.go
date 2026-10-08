package limits

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/felix-xqs/panbridge/internal/metrics"
)

// Budget cooperatively targets a fraction of one logical core, shared across
// phases. It is an approximate average, not an OS-enforced instantaneous cap.
// Wait is called serially by the scheduler; Budget is not a concurrent worker pool.
type Budget struct {
	fraction float64
	lastCPU  float64
	lastTime time.Time
	sampled  bool
}

func NewBudget(percent float64) (*Budget, error) {
	if math.IsNaN(percent) || math.IsInf(percent, 0) || percent < 0 || percent > 100 || (percent > 0 && percent < 1) {
		return nil, fmt.Errorf("cpu-budget 必须为 0 或 1 到 100，0 表示关闭 CPU 节流")
	}
	if percent == 0 {
		return nil, nil
	}
	cpu, ok := metrics.CPUSeconds()
	return &Budget{fraction: percent / 100, lastCPU: cpu, lastTime: time.Now(), sampled: ok}, nil
}

func (b *Budget) Wait(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if b == nil {
		return nil
	}
	now := time.Now()
	elapsed := now.Sub(b.lastTime)
	cpu, ok := metrics.CPUSeconds()
	work := elapsed.Seconds() // Conservative wall-time fallback on other platforms.
	if ok && b.sampled {
		work = math.Max(0, cpu-b.lastCPU)
	}
	needed := time.Duration(work / b.fraction * float64(time.Second))
	if delay := needed - elapsed; delay > 0 {
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	b.lastCPU, b.sampled = metrics.CPUSeconds()
	b.lastTime = time.Now()
	return ctx.Err()
}
