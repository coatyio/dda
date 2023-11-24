// SPDX-FileCopyrightText: © 2023 Siemens AG
// SPDX-License-Identifier: MIT

package mqtt5_test

import (
	"testing"

	"github.com/coatyio/dda/services/com/mqtt5"
	"github.com/stretchr/testify/assert"
)

func TestMqtt5(t *testing.T) {
	bnd := &mqtt5.Mqtt5Binding{}
	assert.Equal(t, "", bnd.ClientId())
}
