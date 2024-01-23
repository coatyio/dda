//go:build testing

// SPDX-FileCopyrightText: © 2023 Siemens AG
// SPDX-License-Identifier: MIT

package dda

// This file provides getters to access non-exposed Dda fields for testing.

import (
	comapi "github.com/coatyio/dda/services/com/api"
)

// ComApi gets the communication API of a Dda. Accessible for testing only.
func (d *Dda) ComApi() comapi.Api {
	return d.comApi
}
