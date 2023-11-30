// SPDX-FileCopyrightText: © 2023 Siemens AG
// SPDX-License-Identifier: MIT

// Package store provides a factory function that creates a specific storage
// binding from a given storage engine.
package store

import (
	"fmt"

	"github.com/coatyio/dda/services/store/api"
	"github.com/coatyio/dda/services/store/pebble"
)

// New creates and initializes a new specific storage binding as configured by
// the given storage engine.
//
// Returns the new storage binding as a *Api interface. An error is returned if
// the given storage engine is not supported.
func New(engine string) (*api.Api, error) {
	var api api.Api
	switch engine {
	case "pebble":
		api = &pebble.PebbleBinding{}
	default:
		// TODO Whensoever Go plugin mechanism is really cross platform, use it
		// to look up storage bindings that are provided externally.
		return nil, fmt.Errorf("storage engine %s: not supported", engine)
	}

	return &api, nil
}
