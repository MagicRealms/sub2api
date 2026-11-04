package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestScheduledTestPersistenceContextSurvivesProbeTimeout(t *testing.T) {
	testCtx, cancelTest := context.WithCancel(context.Background())
	cancelTest()
	if !errors.Is(testCtx.Err(), context.Canceled) {
		t.Fatal("probe context should be canceled")
	}

	persistCtx, cancelPersist := scheduledTestPersistenceContext(testCtx)
	defer cancelPersist()
	if err := persistCtx.Err(); err != nil {
		t.Fatalf("persistence context should remain usable: %v", err)
	}
	deadline, ok := persistCtx.Deadline()
	if !ok || time.Until(deadline) > scheduledTestPersistenceTimeout {
		t.Fatal("persistence context needs its own bounded deadline")
	}
}
