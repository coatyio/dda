// SPDX-FileCopyrightText: © 2023 Siemens AG
// SPDX-License-Identifier: MIT

// Package api provides the local storage API for a single DDA sidecar or
// instance. This API is implemented by storage bindings that use specific
// underlying embedded, i.e. file-based key-value storage systems, like Pebble,
// LevelDB, RocksDB, BoltDB, SQLite, and others. Storage uses a key-value data
// model where keys are strings and values are represented as arbitrary binary
// data. Such embedded storage is typically implemented using B+ Trees for
// read-intensive workloads and log-structured merge trees (LSM) with
// write-ahead log (WAL) for write-intensive workloads. You may choose a storage
// binding that best fits your application's needs.
package api

import "github.com/coatyio/dda/config"

// ScanKeyValue is a callback function invoked by a scanning function on each
// matching key-value pair. If an error is returned scanning stops and no
// further callbacks are invoked.
//
// It is safe to modify the contents of the value argument.
type ScanKeyValue = func(key string, value []byte) error

// Api is an interface for local persistent or in-memory key-value storage to be
// provided by a single DDA sidecar or instance. The storage API is implemented
// by storage bindings that use specific underlying embedded, i.e. file-based
// storage systems, like Pebble, LevelDB, RocksDB, BoltDB, SQLite, and others.
// Storage uses a key-value data model where keys are strings and values are
// represented as arbitrary binary data. Such embedded storage is typically
// implemented using B+ Trees for read-intensive workloads and log-structured
// merge trees (LSM) with write ahead log (WAL) for write-intensive workloads.
// You may choose a storage binding that best fits your application's needs.
//
// The API supports basic key-value store operations such as Get, Set, and
// Delete, deleting all or a range of keys, as well as iterating over keys using
// prefix and range scans.
//
// Note that Api implementations are meant to be thread-safe and individual Api
// interface methods may be run on concurrent goroutines. However, thread safety
// across multiple API methods, such as in read-modify-write-loops, must be
// realized by the application. ACID transactions are not supported.
//
// Note that local storage is not designed to be shared among multiple DDA
// sidecars or instances, even if they run on the same host.
type Api interface {

	// Open synchronously opens the storage maintained by the underlying storage
	// engine using the supplied configuration. Upon successful opening or if
	// the binding has been opened already, nil is returned. A binding-specific
	// error is returned in case the operation fails.
	Open(cfg *config.Config) error

	// Close synchronously closes the storage maintained by the underlying
	// storage engine previously opened. Close is a no-op if the binding is not
	// yet open. If the operation fails the binding-specific error should be
	// logged.
	Close()

	// Get returns the value of the given key. It returns nil along with a nil
	// error if the store does not contain the key. A binding-specific error
	// along with a nil value is returned if the operation fails.
	//
	// It is safe to modify the contents of the returned byte slice.
	Get(key string) ([]byte, error)

	// Set sets the value for the given key. It overwrites any previous value
	// for that key. If value is nil or []byte(nil), or if the operation fails a
	// binding-specific error is returned.
	//
	// It is safe to modify the contents of the value after Set returns.
	Set(key string, value []byte) error

	// Delete deletes the value for the given key. A binding-specific error is
	// returned in case the operation fails.
	Delete(key string) error

	// DeleteAll deletes all key-value pairs in the store. A binding-specific
	// error is returned in case the operation fails.
	DeleteAll() error

	// DeleteRange deletes all of the keys (and values) in the right-open range
	// [start,end) (inclusive on start, exclusive on end). Key strings are
	// ordered lexicographically by their underlying byte representation, i.e.
	// UTF-8 encoding. A binding-specific error is returned in case the
	// operation fails.
	DeleteRange(start, end string) error

	// ScanPrefix iterates over key-value pairs whose keys match the given
	// prefix in key order. Key strings are ordered lexicographically by their
	// underlying byte representation, i.e. UTF-8 encoding. A binding-specific
	// error is returned in case the operation fails.
	//
	// It is safe to modify the contents of the callback value byte slice.
	ScanPrefix(prefix string, callback ScanKeyValue) error

	// ScanRange iterates over a given right-open range of key-value pairs in
	// key order (inclusive on start, exclusive on end). Key strings are ordered
	// lexicographically by their underlying byte representation, i.e. UTF-8
	// encoding. A binding-specific error is returned in case the operation
	// fails.
	//
	// For example, this function can be used to iterate over keys which
	// represent a time range with a sortable time encoding like RFC3339.
	//
	// It is safe to modify the contents of the callback value byte slice.
	ScanRange(start, end string, callback ScanKeyValue) error
}
