package backoff

import (
	"errors"
	"github.com/sirupsen/logrus"
	"math/rand"
	"time"
)

// exponentialBackoffWithJitter implements the Strategy interface
type exponentialBackoffWithJitter struct {
	baseDelay      time.Duration // Base delay between retries (e.g., 100ms)
	maxDelay       time.Duration // Maximum delay before giving up
	currentAttempt uint          // Track the current attempt number
	maxAttempt     uint          // Track the current attempt number
	randSource     *rand.Rand    // Random source for jittering
}

// NewExponentialBackoffWithJitter creates a new instance of exponentialBackoffWithJitter
func NewExponentialBackoffWithJitter(baseDelay, maxDelay time.Duration, maxAttempts uint) Strategy {
	// Seed the random number generator for jitter
	source := rand.NewSource(time.Now().UnixNano())
	return &exponentialBackoffWithJitter{
		baseDelay:      baseDelay,
		maxDelay:       maxDelay,
		currentAttempt: 0,
		maxAttempt:     maxAttempts,
		randSource:     rand.New(source),
	}
}

// Wait calculates the next backoff time with exponential backoff and jitter
func (e *exponentialBackoffWithJitter) Wait() error {
	if e.currentAttempt >= e.maxAttempt {
		return errors.New("maximum retries exceeded")
	}
	// Calculate the exponential backoff delay
	delay := e.baseDelay * time.Duration(1<<e.currentAttempt) // 2^attempt * baseDelay

	// Apply jitter by adding a random factor to the delay (between 0 and 1x the delay)
	jitter := time.Duration(e.randSource.Int63n(int64(delay)))
	delay = delay + jitter - (delay / 2) // Apply jitter in both directions

	// Ensure that delay does not exceed the maximum delay
	if delay > e.maxDelay {
		delay = e.maxDelay
	}

	// Sleep for the calculated delay
	logrus.Debugf("Waiting for %v (attempt %d/%d)\n", delay, e.currentAttempt, e.maxAttempt)
	time.Sleep(delay)

	// Increment the attempt number for the next retry
	e.currentAttempt++
	return nil
}

// Strategy is used to avoid flooding the server with requests by adding a backoff
// when clients are waiting for the delta request to be completed.
type Strategy interface {
	Wait() error
}

// DefaultBackoff returns a sensible default Strategy (exponential with an upper bound).
func DefaultBackoff() Strategy {
	const defaultBaseDelay = 50 * time.Millisecond
	const defaultMaxDelay = 1 * time.Minute
	return NewExponentialBackoffWithJitter(defaultBaseDelay, defaultMaxDelay, 10)
}
