// SPDX-FileCopyrightText: © 2023 Siemens AG
// SPDX-License-Identifier: MIT

package services_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/coatyio/dda/services"
	"github.com/stretchr/testify/assert"
)

func TestRetryableError(t *testing.T) {
	assert.Nil(t, services.NewRetryableError(nil))

	nerr := fmt.Errorf("foo")
	assert.Equal(t, nerr, services.ErrorRetryable(nerr))
	rerr := services.NewRetryableError(nerr)
	assert.Error(t, rerr)
	assert.Equal(t, "retryable error: foo", rerr.Error())
	assert.True(t, services.IsRetryable(rerr))
	assert.True(t, services.IsRetryable(services.RetryableErrorf("foo")))
	assert.False(t, services.IsRetryable(fmt.Errorf("foo")))
	assert.False(t, services.IsRetryable(nil))
	assert.Equal(t, nerr, services.ErrorRetryable(rerr))

	invoc := 0
	err := services.RetryWithBackoff(context.Background(), services.Backoff{}, func(cnt int) error { invoc++; return nil })
	assert.NoError(t, err)
	assert.Equal(t, 1, invoc)

	invoc = 0
	nerr = fmt.Errorf("non-retryable")
	err = services.RetryWithBackoff(context.Background(), services.Backoff{}, func(cnt int) error { invoc++; return nerr })
	assert.Error(t, err)
	assert.False(t, services.IsRetryable(err))
	assert.Equal(t, 1, invoc)

	invoc = 0
	err = services.RetryWithBackoff(context.Background(), services.Backoff{Base: 10 * time.Millisecond, Max: 1}, func(cnt int) error { invoc++; return rerr })
	assert.Error(t, err)
	assert.True(t, services.IsRetryable(err))
	assert.Equal(t, 2, invoc)

	invoc = 0
	err = services.RetryWithBackoff(context.Background(), services.Backoff{Base: 10 * time.Millisecond, Max: 2}, func(cnt int) error { invoc++; return rerr })
	assert.Error(t, err)
	assert.True(t, services.IsRetryable(err))
	assert.Equal(t, 3, invoc)

	invoc = 0
	err = services.RetryWithBackoff(context.Background(), services.Backoff{Base: 10 * time.Millisecond}, func(cnt int) error {
		invoc++
		if invoc == 3 {
			return nil
		}
		return rerr
	})
	assert.NoError(t, err)
	assert.False(t, services.IsRetryable(err))
	assert.Equal(t, 3, invoc)

	ctx, cancel := context.WithCancel(context.Background())
	invoc = 0
	err = services.RetryWithBackoff(ctx, services.Backoff{Base: 20 * time.Millisecond}, func(cnt int) error {
		invoc++
		if invoc == 2 {
			cancel()
		}
		return rerr
	})
	assert.Error(t, err)
	assert.Equal(t, ctx.Err(), err)
	assert.False(t, services.IsRetryable(err))
	assert.Equal(t, 2, invoc)
}
