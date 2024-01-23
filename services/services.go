// SPDX-FileCopyrightText: © 2023 Siemens AG
// SPDX-License-Identifier: MIT

// Package services provides common functionality to be reused by service
// implementations.
package services

import (
	"context"
	"fmt"
	"time"
)

type retryable interface {
	Retryable() bool
}

type retryableError struct {
	error
}

func (r *retryableError) Error() string {
	return "retryable error: " + r.error.Error()
}

func (r *retryableError) Retryable() bool {
	return true
}

// RetryableErrorf returns a retryable error that formats according to the given
// format specifier.
//
// Use the function IsRetryable(error) to check whether a given error is
// retryable or not.
func RetryableErrorf(format string, a ...any) error {
	return &retryableError{error: fmt.Errorf(format, a...)}
}

// NewRetryableError creates and returns a new error from the given error
// indicating that it is retryable. Returns nil if the given error is nil.
//
// Use the function IsRetryable(error) to check whether a given error is
// retryable or not.
func NewRetryableError(err error) error {
	if err == nil {
		return nil
	}
	return &retryableError{error: err}
}

// IsRetryable indicates whether the operation causing the error may be retried
// with a backoff potentially. For example, a retryable error may be caused by
// an operation that times out or due to a temporary unavailability.
//
// IsRetryable returns false if the error is nil or if the error is caused by an
// operation that cannot be retried due to failed preconditions, invalid
// arguments, or on any other grounds. Otherwise, true is returned.
func IsRetryable(err error) bool {
	rerr, ok := err.(retryable)
	return ok && rerr.Retryable()
}

// ErrorRetryable gets the underlying error of a retryable error. If the given
// error err is not retryable, it is returned.
func ErrorRetryable(err error) error {
	if rerr, ok := err.(*retryableError); ok {
		return rerr.error
	}
	return err
}

// Backoff represents a capped exponential backoff strategy for an operation
// that yields a retryable error (see [RetryWithBackoff]).
//
// Note that you can also define a linear backoff strategy by setting Cap to
// the same value as Base.
type Backoff struct {
	Base time.Duration // base delay of the first retry attempt
	Cap  time.Duration // if nonzero, maximum delay between retry attempts
	Max  int           // if nonzero, maximum number of retry attempts
}

// RetryWithBackoff synchronously retries the given operation according to the
// given backoff strategy as long as as it yields a retryable error.
//
// RetryWithBackoff returns nil if the operation succeeds eventually, a
// non-retryable error if it fails eventually (also due to cancelation of given
// context) or a retryable error to indicate that the maximum number of retry
// attempts has been reached.
//
// The given operation is passed a counter that, starting with 1, sums up the
// number of invocations, including the current one.
func RetryWithBackoff(ctx context.Context, b Backoff, op func(cnt int) error) error {
	retry := 0
	base := b.Base

	for {
		if err := op(retry + 1); !IsRetryable(err) {
			return err
		}
		retry++
		if b.Max > 0 && retry > b.Max {
			return RetryableErrorf("reached maximum number of retry attempts")
		}
		d := base
		if b.Cap > 0 {
			d = min(b.Cap, base)
		}
		timer := time.NewTimer(d)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C // drain channel
			}
			return ctx.Err()
		case <-timer.C:
			timer.Stop()
		}
		if d == base {
			base *= 2
		}
	}
}
