// SPDX-FileCopyrightText: © 2024 Siemens AG
// SPDX-License-Identifier: MIT

// Package raft exports the RaftStore which is an implementation of a LogStore,
// a StableStore, and a FileSnapshotStore for the [HashiCorp Raft] library.
// RaftStore uses the internal DDA implementation of the [Pebble] storage
// engine.
//
// [HashiCorp Raft]: https://github.com/hashicorp/raft
// [Pebble]: https://github.com/cockroachdb/pebble
package raft

import (
	"encoding/binary"
	"errors"
	"math"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/coatyio/dda/config"
	"github.com/coatyio/dda/plog"
	"github.com/coatyio/dda/services/state/api"
	"github.com/coatyio/dda/services/store/pebble"
	hraft "github.com/hashicorp/raft"
)

const retainSnapshotCount = 2 // how many snapshots should be retained, must be at least 1

var (
	ErrKeyNotFound = errors.New("not found") // corresponds with Hashicorp raft key not found error string
)

type observer chan api.MembershipChange

// RaftStore implements a LogStore, a StableStore, and a FileSnapshotStore for
// the [HashiCorp Raft] library.
type RaftStore struct {
	logDir         string
	logStore       *pebble.PebbleBinding
	stableDir      string
	stableStore    *pebble.PebbleBinding
	snapDir        string
	SnapStore      hraft.SnapshotStore // Raft snapshot store
	mu             sync.Mutex          // protects following fields
	observers      map[uint64]observer // membership change observers indexed by observer id
	nextObserverId uint64              // id of next registered membership change channel
	memberIds      map[string]struct{} // set of current deduped members indexed by member id
}

// NewRaftStore creates local storage for persisting Raft specific durable data
// including log entries, stable store, and file snapshot store. Returns an
// error along with a nil *RaftStore if any of the stores couldn't be created.
//
// The given storage location should specify a directory given by an absolute
// pathname or a pathname relative to the working directory of the DDA sidecar
// or instance, or an empty string to indicate that storage is non-persistent
// and completely memory-backed as long as the DDA instance is running.
func NewRaftStore(location string) (*RaftStore, error) {
	logDir := location
	if logDir != "" {
		logDir = filepath.Join(location, "log")
	}
	stableDir := location
	if stableDir != "" {
		stableDir = filepath.Join(location, "stable")
	}
	var snapStore hraft.SnapshotStore
	var err error
	snapDir := location
	if snapDir != "" {
		// Snapshots are stored in subfolder "snapshots" in parent folder location.
		snapStore, err = hraft.NewFileSnapshotStore(location, retainSnapshotCount, os.Stderr)
		if err != nil {
			return nil, err
		}
	} else {
		// Create an in-memory SnapshotStore that retains only the most recent snapshot.
		snapStore = hraft.NewInmemSnapshotStore()
	}

	rs := &RaftStore{
		logDir:         logDir,
		logStore:       &pebble.PebbleBinding{},
		stableDir:      stableDir,
		stableStore:    &pebble.PebbleBinding{},
		snapDir:        snapDir,
		SnapStore:      snapStore,
		observers:      make(map[uint64]observer),
		nextObserverId: 0,
		memberIds:      make(map[string]struct{}),
	}

	cfg := config.New()
	cfg.Services.Store = config.ConfigStoreService{
		Engine:   "pebble",
		Location: rs.logDir,
		Disabled: false,
	}
	if err := rs.logStore.Open(cfg); err != nil {
		return nil, err
	}

	cfg.Services.Store = config.ConfigStoreService{
		Engine:   "pebble",
		Location: rs.stableDir,
		Disabled: false,
	}
	if err := rs.stableStore.Open(cfg); err != nil {
		return nil, err
	}

	return rs, nil
}

