// SPDX-FileCopyrightText: © 2023 Siemens AG
// SPDX-License-Identifier: MIT

// Package pebble provides a storage binding implementation for the [Pebble]
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

// PebbleBinding realizes a storage binding for the Pebble key-value store by
// implementing the storage API interface.
type PebbleBinding struct {
	mu sync.RWMutex // protects following fields
	db *pebble.DB
}

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

func (b *PebbleBinding) Get(key string) ([]byte, error) {
	b.mu.RLock() // it is safe to call Pebble Get and NewIter from concurrent goroutines
	defer b.mu.RUnlock()

	if b.db == nil {
		return nil, fmt.Errorf("Get %s failed as binding is not yet open", key)
	}

	v, closer, err := b.db.Get([]byte(key))
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

func (b *PebbleBinding) Set(key string, value []byte) error {
	if value == nil {
		return fmt.Errorf("Set %s failed as value must not be nil", key)
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.db == nil {
		return fmt.Errorf("Set %s failed as binding is not yet open", key)
	}
	if err := b.db.Set([]byte(key), value, pebble.Sync); err != nil {
		return services.NewRetryableError(err)
	}
	return nil
}

func (b *PebbleBinding) Delete(key string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.db == nil {
		return fmt.Errorf("Delete %s failed as binding is not yet open", key)
	}
	if err := b.db.Delete([]byte(key), pebble.Sync); err != nil {
		return services.NewRetryableError(err)
	}
	return nil
}

func (b *PebbleBinding) DeleteAll() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.db == nil {
		return fmt.Errorf("DeleteAll failed as binding is not yet open")
	}

	iter, err := b.db.NewIter(nil)
	if err != nil {
		return services.NewRetryableError(err)
	}
	defer b.db.Flush() // NoSync+Flush is one order of magnitude faster than Sync after every Delete
	for iter.First(); iter.Valid(); iter.Next() {
		err := b.db.Delete(iter.Key(), pebble.NoSync)
		if err != nil { // fail fast
			return services.NewRetryableError(fmt.Errorf("DeleteAll failed on key %s: %w: %w", iter.Key(), err, iter.Close()))
		}
	}
	return iter.Close()
}

func (b *PebbleBinding) DeletePrefix(prefix string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.db == nil {
		return fmt.Errorf("DeletePrefix %s failed as binding is not yet open", prefix)
	}
	p := []byte(prefix)
	if err := b.db.DeleteRange(p, b.keyUpperBound(p), pebble.Sync); err != nil {
		return services.NewRetryableError(err)
	}
	return nil
}

func (b *PebbleBinding) DeleteRange(start, end string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.db == nil {
		return fmt.Errorf("DeleteRange [%s,%s) failed as binding is not yet open", start, end)
	}
	if err := b.db.DeleteRange([]byte(start), []byte(end), pebble.Sync); err != nil {
		return services.NewRetryableError(err)
	}
	return nil
}

func (b *PebbleBinding) ScanPrefix(prefix string, callback api.ScanKeyValue) error {
	b.mu.RLock() // it is safe to call Pebble NewIter and Get from concurrent goroutines
	defer b.mu.RUnlock()

	if b.db == nil {
		return fmt.Errorf("ScanPrefix %s failed as binding is not yet open", prefix)
	}

	pre := []byte(prefix)
	iter, err := b.db.NewIter(&pebble.IterOptions{
		LowerBound: pre,
		UpperBound: b.keyUpperBound(pre), // excluding
	})
	if err != nil {
		return services.NewRetryableError(err)
	}
	for iter.First(); iter.Valid(); iter.Next() {
		val := make([]byte, len(iter.Value()))
		copy(val, iter.Value())
		if !callback(string(iter.Key()), val) {
			break
		}
	}
	return iter.Close()
}

func (b *PebbleBinding) ScanRange(start, end string, callback api.ScanKeyValue) error {
	b.mu.RLock() // it is safe to call Pebble NewIter and Get from concurrent goroutines
	defer b.mu.RUnlock()

	if b.db == nil {
		return fmt.Errorf("ScanRange [%s,%s) failed as binding is not yet open", start, end)
	}

	iter, err := b.db.NewIter(&pebble.IterOptions{
		LowerBound: []byte(start),
		UpperBound: []byte(end),
	})
	if err != nil {
		return services.NewRetryableError(err)
	}
	for iter.First(); iter.Valid(); iter.Next() {
		val := make([]byte, len(iter.Value()))
		copy(val, iter.Value())
		if !callback(string(iter.Key()), val) {
			break
		}
	}
	return iter.Close()
}

func (b *PebbleBinding) keyUpperBound(prefix []byte) []byte {
	end := make([]byte, len(prefix))
	copy(end, prefix)
	for i := len(end) - 1; i >= 0; i-- {
		end[i] = end[i] + 1
		if end[i] != 0 {
			return end[:i+1]
		}
	}
	return nil // no upper-bound
}
