// SPDX-FileCopyrightText: © 2023 Siemens AG
// SPDX-License-Identifier: MIT

// Package apis provides an interface that defines common operations to be
// implemented by each public client API server.
package apis

import "github.com/coatyio/dda/config"

// ApiServer is an interface that represents common methods that must be
// implemented by each server realizing a public client API.
type ApiServer interface {

	// Open starts the server to serve client API requests.
	//
	// Upon successful startup, nil is returned. If listening on the configured
	// server address fails, an error is returned.
	Open(cfg *config.Config) error

	// Close gracefully stops the server to no longer serve client API requests.
	Close()
}
