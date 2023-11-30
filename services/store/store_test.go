// SPDX-FileCopyrightText: © 2023 Siemens AG
// SPDX-License-Identifier: MIT

package store_test

import (
	"testing"

	"github.com/coatyio/dda/services/store"
	"github.com/stretchr/testify/assert"
)

func TestStore(t *testing.T) {
	api, err := store.New("foo")
	assert.Nil(t, api)
	assert.Error(t, err)

	api, err = store.New("pebble")
	assert.NotNil(t, api)
	assert.NoError(t, err)
}
