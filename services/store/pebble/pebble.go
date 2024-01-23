// SPDX-FileCopyrightText: © 2023 Siemens AG
// SPDX-License-Identifier: MIT

// Package pebble provides a storage binding implementation using the [Pebble]
// storage engine.
//
// [Pebble]: https://github.com/cockroachdb/pebble
package pebble

import (
	"fmt"
	"sync"

	"github.com/coatyio/dda/config"
	"github.com/coatyio/dda/plog"
	"github.com/coatyio/dda/services"
	"github.com/coatyio/dda/services/store/api"
	"github.com/cockroachdb/pebble"
	"github.com/cockroachdb/pebble/vfs"
)

// errorOnlyLogger outputs Pebble error messages using the plog package.
type errorOnlyLogger struct{}

func (l *errorOnlyLogger) Infof(format string, args ...any) {
	// Suppress informational output.
}

func (l *errorOnlyLogger) Errorf(format string, args ...any) {
	plog.Printf(format, args...)
}

func (l *errorOnlyLogger) Fatalf(format string, args ...any) {
	plog.Printf(format, args...)
}

// PebbleBinding realizes a storage binding for the [Pebble] key-value store by
// implementing the storage API interface [api.Api].
//
// [Pebble]: https://github.com/cockroachdb/pebble
type PebbleBinding struct {
	mu sync.RWMutex // protects following fields
	db *pebble.DB
}

// Open implements the [api.Api] interface.
func (b *PebbleBinding) Open(cfg *config.Config) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.db != nil {
		return nil
	}

	loc := cfg.Services.Store.Location
	opts := &pebble.Options{
		Logger: &errorOnlyLogger{},
	}
	if loc == "" {
		opts.FS = vfs.NewMem()
		plog.Printf("Open Pebble storage binding with in-memory store...\n")
	} else {
		plog.Printf("Open Pebble storage binding with persistent store at %s...\n", loc)
	}

	var err error
	if b.db, err = pebble.Open(loc, opts); err != nil {
		return err
	}
	return nil
}

// Close implements the [api.Api] interface.
func (b *PebbleBinding) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.db == nil {
		return
	}

	if err := b.db.Close(); err != nil {
		plog.Printf("close Pebble store failed: %v", err)
	}

	b.db = nil

	plog.Printf("Closed Pebble storage binding\n")
}

// Get implements the [api.Api] interface.
func (b *PebbleBinding) Get(key string) ([]byte, error) {
	return b.GetB([]byte(key))
}

// GetB implements the [api.Api] interface.
func (b *PebbleBinding) GetB(key []byte) ([]byte, error) {
	b.mu.RLock() // it is safe to call Pebble Get and NewIter from concurrent goroutines
	defer b.mu.RUnlock()

	if b.db == nil {
		return nil, fmt.Errorf("GetB %v failed as binding is not yet open", key)
	}

	v, closer, err := b.db.Get(key)
	if err == pebble.ErrNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, services.NewRetryableError(err)
	}
	defer closer.Close()
	val := make([]byte, len(v))
	copy(val, v)
	return val, nil
}

// Set implements the [api.Api] interface.
func (b *PebbleBinding) Set(key string, value []byte) error {
	return b.SetB([]byte(key), value)
}

// SetB implements the [api.Api] interface.
func (b *PebbleBinding) SetB(key []byte, value []byte) error {
	if value == nil {
		return fmt.Errorf("SetB %v failed as value must not be nil", key)
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.db == nil {
		return fmt.Errorf("SetB %v failed as binding is not yet open", key)
	}
	if err := b.db.Set(key, value, pebble.Sync); err != nil {
		return services.NewRetryableError(err)
	}
	return nil
}

// Delete implements the [api.Api] interface.
func (b *PebbleBinding) Delete(key string) error {
	return b.DeleteB([]byte(key))
}

// DeleteB implements the [api.Api] interface.
func (b *PebbleBinding) DeleteB(key []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.db == nil {
		return fmt.Errorf("DeleteB %v failed as binding is not yet open", key)
	}
	if err := b.db.Delete(key, pebble.Sync); err != nil {
		return services.NewRetryableError(err)
	}
	return nil
}

// DeleteAll implements the [api.Api] interface.
func (b *PebbleBinding) DeleteAll() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.db == nil {
		return fmt.Errorf("DeleteAll failed as binding is not yet open")
	}

	iter := b.db.NewIter(nil)
	defer b.db.Flush() // NoSync+Flush is one order of magnitude faster than Sync after every Delete
	for iter.First(); iter.Valid(); iter.Next() {
		err := b.db.Delete(iter.Key(), pebble.NoSync)
		if err != nil { // fail fast
			return services.NewRetryableError(fmt.Errorf("DeleteAll failed on key %v: %w: %w", iter.Key(), err, iter.Close()))
		}
	}
	return iter.Close()
}