// Close gracefully closes the Raft store, optionally removing all associated
// persistent storage files and folders.
//
// You should not remove storage if you want to restart your DDA state member at
// a later point in time.
func (s *RaftStore) Close(removeStorage bool) error {
	s.logStore.Close()
	s.stableStore.Close()

	if removeStorage && s.logDir != "" {
		_ = os.RemoveAll(s.logDir)
	}
	if removeStorage && s.stableDir != "" {
		_ = os.RemoveAll(s.stableDir)
	}
	if removeStorage && s.snapDir != "" {
		_ = os.RemoveAll(s.snapDir)
	}

	return nil
}

// AddMembershipChangeObserver registers the given channel to listen to
// membership changes and returns a channel ID that can be used to deregister
// the channel later.
//
// The channel should continuously receive data on the channel in a non-blocking
// manner to prevent blocking send operations.
func (s *RaftStore) AddMembershipChangeObserver(ch chan api.MembershipChange, raft *hraft.Raft) uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.nextObserverId++

	go s.sendMembershipChangesForObserver(raft, ch, s.nextObserverId)

	return s.nextObserverId
}

// RemoveMembershipChangeObserver deregisters the channel with the given channel
// id.
//
// Note that the channel is not closed, it must be closed by the caller.
func (s *RaftStore) RemoveMembershipChangeObserver(chanId uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.observers, chanId)
	if len(s.observers) == 0 {
		clear(s.memberIds)
		return
	}
}

// FirstIndex returns the first index written. 0 for no entries.
//
// FirstIndex implements the [hraft.LogStore] interface.
func (s *RaftStore) FirstIndex() (uint64, error) {
	first := uint64(0) // zero on empty log
	err := s.logStore.ScanRangeB(nil, nil, func(k, v []byte) bool {
		first = bytesToUint64(k)
		return false // stop scanning
	})
	if err != nil {
		return 0, err
	}
	return first, nil
}

// LastIndex returns the last index written. 0 for no entries.
//
// LastIndex implements the [hraft.LogStore] interface.
func (s *RaftStore) LastIndex() (uint64, error) {
	last := uint64(0) // zero on empty log
	err := s.logStore.ScanRangeReverseB(nil, nil, func(k, v []byte) bool {
		last = bytesToUint64(k)
		return false // stop scanning
	})
	if err != nil {
		return 0, err
	}
	return last, nil
}

// GetLog gets a log entry at a given index.
//
// GetLog implements the [hraft.LogStore] interface.
func (s *RaftStore) GetLog(index uint64, log *hraft.Log) error {
	val, err := s.logStore.GetB(uint64ToBytes(index))
	if err != nil {
		return err
	}
	if val == nil {
		return hraft.ErrLogNotFound
	}
	return DecodeMsgPack(val, log)
}

// StoreLog stores a log entry.
//
// StoreLog implements the [hraft.LogStore] interface.
func (s *RaftStore) StoreLog(log *hraft.Log) error {
	return s.StoreLogs([]*hraft.Log{log})
}

// StoreLogs stores multiple log entries.
//
// By default the logs stored may not be contiguous with previous logs (i.e. may
// have a gap in Index since the last log written). If an implementation can't
// tolerate this it may optionally implement `MonotonicLogStore` to indicate
// that this is not allowed. This changes Raft's behaviour after restoring a
// user snapshot to remove all previous logs instead of relying on a "gap" to
// signal the discontinuity between logs before the snapshot and logs after.
//
// StoreLogs implements the [hraft.LogStore] interface.
func (s *RaftStore) StoreLogs(logs []*hraft.Log) error {
	for _, log := range logs {
		if log.Type == hraft.LogConfiguration {
			s.sendMembershipChangesForServers(hraft.DecodeConfiguration(log.Data).Servers)
		}
		key := uint64ToBytes(log.Index)
		b, err := EncodeMsgPack(log)
		if err != nil {
			return err
		}

		if err := s.logStore.SetB(key, b); err != nil {
			return err
		}
	}
	return nil
}

