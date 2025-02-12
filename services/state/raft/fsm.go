// SPDX-FileCopyrightText: © 2024 Siemens AG
// SPDX-License-Identifier: MIT

// Package raft exports a Raft finite state machine (FSM) for DDA state members
// to make use of replicated state.
package raft

import (
	"fmt"
	"io"
	"sync"

	"github.com/coatyio/dda/services/state/api"
	hraft "github.com/hashicorp/raft"
)

// RaftFsm implements the [hraft.FSM] interface to model replicated state in the
// form of a key-value dictionary.
type RaftFsm struct {
	mu             sync.Mutex                // protects following fields
	state          api.State                 // key-value pairs
	observers      map[uint64]chan api.Input // observing state change channels
	nextObserverId uint64                    // id of next registered state change channel
}

// NewRaftFsm creates a new Raft FSM that models replicated state in the form of
// a key-value dictionary.
func NewRaftFsm() *RaftFsm {
	return &RaftFsm{state: make(api.State), observers: make(map[uint64]chan api.Input), nextObserverId: 0}
}

// State gets a deep copy of the current key-value pairs of a RaftFsm. The
// returned state can be safely mutated. To be used for testing purposes only.
func (f *RaftFsm) State() api.State {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.cloneState()
}

// AddStateChangeObserver registers the given channel to listen to state changes
// and returns a channel ID that can be used to deregister the channel later.
//
// The channel should continuously receive data on the channel in a non-blocking
// manner to prevent blocking send operations.
func (f *RaftFsm) AddStateChangeObserver(ch chan api.Input) uint64 {
	f.mu.Lock()
	f.nextObserverId++
	f.observers[f.nextObserverId] = ch
	f.mu.Unlock()

	// Emit synthetic inputs reproducing the current key-value pairs.
	go func() {
		// we need to protect all accesses to shared maps
		f.mu.Lock()
		defer f.mu.Unlock()
		for k, v := range f.state {
			ch <- f.cloneInput(&api.Input{Op: api.InputOpSet, Key: k, Value: v})
		}
	}()

	return f.nextObserverId
}

// RemoveStateChangeObserver deregisters the channel with the given channel id.
//
// Note that the channel is not closed, it must be closed by the caller.
func (f *RaftFsm) RemoveStateChangeObserver(chanId uint64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.observers, chanId)
}

// Apply is called once a log entry is committed by a majority of the cluster.
//
// Apply should apply the log to the FSM. Apply must be deterministic and
// produce the same result on all peers in the cluster.
//
// The returned value is returned to the client as the ApplyFuture.Response.
// Note that if Apply returns an error, it will be returned by Response, and not
// by the Error method of ApplyFuture, so it is always important to check
// Response for errors from the FSM. If the given input operation is applied
// successfully, ApplyFuture.Response returns nil.
//
// Apply implements the [hraft.FSM] interface.
func (f *RaftFsm) Apply(entry *hraft.Log) any {
	var input api.Input
	if err := DecodeMsgPack(entry.Data, &input); err != nil {
		return err
	}

	switch input.Op {
	case api.InputOpUndefined:
		return nil
	case api.InputOpSet:
		f.mu.Lock()
		defer f.mu.Unlock()
		f.state[input.Key] = input.Value
		f.sendStateChange(&input)
		return nil
	case api.InputOpDelete:
		f.mu.Lock()
		defer f.mu.Unlock()
		delete(f.state, input.Key)
		f.sendStateChange(&input)
		return nil
	default:
		return fmt.Errorf("unknown input operation in Raft log entry: %v", input.Op)
	}
}

// Snapshot returns an FSMSnapshot used to: support log compaction, to restore
// the FSM to a previous state, or to bring out-of-date followers up to a recent
// log index.
//
// The Snapshot implementation should return quickly, because Apply can not be
// called while Snapshot is running. Generally this means Snapshot should only
// capture a pointer to the state, and any expensive IO should happen as part of
// FSMSnapshot.Persist.
//
// Apply and Snapshot are always called from the same thread, but Apply will be
// called concurrently with FSMSnapshot.Persist. This means the FSM should be
// implemented to allow for concurrent updates while a snapshot is happening.
//
// Snapshot implements the [hraft.FSM] interface.
func (f *RaftFsm) Snapshot() (hraft.FSMSnapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Deep copy state to allow concurrent updates while persisting.
	return &fsmSnapshot{State: f.cloneState()}, nil
}

// Restore is used to restore an FSM from a snapshot. It is not called
// concurrently with any other command. The FSM must discard all previous state
// before restoring the snapshot.
//
// Restore implements the [hraft.FSM] interface.
func (f *RaftFsm) Restore(snapshot io.ReadCloser) error {
	var snap fsmSnapshot
	if err := DecodeMsgPackFromReader(snapshot, &snap); err != nil {
		return err
	}

	f.state = snap.State
	return nil
}

func (f *RaftFsm) sendStateChange(in *api.Input) {
	for _, ch := range f.observers {
		ch <- f.cloneInput(in)
	}
}

func (f *RaftFsm) cloneInput(in *api.Input) api.Input {
	switch in.Op {
	case api.InputOpSet:
		cv := make([]byte, len(in.Value))
		copy(cv, in.Value)
		return api.Input{Op: in.Op, Key: in.Key, Value: cv}
	default:
		return api.Input{Op: in.Op, Key: in.Key}
	}
}

func (f *RaftFsm) cloneState() api.State {
	s := make(api.State, len(f.state))
	// no need for locking, as the lock is held by all callers of this function
	for k, v := range f.state {
		cv := make([]byte, len(v))
		copy(cv, v)
		s[k] = cv
	}
	return s
}

// fsmSnapshot implements the [hraft.FSMSnapshot] interface. It is returned by
// an FSM in response to a Snapshot. It must be safe to invoke FSMSnapshot
// methods with concurrent calls to Apply.
type fsmSnapshot struct {
	State api.State // field must be exported for serialization
}

// Persist should dump all necessary state to the WriteCloser 'sink', and call
// sink.Close() when finished or call sink.Cancel() on error.
//
// Persist implements the [hraft.FSMSnapshot] interface.
func (f *fsmSnapshot) Persist(sink hraft.SnapshotSink) error {
	err := func() error {
		b, err := EncodeMsgPack(f)
		if err != nil {
			return err
		}

		if _, err := sink.Write(b); err != nil {
			return err
		}

		if err := sink.Close(); err != nil {
			return err
		}

		return nil
	}()

	if err != nil {
		_ = sink.Cancel()
		return err
	}

	return nil
}

// Release is invoked when we are finished with the snapshot.
//
// Release implements the [hraft.FSMSnapshot] interface.
func (f *fsmSnapshot) Release() {
}
