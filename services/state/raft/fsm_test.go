// SPDX-FileCopyrightText: © 2024 Siemens AG
// SPDX-License-Identifier: MIT

package raft_test

import (
	"bytes"
	"io"
	"testing"

	"github.com/coatyio/dda/services/state/api"
	"github.com/coatyio/dda/services/state/raft"
	hraft "github.com/hashicorp/raft"
	"github.com/stretchr/testify/assert"
)

func newLogCommand(op api.InputOp, key string, value []byte) (*hraft.Log, error) {
	var in api.Input
	switch op {
	case api.InputOpSet:
		if value == nil {
			in = api.Input{Op: op, Key: key}
		} else {
			in = api.Input{Op: op, Key: key, Value: value}
		}
	default:
		in = api.Input{Op: op, Key: key}

	}
	b, err := raft.EncodeMsgPack(in)
	if err != nil {
		return nil, err
	}
	return &hraft.Log{
		Type: hraft.LogCommand,
		Data: b,
	}, nil
}

func TestFsm(t *testing.T) {
	fsm := raft.NewRaftFsm()
	assert.NotNil(t, fsm)

	t.Run("invalid log command", func(t *testing.T) {
		log := &hraft.Log{Type: hraft.LogCommand, Data: []byte("foo")}
		if r, ok := fsm.Apply(log).(error); !ok {
			assert.Fail(t, "expect error result")
		} else {
			assert.Error(t, r)
		}
	})

	t.Run("log operation type unknown", func(t *testing.T) {
		log, err := newLogCommand(api.InputOp(3), "baz", []byte("baz"))
		assert.NotNil(t, log)
		assert.NoError(t, err)
		if r, ok := fsm.Apply(log).(error); !ok {
			assert.Fail(t, "expect error result")
		} else {
			assert.EqualError(t, r, "unknown input operation in Raft log entry: 3")
		}
	})

	t.Run("log operation type undefined", func(t *testing.T) {
		log, err := newLogCommand(api.InputOpUndefined, "undefined", []byte("undefined"))
		assert.NotNil(t, log)
		assert.NoError(t, err)
		assert.Nil(t, fsm.Apply(log))
		assert.EqualValues(t, api.State(map[string][]byte{}), fsm.State())
	})

	t.Run("log operation type set without value, i.e. nil", func(t *testing.T) {
		log, err := newLogCommand(api.InputOpSet, "foo", nil)
		assert.NotNil(t, log)
		assert.NoError(t, err)
		assert.Nil(t, fsm.Apply(log))
		assert.EqualValues(t, api.State(map[string][]byte{"foo": []byte("")}), fsm.State())
	})

	t.Run("log operation type set with []byte(nil) value", func(t *testing.T) {
		log, err := newLogCommand(api.InputOpSet, "foo", []byte(nil))
		assert.NotNil(t, log)
		assert.NoError(t, err)
		assert.Nil(t, fsm.Apply(log))
		assert.EqualValues(t, api.State(map[string][]byte{"foo": []byte("")}), fsm.State())
	})

	t.Run("log operation type set foo", func(t *testing.T) {
		log, err := newLogCommand(api.InputOpSet, "foo", []byte("foo"))
		assert.NotNil(t, log)
		assert.NoError(t, err)
		assert.Nil(t, fsm.Apply(log))
		assert.EqualValues(t, api.State(map[string][]byte{"foo": []byte("foo")}), fsm.State())
	})

	t.Run("log operation type set bar", func(t *testing.T) {
		log, err := newLogCommand(api.InputOpSet, "bar", []byte("bar"))
		assert.NotNil(t, log)
		assert.NoError(t, err)
		assert.Nil(t, fsm.Apply(log))
		assert.EqualValues(t, api.State(map[string][]byte{"foo": []byte("foo"), "bar": []byte("bar")}), fsm.State())
	})

	t.Run("log operation type delete bar", func(t *testing.T) {
		log, err := newLogCommand(api.InputOpDelete, "bar", nil)
		assert.NotNil(t, log)
		assert.NoError(t, err)
		assert.Nil(t, fsm.Apply(log))
		assert.EqualValues(t, api.State(map[string][]byte{"foo": []byte("foo")}), fsm.State())
	})

	t.Run("snapshot-restore by persist-release", func(t *testing.T) {
		snap, err := fsm.Snapshot()
		assert.NotNil(t, snap)
		assert.NoError(t, err)

		expectedState := api.State(map[string][]byte{"foo": []byte("foo")})
		assert.EqualValues(t, expectedState, fsm.State())

		sink := newMockSnapshotSink()
		err = snap.Persist(sink)
		assert.NoError(t, err)
		snap.Release()

		// Clear FSM before restoring from snapshot.
		log, _ := newLogCommand(api.InputOpDelete, "foo", nil)
		fsm.Apply(log)
		assert.EqualValues(t, api.State(map[string][]byte{}), fsm.State())

		err = fsm.Restore(io.NopCloser(sink.contents))
		assert.NoError(t, err)
		assert.EqualValues(t, expectedState, fsm.State())
	})

}

// mockSnapshotSink implements [hraft.SnapshotSink] for testing purposes.
type mockSnapshotSink struct {
	meta     hraft.SnapshotMeta
	contents *bytes.Buffer
}

func newMockSnapshotSink() *mockSnapshotSink {
	return &mockSnapshotSink{
		meta:     hraft.SnapshotMeta{ID: "42"},
		contents: &bytes.Buffer{},
	}
}

// Write appends the given bytes to the snapshot contents
func (s *mockSnapshotSink) Write(p []byte) (n int, err error) {
	written, err := s.contents.Write(p)
	s.meta.Size += int64(written)
	return written, err
}

// Close updates the Size and is otherwise a no-op
func (s *mockSnapshotSink) Close() error {
	return nil
}

// ID returns the ID of the SnapshotMeta
func (s *mockSnapshotSink) ID() string {
	return s.meta.ID
}

// Cancel returns successfully with a nil error
func (s *mockSnapshotSink) Cancel() error {
	return nil
}