// DeleteRange deletes a range of log entries. The range is inclusive.
//
// DeleteRange implements the [hraft.LogStore] interface.
func (s *RaftStore) DeleteRange(min, max uint64) error {
	var maxb []byte
	if max == math.MaxUint64 {
		maxb = s.logStore.KeyUpperBound(uint64ToBytes(max))
	} else {
		maxb = uint64ToBytes(max + 1)
	}
	return s.logStore.DeleteRangeB(uint64ToBytes(min), maxb)
}

// Set sets the given key-value pair.
//
// Set implements the [hraft.StableStore] interface.
func (s *RaftStore) Set(key []byte, val []byte) error {
	return s.stableStore.SetB(key, val)
}

// Get returns the value for key, or an empty byte slice if key was not found.
//
// Get implements the [hraft.StableStore] interface.
func (s *RaftStore) Get(key []byte) ([]byte, error) {
	v, err := s.stableStore.GetB(key)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return []byte{}, ErrKeyNotFound
	}
	return v, nil
}

// SetUint64 sets the given key-value pair.
//
// SetUint64 implements the [hraft.StableStore] interface.
func (s *RaftStore) SetUint64(key []byte, val uint64) error {
	return s.stableStore.SetB(key, uint64ToBytes(val))
}

// GetUint64 returns the uint64 value for key, or 0 if key was not found.
//
// GetUint64 implements the [hraft.StableStore] interface.
func (s *RaftStore) GetUint64(key []byte) (uint64, error) {
	v, err := s.stableStore.GetB(key)
	if err != nil {
		return 0, err
	}
	if v == nil {
		return 0, ErrKeyNotFound
	}
	return bytesToUint64(v), nil
}

// bytesToUint64 converts bytes to a uint64.
func bytesToUint64(b []byte) uint64 {
	return binary.BigEndian.Uint64(b)
}

// uint64ToBytes converts a uint64 to a byte slice.
func uint64ToBytes(u uint64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, u)
	return buf
}

func (s *RaftStore) sendMembershipChangesForObserver(raft *hraft.Raft, o observer, observerId uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.observers) == 0 {
		// For first observer, dispatch member changes from current Raft
		// configuration and initialize memberIds (has been cleared by
		// RemoveMembershipChangeObserver).
		f := raft.GetConfiguration()
		if f.Error() != nil {
			plog.Printf("Raft configuration could not be retrieved for membership changes: %v", f.Error())
			return
		}
		for _, sv := range f.Configuration().Servers {
			id := string(sv.ID)
			if _, ok := s.memberIds[id]; !ok { // dedupe member ids
				s.memberIds[id] = struct{}{}
				o <- api.MembershipChange{Id: id, Joined: true}
			}
		}
	} else {
		// For late observers, dispatch member changes from current memberIds.
		for id := range s.memberIds {
			o <- api.MembershipChange{Id: id, Joined: true}
		}
	}

	// Add observer finally, ensuring that changes issued by
	// sendMembershipChangesForServers are not dispatched on the new observer
	// before initialization is completed.
	s.observers[s.nextObserverId] = o
}

func (s *RaftStore) sendMembershipChangesForServers(servers []hraft.Server) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.observers) == 0 {
		// Ignore early changes that happen before any observer is added, and
		// late changes that happen after the last observer is removed.
		return
	}

	for id := range s.memberIds {
		if !slices.ContainsFunc(servers, func(srv hraft.Server) bool { return id == string(srv.ID) }) {
			delete(s.memberIds, id)
			for _, o := range s.observers {
				o <- api.MembershipChange{Id: id, Joined: false}
			}
		}
	}
	for _, sv := range servers {
		id := string(sv.ID)
		if _, ok := s.memberIds[id]; !ok { // dedupe member ids
			s.memberIds[id] = struct{}{}
			for _, o := range s.observers {
				o <- api.MembershipChange{Id: id, Joined: true}
			}
		}
	}
}
