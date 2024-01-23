// SPDX-FileCopyrightText: © 2024 Siemens AG
// SPDX-License-Identifier: MIT

// Package state provides a factory function that creates a specific state
// synchronization binding from a given consensus protocol.
package state

import (
	"fmt"

	"github.com/coatyio/dda/services/state/api"
	"github.com/coatyio/dda/services/state/raft"
)

// New creates and initializes a new specific state synchronization binding as
// configured by the given consensus protocol.
//
// Returns the new state synchronization binding as a *Api interface. An error
// is returned if the given consensus protool is not supported.
func New(protocol string) (*api.Api, error) {
	var api api.Api
	switch protocol {
	case "raft":
		api = &raft.RaftBinding{}
	default:
		// TODO Whensoever Go plugin mechanism is really cross platform, use it
		// to look up bindings that are provided externally.
		return nil, fmt.Errorf("consensus protocol %s: not supported", protocol)
	}

	return &api, nil
}
