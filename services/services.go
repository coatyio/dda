// SPDX-FileCopyrightText: © 2023 Siemens AG
// SPDX-License-Identifier: MIT

// Package services provides common functionality to be reused by service
// implementations.
package services

import "fmt"

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

// IsRetryable indicates whether the error is caused by an operation that timed
// out due to a temporary unavailability and may be retried with a backoff
// potentially. IsRetryable returns false if the error is nil or if the error is
// caused by an operation that cannot be retried due to failed preconditions,
// invalid arguments, or on any other grounds.
func IsRetryable(err error) bool {
	rerr, ok := err.(retryable)
	return ok && rerr.Retryable()
}
