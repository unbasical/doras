package observer

import (
	"errors"
	"testing"
	"time"
)

// Test that if the stop channel is already closed, Observe calls f exactly once and returns nil.
func TestObserveStopsImmediately(t *testing.T) {
	calls := 0
	o := &IntervalObserver[int]{
		Interval: time.Hour, // large so we don't actually sleep
		F: func(i int) error {
			calls++
			return nil
		},
		Observable: 42,
	}

	stop := make(chan any)
	close(stop)

	if err := o.Observe(stop); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call to f, got %d", calls)
	}
}

// Test that if f returns an error on the 2nd invocation, Observe returns it immediately
func TestObserveStopsOnError(t *testing.T) {
	calls := 0
	testErr := errors.New("boom")
	o := &IntervalObserver[int]{
		Interval: 100 * time.Millisecond, // interval large so select will hit time.After only after error
		F: func(i int) error {
			calls++
			if calls == 2 {
				return testErr
			}
			return nil
		},
		Observable: 7,
	}

	stop := make(chan any)
	err := o.Observe(stop)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !errors.Is(err, testErr) {
		t.Fatalf("expected error %v, got %v", testErr, err)
	}
	if calls != 2 {
		t.Errorf("expected f to be called twice, got %d", calls)
	}
}

// Test that f itself can close the stop channel and cause Observe to return nil
func TestObserveStopClosedInF(t *testing.T) {
	calls := 0
	stop := make(chan any)

	o := &IntervalObserver[struct{}]{
		Interval: 100 * time.Millisecond,
		F: func(_ struct{}) error {
			calls++
			// on the third call, signal to stop
			if calls == 3 {
				close(stop)
			}
			return nil
		},
		Observable: struct{}{},
	}

	if err := o.Observe(stop); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if calls != 3 {
		t.Errorf("expected f to be called 3 times, got %d", calls)
	}
}
