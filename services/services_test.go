// SPDX-FileCopyrightText: © 2023 Siemens AG
// SPDX-License-Identifier: MIT

package services_test

import (
	"fmt"
	"testing"

	"github.com/coatyio/dda/services"
	"github.com/stretchr/testify/assert"
)

func TestRetryableError(t *testing.T) {
	assert.Nil(t, services.NewRetryableError(nil))

	rerr := services.NewRetryableError(fmt.Errorf("foo"))
	assert.Error(t, rerr)
	assert.Equal(t, "retryable error: foo", rerr.Error())
	assert.True(t, services.IsRetryable(rerr))
	assert.True(t, services.IsRetryable(services.RetryableErrorf("foo")))
	assert.False(t, services.IsRetryable(fmt.Errorf("foo")))
	assert.False(t, services.IsRetryable(nil))
}
