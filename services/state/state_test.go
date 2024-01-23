// SPDX-FileCopyrightText: © 2024 Siemens AG
// SPDX-License-Identifier: MIT

package state_test

import (
	"testing"

	"github.com/coatyio/dda/services/state"
	"github.com/stretchr/testify/assert"
)

func TestState(t *testing.T) {
	api, err := state.New("foo")
	assert.Nil(t, api)
	assert.Error(t, err)

	api, err = state.New("raft")
	assert.NotNil(t, api)
	assert.NoError(t, err)
}
