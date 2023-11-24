// SPDX-FileCopyrightText: © 2023 Siemens AG
// SPDX-License-Identifier: MIT

package api_test

import (
	"testing"

	"github.com/coatyio/dda/config"
	"github.com/coatyio/dda/services/com/api"
	"github.com/stretchr/testify/assert"
)

func TestApi(t *testing.T) {
	t.Run("Scope", func(t *testing.T) {
		scope, err := api.ToScope("")
		if assert.NoError(t, err) {
			assert.Equal(t, "com", string(scope))
		}
		scope, err = api.ToScope("com")
		if assert.NoError(t, err) {
			assert.Equal(t, "com", string(scope))
		}
		scope, err = api.ToScope("sta")
		if assert.NoError(t, err) {
			assert.Equal(t, "sta", string(scope))
		}
		scope, err = api.ToScope("sto")
		if assert.NoError(t, err) {
			assert.Equal(t, "sto", string(scope))
		}
		scope, err = api.ToScope("sdc")
		if assert.NoError(t, err) {
			assert.Equal(t, "sdc", string(scope))
		}
		_, err = api.ToScope("foo")
		if assert.Error(t, err) {
			assert.EqualError(t, err, `unknown scope foo`)
		}
	})
	t.Run("ValidateName", func(t *testing.T) {
		context := []string{"foo", "bar"}
		err := config.ValidateName("", context...)
		assert.Error(t, err)
		for i := 0; i <= 127; i++ {
			err = config.ValidateName(string(rune(i)), context...)
			if (i >= 'a' && i <= 'z') || (i >= 'A' && i <= 'Z') || (i >= '0' && i <= '9') || i == '.' || i == '-' || i == '_' {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		}
	})
}
