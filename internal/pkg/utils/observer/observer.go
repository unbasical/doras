package observer

import "time"

type IntervalObserver[T any] struct {
	Interval   time.Duration
	F          func(T) error
	Observable T
}

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
