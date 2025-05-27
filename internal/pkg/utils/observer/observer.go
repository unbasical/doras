package observer

import "time"

// IntervalObserver defines a generic structure that observes a value at a fixed interval and executes a function on it.
// The observation continues until a stop signal is received from a shared channel or an error occurs in the function.
type IntervalObserver[T any] struct {
	Interval   time.Duration
	F          func(T) error
	Observable T
}

// Observe starts observing the Observable at regular intervals, executing the specified function until stopped or an error occurs.
// The observation loop terminates when the stop channel receives a signal or the function returns an error.
func (o *IntervalObserver[T]) Observe(stop <-chan any) error {
	for {
		if err := o.F(o.Observable); err != nil {
			return err
		}
		select {
		case <-stop:
			return nil
		case <-time.After(o.Interval):
			continue
		}
	}
}
