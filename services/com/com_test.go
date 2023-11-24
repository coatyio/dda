// SPDX-FileCopyrightText: © 2023 Siemens AG
// SPDX-License-Identifier: MIT

package com_test

import (
	"testing"

	"github.com/coatyio/dda/services/com"
	"github.com/stretchr/testify/assert"
)

func TestCom(t *testing.T) {
	api, err := com.New("foo")
	assert.Nil(t, api)
	assert.Error(t, err)

	api, err = com.New("mqtt5")
	assert.NotNil(t, api)
	assert.NoError(t, err)
}
