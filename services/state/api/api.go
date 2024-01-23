// SPDX-FileCopyrightText: © 2024 Siemens AG
// SPDX-License-Identifier: MIT

// Package api provides the distributed state synchronization API. This API is
// implemented by state synchronization bindings that use specific underlying
// consensus protocols, like Raft or Paxos, that guarantee strong consistency on
// a replicated state. State is represented by a key-value data model where keys
// are strings and values are opaque binary blobs.
package api

import (
	"context"

	"github.com/coatyio/dda/config"
	comapi "github.com/coatyio/dda/services/com/api"
)

// State represents the state of a replicated key-value store.
type State map[string][]byte

// InputOp defines all input operations available on replicated state.
type InputOp int

const (
	InputOpUndefined InputOp = iota // no operation, should never be specified and will be ignored
	InputOpSet                      // set or change the value of a key
	InputOpDelete                   // delete a key-value pair
)

// Api is an interface for the distributed state synchronization API to be
// provided by a single DDA sidecar or instance.
//
// This API enables application components to share distributed state with each
// other by use of an underlying consensus protocol, like Raft or Paxos, that
// guarantees strong consistency.
//
// This API maintains a replicated state in a state synchronization group that
// is formed by all associated agents in a DDA cluster that are configured as
// state group members. State changes can be observed and proposed by members of
// the group. Replicated state is represented as a key-value store. Whereas keys
// are strings, values can be any application-specific binary encoding. You may
// partition the overall state into multiple application-specific use cases by
// providing a unique key prefix for each one.
//
// Replicated state can be modified with [ProposeInput] to propose new input
// that should be applied to the replicated state. Applied state changes can be
// observed with [ObserveStateChange].
//
// All members of a state synchronization group can monitor the lifecycle of
// each other by [ObserveMembershipChange] emitting new membership change
// information whenever a member is joining or leaving the group.
//
// Note that Api implementations are meant to be thread-safe and individual Api
// interface methods may be run on concurrent goroutines.
type Api interface {

	// Open connects a member to the state synchronization group with the
	// supplied DDA configuration and pub-sub communication API.
	//
	// Upon successful connection or if already connected, nil is returned. If
	// connection fails eventually, a binding-specific error is returned.
	//
	// If this method is called after a crash and the associated state group
	// member was connected to the state synchronization group before the crash,
	// persisted data will be used to restore the state group member and to
	// reconnect to the state synchronization group.
	//
	// A state group member configured with option "bootstrap" set to true will
	// create a new state synchronization group on opening. Trying to connect a
	// state group member with this option set to false will only acknowledge
	// after a state synchronization group has been created and a majority of
	// the existing members has agreed on letting this member join.
	Open(cfg *config.Config, com comapi.Api) error

	// Close disconnects this member from the state synchronization group.
	//
	// The state group member is removed from its group but persisted state is
	// preserved to be restored when the member reconnects later.
	Close()

	// ProposeInput proposes the given input to the state synchronization group.
	// It tries to add the input to a log replicated by all members. If the
	// input is accepted into the log it counts as committed and is applied to
	// each member's key-value store. The resulting state of the key-value store
	// after the proposed input has been applied, if it ever gets applied, can
	// be observed with [ObserveStateChange].
	//
	// A majority of members need to give their consent before the given input
	// is accepted into the log. This might take indefinitely if no majority can
	// be reached, e.g. if too many members have crashed and cannot recover. In
	// this case the call never returns unless you specify a deadline/timeout
	// with the call.
	//
	// If the operation fails due to a non-retryable error, such as a closing or
	// closed binding, a binding-specific error is returned.
	ProposeInput(ctx context.Context, in *Input) error

	// ObserveStateChange emits new input that has been proposed and applied to
	// the replicated key-value store as soon as the change becomes known to the
	// local state group member. A state change is triggered whenever a new
	// input of type [InputOpSet] or [InputOpDelete] is committed. Upon
	// invocation, synthetic [InputOpSet] state changes are triggered to
	// reproduce the current key-value entries of the replicated state. Emitted
	// input can be safely mutated.
	//
	// To stop receiving state changes, the given context should be canceled
	// explicitly or a deadline/timeout should be specified from the very start.
	//
	// The returned channel should be continuously receiving data in a
	// non-blocking way. The channel is closed once the context is canceled.
	//
	// If the operation fails, a binding-specific error is returned along with a
	// nil channel.
	ObserveStateChange(ctx context.Context) (<-chan Input, error)

	// ObserveMembershipChange emits state membership information on every state
	// membership change as soon as the update becomes known to the local
	// member. State membership changes are triggered whenever a member joins or
	// leaves the state synchronization group.
	//
	// To stop receiving membership changes, the given context should be
	// canceled explicitly or a deadline/timeout should be specified from the
	// very start.
	//
	// The returned channel should be continuously receiving data in a
	// non-blocking way. The channel is closed once the context is canceled.
	//
	// If the operation fails, a binding-specific error is returned along with a
	// nil channel.
	ObserveMembershipChange(ctx context.Context) (<-chan MembershipChange, error)
}

// Input represents an operation proposed by [ProposeInput]. Input is applied to
// the replicated state represented as a key-value store and finally emitted as
// a state change by [ObserveStateChange].
type Input struct {

	// Operation applied on given key ([InputOpDelete]) or key-value pair
	// ([InputOpSet]) (required).
	Op InputOp

	// Key on which given operation is applied (required).
	//
	// If not present, the default key "" is used.
	Key string

	// Value that is set or changed (required for [InputOpSet] operation only).
	//
	// Value is represented as any application-specific binary encoding.
	// Encoding and decoding of the binary data is left to the user of the API
	// interface. If not present but required, the value which is set represents
	// an empty byte slice.
	Value []byte
}

// MembershipChange represents information about a single member joining or
// leaving the state synchronization group.
type MembershipChange struct {

	// Unique ID of member usually represented as a UUID v4; corresponds with
	// the configured DDA identity id.
	Id string

	// Determines whether the member has joined (true) or left (false) the state
	// synchronization group.
	Joined bool
}
