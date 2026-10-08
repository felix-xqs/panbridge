package limits

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"
)

func TestBudgetValidationAndDisable(t *testing.T) {
	for _, percent := range []float64{-1, 0.00001, 101, math.NaN(), math.Inf(1)} {
		if _, err := NewBudget(percent); err == nil {
			t.Fatalf("invalid budget accepted: %v", percent)
		}
	}
	budget, err := NewBudget(0)
	if err != nil || budget != nil {
		t.Fatalf("zero budget should disable throttling: %v, %v", budget, err)
	}
	if err := budget.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestCancellationInterruptsBudgetWait(t *testing.T) {
	budget, err := NewBudget(25)
	if err != nil {
		t.Fatal(err)
	}
	// The fallback models a second of work and must sleep about three seconds;
	// cancellation should stop that wait, not defer until the budget replenishes.
	budget.sampled = false
	budget.lastTime = time.Now().Add(-time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := budget.Wait(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected cancellation during throttling, got %v", err)
	}
}
