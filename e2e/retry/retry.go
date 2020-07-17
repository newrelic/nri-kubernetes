package retry

import (
	"fmt"
	"time"
)

type RetriableFunc func() error
type OnRetryFunc func(err error)

type config struct {
	delay   time.Duration
	timeout time.Duration
	onRetry OnRetryFunc
}

type Option func(*config)

func Delay(delay time.Duration) Option {
	return func(c *config) {
		c.delay = delay
	}
}

func Timeout(timeout time.Duration) Option {
	return func(c *config) {
		c.timeout = timeout
	}
}

func OnRetry(fn OnRetryFunc) Option {
	return func(c *config) {
		c.onRetry = fn
	}
}

func Do(fn RetriableFunc, opts ...Option) error {
	var nRetries int
	c := &config{
		delay:   2 * time.Second,
		timeout: 2 * time.Minute,
		onRetry: func(err error) {},
	}
	for _, opt := range opts {
		opt(c)
	}
	tRetry := time.NewTicker(c.delay)
	tTimeout := time.NewTicker(c.timeout)
	for {
		lastError := fn()
		if lastError == nil {
			return nil
		}

		select {
		case <-tTimeout.C:
			tRetry.Stop()
			tTimeout.Stop()
			return fmt.Errorf("timeout reached, %d retries executed. last error: %s", nRetries, lastError)
		case <-tRetry.C:
			c.onRetry(lastError)
			nRetries++
		}
	}
}
