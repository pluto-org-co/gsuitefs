package httputils

import (
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type RetryTransport struct {
	once        sync.Once
	Parent      http.RoundTripper
	MaxAttempts int
	MinSleep    time.Duration
}

func NewRetryTransport(parent http.RoundTripper, maxAttempts int, minSleep time.Duration) (rt *RetryTransport) {
	return &RetryTransport{
		Parent:      parent,
		MaxAttempts: maxAttempts,
		MinSleep:    minSleep,
	}
}

func (r *RetryTransport) RoundTrip(req *http.Request) (res *http.Response, err error) {
	r.once.Do(func() {
		if r.Parent == nil {
			r.Parent = http.DefaultTransport
		}
	})

	for attempt := range r.MaxAttempts {
		res, err = r.Parent.RoundTrip(req)
		if err == nil {
			return res, nil
		}

		if res == nil {
			return nil, fmt.Errorf("failed to execute request: %w", err)
		}

		if res.StatusCode != http.StatusTooManyRequests {
			return res, fmt.Errorf("failed to execute request: %w", err)
		}

		time.Sleep((1 + time.Duration(attempt)) * r.MinSleep)
	}
	return nil, errors.New("max attempts exceeded")
}

var _ http.RoundTripper = (*RetryTransport)(nil)
