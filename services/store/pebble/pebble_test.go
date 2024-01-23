// SPDX-FileCopyrightText: © 2024 Siemens AG
// SPDX-License-Identifier: MIT

package pebble_test

import (
	"testing"

	"github.com/coatyio/dda/services/store/pebble"
	"github.com/stretchr/testify/assert"
)

func TestPebble(t *testing.T) {
	bnd := &pebble.PebbleBinding{}
	key := bnd.KeyUpperBound(nil)
	assert.Equal(t, []byte(nil), key)
	key = bnd.KeyUpperBound([]byte{})
	assert.Equal(t, []byte(nil), key)
	key = bnd.KeyUpperBound([]byte{255})
	assert.Equal(t, []byte(nil), key)
	key = bnd.KeyUpperBound([]byte{255, 255})
	assert.Equal(t, []byte(nil), key)
	key = bnd.KeyUpperBound([]byte{1, 255})
	assert.Equal(t, []byte{2}, key)
	key = bnd.KeyUpperBound([]byte{1, 254})
	assert.Equal(t, []byte{1, 255}, key)
}
