// SPDX-FileCopyrightText: © 2023 Siemens AG
// SPDX-License-Identifier: MIT

// Package com provides a factory function that creates a specific communication
// binding from a given communication protocol.
package com

import (
	"fmt"

	"github.com/coatyio/dda/services/com/api"
	"github.com/coatyio/dda/services/com/mqtt5"
)

// New creates and initializes a new protocol-specific communication binding as
// configured by the given communication protocol.
//
// Returns the new communication binding as a *Api interface. An error is
// returned if the given communication protocol is not supported.
func New(protocol string) (*api.Api, error) {
	var api api.Api
	switch protocol {
	case "mqtt5":
		api = &mqtt5.Mqtt5Binding{}
	default:
		// TODO Whensoever Go plugin mechanism is really cross platform, use it
		// to look up communication bindings that are provided externally.
		return nil, fmt.Errorf("communication protocol %s: not supported", protocol)
	}

	return &api, nil
}