// DeletePrefix implements the [api.Api] interface.
func (b *PebbleBinding) DeletePrefix(prefix string) error {
	return b.DeletePrefixB([]byte(prefix))
}

// DeletePrefixB implements the [api.Api] interface.
func (b *PebbleBinding) DeletePrefixB(prefix []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.db == nil {
		return fmt.Errorf("DeletePrefixB %v failed as binding is not yet open", prefix)
	}
	if err := b.db.DeleteRange(prefix, b.KeyUpperBound(prefix), pebble.Sync); err != nil {
		return services.NewRetryableError(err)
	}
	return nil
}

// DeleteRange implements the [api.Api] interface.
func (b *PebbleBinding) DeleteRange(start, end string) error {
	return b.DeleteRangeB([]byte(start), []byte(end))
}

// DeleteRangeB implements the [api.Api] interface.
func (b *PebbleBinding) DeleteRangeB(start, end []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.db == nil {
		return fmt.Errorf("DeleteRangeB [%v,%v) failed as binding is not yet open", start, end)
	}
	if err := b.db.DeleteRange(start, end, pebble.Sync); err != nil {
		return services.NewRetryableError(err)
	}
	return nil
}

// ScanPrefix implements the [api.Api] interface.
func (b *PebbleBinding) ScanPrefix(prefix string, callback api.ScanKeyValue) error {
	return b.ScanPrefixB([]byte(prefix), func(key []byte, value []byte) bool {
		return callback(string(key), value)
	})
}

// ScanPrefixB implements the [api.Api] interface.
func (b *PebbleBinding) ScanPrefixB(prefix []byte, callback api.ScanKeyValueB) error {
	b.mu.RLock() // it is safe to call Pebble NewIter and Get from concurrent goroutines
	defer b.mu.RUnlock()

	if b.db == nil {
		return fmt.Errorf("ScanPrefixB %v failed as binding is not yet open", prefix)
	}

	iter := b.db.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: b.KeyUpperBound(prefix), // excluding
	})
	for iter.First(); iter.Valid(); iter.Next() {
		key := make([]byte, len(iter.Key()))
		val := make([]byte, len(iter.Value()))
		copy(key, iter.Key())
		copy(val, iter.Value())
		if !callback(key, val) {
			break
		}
	}
	return iter.Close()
}

// ScanRange implements the [api.Api] interface.
func (b *PebbleBinding) ScanRange(start, end string, callback api.ScanKeyValue) error {
	return b.ScanRangeB([]byte(start), []byte(end), func(key []byte, value []byte) bool {
		return callback(string(key), value)
	})
}

// ScanRangeB implements the [api.Api] interface.
func (b *PebbleBinding) ScanRangeB(start, end []byte, callback api.ScanKeyValueB) error {
	b.mu.RLock() // it is safe to call Pebble NewIter and Get from concurrent goroutines
	defer b.mu.RUnlock()

	if b.db == nil {
		return fmt.Errorf("ScanRangeB [%v,%v) failed as binding is not yet open", start, end)
	}

	iter := b.db.NewIter(&pebble.IterOptions{
		LowerBound: start,
		UpperBound: end,
	})
	for iter.First(); iter.Valid(); iter.Next() {
		key := make([]byte, len(iter.Key()))
		val := make([]byte, len(iter.Value()))
		copy(key, iter.Key())
		copy(val, iter.Value())
		if !callback(key, val) {
			break
		}
	}
	return iter.Close()
}

// ScanRangeReverseB implements the [api.Api] interface.
func (b *PebbleBinding) ScanRangeReverseB(start, end []byte, callback api.ScanKeyValueB) error {
	b.mu.RLock() // it is safe to call Pebble NewIter and Get from concurrent goroutines
	defer b.mu.RUnlock()

	if b.db == nil {
		return fmt.Errorf("ScanRangeReverseB [%v,%v) failed as binding is not yet open", start, end)
	}

	iter := b.db.NewIter(&pebble.IterOptions{
		LowerBound: start,
		UpperBound: end,
	})
	for iter.Last(); iter.Valid(); iter.Prev() {
		key := make([]byte, len(iter.Key()))
		val := make([]byte, len(iter.Value()))
		copy(key, iter.Key())
		copy(val, iter.Value())
		if !callback(key, val) {
			break
		}
	}
	return iter.Close()
}

// KeyUpperBound implements the [api.Api] interface.
func (b *PebbleBinding) KeyUpperBound(key []byte) []byte {
	end := make([]byte, len(key))
	copy(end, key)
	for i := len(end) - 1; i >= 0; i-- {
		end[i] = end[i] + 1
		if end[i] != 0 {
			return end[:i+1]
		}
	}
	return nil // no upper-bound
}
