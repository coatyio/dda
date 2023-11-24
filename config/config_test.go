// SPDX-FileCopyrightText: © 2023 Siemens AG
// SPDX-License-Identifier: MIT

package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/coatyio/dda/config"
	"github.com/stretchr/testify/assert"
)

func TestConfig(t *testing.T) {
	t.Run("ValidateNonEmpty", func(t *testing.T) {
		assert.NoError(t, config.ValidateNonEmpty("foo", "context"))
		assert.Error(t, config.ValidateNonEmpty("", "context"))
	})
	t.Run("Verify", func(t *testing.T) {
		cfg := config.New()
		err := cfg.Verify()
		assert.NoError(t, err)
		cfg.Version = "2"
		err = cfg.Verify()
		assert.Error(t, err)
	})
	t.Run("ReadConfig ok", func(t *testing.T) {
		_, err := config.ReadConfig("../dda.yaml")
		assert.NoError(t, err)
	})
	t.Run("ReadConfig missing file", func(t *testing.T) {
		_, err := config.ReadConfig("../xxx.yaml")
		assert.Error(t, err)
	})
	t.Run("ReadConfig invalid YAML", func(t *testing.T) {
		data, err := os.ReadFile("../dda.yaml")
		assert.NoError(t, err)
		data = append(data, 20, 30, 40)
		out := filepath.Join(t.TempDir(), "dda.yaml")
		err = os.WriteFile(out, data, 0)
		assert.NoError(t, err)
		_, err = config.ReadConfig(out)
		assert.Error(t, err)
	})
}
