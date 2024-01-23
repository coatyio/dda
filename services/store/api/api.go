// SPDX-FileCopyrightText: © 2023 Siemens AG
// SPDX-License-Identifier: MIT

// Package api provides the local storage API for a single DDA sidecar or
// instance. This API is implemented by storage bindings that use specific
// underlying embedded, i.e. file-based key-value storage systems, like Pebble,
// LevelDB, RocksDB, BoltDB, SQLite, and others. Storage uses a key-value data
// model where keys are strings and values are opaque binary blobs. Such
// embedded storage is typically implemented using B+ Trees for read-intensive
// workloads and log-structured merge trees (LSM) with write-ahead log (WAL) for
// write-intensive workloads. You may choose a storage binding that best fits
// your application's needs.
package api

import "github.com/coatyio/dda/config"

// ScanKeyValue is a callback function invoked by a scanning function on each
// matching key-value pair with keys as strings. Returns true to continue
// scanning; false to stop scanning so that no further callbacks are invoked.
//
// It is safe to modify the contents of the value argument.
type ScanKeyValue = func(key string, value []byte) bool

// ScanKeyValueB is a callback function invoked by a scanning function on each
// matching key-value pair with key as byte slices. Returns true to continue
// scanning; false to stop scanning so that no further callbacks are invoked.
//
// It is safe to modify the contents of the value argument.
type ScanKeyValueB = func(key []byte, value []byte) bool

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
	// engine using the supplied DDA configuration. Upon successful opening or
	// if the binding has been opened already, nil is returned. A
	// binding-specific error is returned in case the operation fails.
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

	// GetB is identical to [Get] with key as a byte slice.
	GetB(key []byte) ([]byte, error)

	// Set sets the value for the given key. It overwrites any previous value
	// for that key. If value is nil or []byte(nil), or if the operation fails a
	// binding-specific error is returned.
	//
	// It is safe to modify the contents of the value after Set returns.
	Set(key string, value []byte) error

	// SetB is identical to [Set] with key as a byte slice.
	SetB(key []byte, value []byte) error

	// Delete deletes the value for the given key. A binding-specific error is
	// returned in case the operation fails.
	Delete(key string) error

	// DeleteB is identical to [Delete] with prefix as a byte slice.
	DeleteB(key []byte) error

	// DeleteAll deletes all key-value pairs in the store. A binding-specific
	// error is returned in case the operation fails.
	DeleteAll() error

	// DeletePrefix deletes all of the keys (and values) that start with the
	// given prefix. Key strings are ordered lexicographically by their
	// underlying byte representation, i.e. UTF-8 encoding. A binding-specific
	// error is returned in case the operation fails.
	DeletePrefix(prefix string) error

	// DeletePrefixB is identical to [DeletePrefix] with prefix as a byte slice.
	DeletePrefixB(prefix []byte) error

	// DeleteRange deletes all of the keys (and values) in the right-open range
	// [start,end) (inclusive on start, exclusive on end). Key strings are
	// ordered lexicographically by their underlying byte representation, i.e.
	// UTF-8 encoding. A binding-specific error is returned in case the
	// operation fails.
	DeleteRange(start, end string) error

	// DeleteRangeB is identical to [DeleteRange] with start and end as byte
	// slices.
	DeleteRangeB(start, end []byte) error

	// ScanPrefix iterates in ascending key order over key-value pairs whose
	// keys start with the given prefix. Key strings are ordered
	// lexicographically by their underlying byte representation, i.e. UTF-8
	// encoding. A binding-specific error is returned in case the operation
	// fails.
	//
	// It is safe to modify the contents of the callback value byte slice.
	//
	// It is not safe to invoke Set, Delete, DeleteAll, DeletePrefix, and
	// DeleteRange store operations in the callback as such calls may block
	// forever. Instead, accumulate key-value pairs and issue such operations
	// after scanning is finished.
	ScanPrefix(prefix string, callback ScanKeyValue) error

	// ScanPrefixB is identical to [ScanPrefix] with prefix as a byte slice.
	ScanPrefixB(prefix []byte, callback ScanKeyValueB) error

	// ScanRange iterates in ascending key order over a right-open range
	// [start,end) of key-value pairs (inclusive on start, exclusive on end).
	// Key strings are ordered lexicographically by their underlying byte
	// representation, i.e. UTF-8 encoding. A binding-specific error is returned
	// in case the operation fails.
	//
	// You may specify start and/or end as nil to start/end scanning with the
	// first/last key in the store.
	//
	// For example, this function can be used to iterate over keys which
	// represent a time range with a sortable time encoding like RFC3339.
	//
	// It is safe to modify the contents of the callback value byte slice.
	//
	// It is not safe to invoke Set, Delete, DeleteAll, DeletePrefix, and
	// DeleteRange store operations in the callback as such calls may block
	// forever. Instead, accumulate key-value pairs and issue such operations
	// after scanning is finished.
	ScanRange(start, end string, callback ScanKeyValue) error

	// ScanRangeB is identical to [ScanRange] with start and end as byte slices.
	ScanRangeB(start, end []byte, callback ScanKeyValueB) error

	// ScanRangeReverseB is like [ScanRangeB] but scans the given range [start,
	// end) in reverse, i.e. descending key order.
	ScanRangeReverseB(start, end []byte, callback ScanKeyValueB) error

	// KeyUpperBound returns the next key by incrementing the given key bytes.
	// Returns []byte(nil) if given key byte slice is empty or if all bytes in
	// the given key have a value of 255.
	//
	// This function is useful if you have an inclusive upper bound for scanning
	// or deleting a range of keys requiring an exclusive upper bound.
	KeyUpperBound(key []byte) []byte
}
