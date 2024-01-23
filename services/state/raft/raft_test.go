// SPDX-FileCopyrightText: © 2024 Siemens AG
// SPDX-License-Identifier: MIT

package raft_test

import (
	"testing"

	"github.com/coatyio/dda/services/state/raft"
	"github.com/stretchr/testify/assert"
)

func TestRaft(t *testing.T) {
	bnd := &raft.RaftBinding{}
	assert.Equal(t, "", bnd.NodeId())
}
