// SPDX-FileCopyrightText: © 2024 Siemens AG
// SPDX-License-Identifier: MIT

package raft_test

import (
	"testing"

	"github.com/coatyio/dda/services/state/raft"
	"github.com/coatyio/dda/testdata"
	hraft "github.com/hashicorp/raft"
	"github.com/stretchr/testify/assert"
)

func testRaftLog(idx uint64, data string) *hraft.Log {
	return &hraft.Log{
		Data:  []byte(data),
		Index: idx,
	}
}

func TestRaftStore(t *testing.T) {
	testdata.RunWithSetup(func() {
		rs, err := raft.NewRaftStore("") // with in-memory stores

		assert.NotNil(t, rs)
		assert.NoError(t, err)

		t.Run("provide StableStore LogStore SnapshotStore", func(t *testing.T) {
			var store any = rs
			if s, ok := store.(hraft.StableStore); !ok {
				t.Fatalf("RaftStore does not implement raft.StableStore")
			} else {
				assert.NotNil(t, s)
			}
			if s, ok := store.(hraft.LogStore); !ok {
				t.Fatalf("RaftStore does not implement raft.LogStore")
			} else {
				assert.NotNil(t, s)
			}
			switch s := rs.SnapStore.(type) {
			case hraft.SnapshotStore:
				assert.NotNil(t, s)
			default:
				t.Fatalf("RaftStore does not provide raft.SnapshotStore")
			}
		})

		t.Run("FirstIndex", func(t *testing.T) {
			idx, err := rs.FirstIndex()
			assert.Equal(t, uint64(0), idx) // 0 on empty log
			assert.NoError(t, err)

			logs := []*hraft.Log{
				testRaftLog(1, "log1"),
				testRaftLog(2, "log2"),
				testRaftLog(3, "log3"),
			}
			assert.NoError(t, rs.StoreLogs(logs))

			idx, err = rs.FirstIndex()
			assert.Equal(t, uint64(1), idx)
			assert.NoError(t, err)
		})

		t.Run("DeleteRange", func(t *testing.T) {
			err := rs.DeleteRange(1, 3)
			assert.NoError(t, err)

			idx, err := rs.FirstIndex()
			assert.Equal(t, uint64(0), idx) // 0 on empty log
			assert.NoError(t, err)

			log := &hraft.Log{}
			err = rs.GetLog(1, log)
			assert.Equal(t, hraft.ErrLogNotFound, err)
			log = &hraft.Log{}
			err = rs.GetLog(2, log)
			assert.Equal(t, hraft.ErrLogNotFound, err)
			log = &hraft.Log{}
			err = rs.GetLog(3, log)
			assert.Equal(t, hraft.ErrLogNotFound, err)
		})

		t.Run("LastIndex", func(t *testing.T) {
			idx, err := rs.LastIndex()
			assert.Equal(t, uint64(0), idx) // 0 on empty log
			assert.NoError(t, err)

			log := testRaftLog(5, "log5")
			assert.NoError(t, rs.StoreLog(log))
			log = testRaftLog(6, "log6")
			assert.NoError(t, rs.StoreLog(log))
			log = testRaftLog(7, "log7")
			assert.NoError(t, rs.StoreLog(log))

			idx, err = rs.LastIndex()
			assert.Equal(t, uint64(7), idx)
			assert.NoError(t, err)
		})

		t.Run("GetLog", func(t *testing.T) {
			log := &hraft.Log{}
			err := rs.GetLog(5, log)
			assert.NoError(t, err)
			assert.Equal(t, testRaftLog(5, "log5"), log)

			log = &hraft.Log{}
			err = rs.GetLog(6, log)
			assert.NoError(t, err)
			assert.Equal(t, testRaftLog(6, "log6"), log)

			log = &hraft.Log{}
			err = rs.GetLog(7, log)
			assert.NoError(t, err)
			assert.Equal(t, testRaftLog(7, "log7"), log)

			log = &hraft.Log{}
			err = rs.GetLog(4, log)
			assert.Equal(t, hraft.ErrLogNotFound, err)

			log = &hraft.Log{}
			err = rs.GetLog(1, log)
			assert.Equal(t, hraft.ErrLogNotFound, err)

			log = &hraft.Log{}
			err = rs.GetLog(0, log)
			assert.Equal(t, hraft.ErrLogNotFound, err)
		})

		t.Run("Set-Get", func(t *testing.T) {
			v, err := rs.Get([]byte(""))
			assert.Equal(t, raft.ErrKeyNotFound, err)
			assert.Equal(t, []byte{}, v)

			v, err = rs.Get([]byte("bad"))
			assert.Equal(t, raft.ErrKeyNotFound, err)
			assert.Equal(t, []byte{}, v)

			k, v := []byte("hello"), []byte("world")
			err = rs.Set(k, v)
			assert.NoError(t, err)

			val, err := rs.Get(k)
			assert.NoError(t, err)
			assert.Equal(t, v, val)
		})

		t.Run("SetUint64-GetUint64", func(t *testing.T) {
			val, err := rs.GetUint64([]byte(""))
			assert.Equal(t, raft.ErrKeyNotFound, err)
			assert.Equal(t, uint64(0), val)

			val, err = rs.GetUint64([]byte("bad"))
			assert.Equal(t, raft.ErrKeyNotFound, err)
			assert.Equal(t, uint64(0), val)

			k, v := []byte("foo"), uint64(42)
			err = rs.SetUint64(k, v)
			assert.NoError(t, err)

			val, err = rs.GetUint64(k)
			assert.NoError(t, err)
			assert.Equal(t, v, val)
		})

		t.Run("Close without error", func(t *testing.T) {
			assert.NoError(t, rs.Close(true))
		})

	}, func() map[string]func() { return make(map[string]func()) })
}
