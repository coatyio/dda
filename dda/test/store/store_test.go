//go:build testing

// SPDX-FileCopyrightText: © 2023 Siemens AG
// SPDX-License-Identifier: MIT

// Package store_test provides end-to-end test and benchmark functions for the
// storage service to be tested over different storage bindings.
package store_test

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/coatyio/dda/config"
	"github.com/coatyio/dda/testdata"
	"github.com/stretchr/testify/assert"
)

var testServices = map[string]*config.ConfigStoreService{
	"Pebble-inmemory": {
		Engine:   "pebble",
		Location: "",
		Disabled: false,
	},
	"Pebble-persistent": {
		Engine:   "pebble",
		Location: "dda-test-pebble",
		Disabled: false,
	},
}

var testStoreSetup = make(testdata.StoreSetup)

func init() {
	// Set up unique storage locations in temp directory.
	s := testServices["Pebble-persistent"]
	testStoreSetup["pebble"] = &testdata.StoreSetupOptions{
		StorageDir: testdata.CreateStoreLocation(s),
	}
}

func TestMain(m *testing.M) {
	testdata.RunWithStoreSetup(func() { m.Run() }, testStoreSetup)
}

func TestStore(t *testing.T) {
	for name, srv := range testServices {
		t.Run(name, func(t *testing.T) {
			RunTestStoreService(t, *srv)
		})
	}
}

func BenchmarkStore(b *testing.B) {
	// This benchmark with setup will not be measured itself and called once
	// with b.N=1

	for name, srv := range testServices {
		// This subbenchmark with setup will not be measured itself and called
		// once with b.N=1
		b.Run(name, func(b *testing.B) {
			RunBenchStoreService(b, srv)
		})
	}
}

// RunTestStoreService runs all tests on the given storage service
// configuration.
func RunTestStoreService(t *testing.T, srv config.ConfigStoreService) {
	cfg := testdata.NewStoreConfig(srv)

	dda, err := testdata.OpenDdaWithConfig(cfg)
	if !assert.NoError(t, err) {
		assert.FailNow(t, "Couldn't open DDA")
	}

	defer testdata.CloseDda(dda)

	t.Run("Get non-existent key", func(t *testing.T) {
		val, err := dda.Get("foo")
		assert.Nil(t, val)
		assert.NoError(t, err)
	})

	t.Run("Delete non-existent key", func(t *testing.T) {
		err := dda.Delete("foo")
		assert.NoError(t, err)
	})

	t.Run("DeleteRange non-existent keys", func(t *testing.T) {
		err := dda.DeleteRange("", "xxx")
		assert.NoError(t, err)
	})

	t.Run("DeleteAll non-existent keys", func(t *testing.T) {
		err := dda.DeleteAll()
		assert.NoError(t, err)
	})

	t.Run("Iterate non-existent keys", func(t *testing.T) {
		err := dda.ScanRange("", "xxxx", func(key string, value []byte) bool {
			assert.FailNow(t, "Scanning unexpected key")
			return true
		})
		assert.NoError(t, err)
	})

	t.Run("Set-Get empty key and value", func(t *testing.T) {
		err := dda.Set("", []byte{})
		assert.NoError(t, err)
		val, err := dda.Get("")
		assert.Equal(t, []byte{}, val)
		assert.NoError(t, err)
	})

	t.Run("Set nil value", func(t *testing.T) {
		err := dda.Set("foo", nil)
		assert.Error(t, err)
		err = dda.Set("foo", []byte(nil))
		assert.Error(t, err)
	})

	t.Run("Set non-nil values", func(t *testing.T) {
		err = dda.Set("prefix", []byte("prefix"))
		assert.NoError(t, err)
		err = dda.Set("prefix2", []byte("prefix2"))
		assert.NoError(t, err)
		err = dda.Set("prefix1", []byte("prefix1"))
		assert.NoError(t, err)
		err = dda.Set("pref", []byte("pref"))
		assert.NoError(t, err)
		err := dda.Set("pre", []byte("pre"))
		assert.NoError(t, err)
		err = dda.Set("prd", []byte("prd"))
		assert.NoError(t, err)
		err = dda.Set("prf", []byte("prf"))
		assert.NoError(t, err)
	})

	t.Run("Get non-nil values", func(t *testing.T) {
		val, err := dda.Get("prefix")
		assert.Equal(t, []byte("prefix"), val)
		assert.NoError(t, err)
		val, err = dda.Get("prefix2")
		assert.Equal(t, []byte("prefix2"), val)
		assert.NoError(t, err)
		val, err = dda.Get("prefix1")
		assert.Equal(t, []byte("prefix1"), val)
		assert.NoError(t, err)
		val, err = dda.Get("pref")
		assert.Equal(t, []byte("pref"), val)
		assert.NoError(t, err)
		val, err = dda.Get("pre")
		assert.Equal(t, []byte("pre"), val)
		assert.NoError(t, err)
		val, err = dda.Get("prd")
		assert.Equal(t, []byte("prd"), val)
		assert.NoError(t, err)
		val, err = dda.Get("prf")
		assert.Equal(t, []byte("prf"), val)
		assert.NoError(t, err)
	})

	t.Run("Delete and Set value", func(t *testing.T) {
		err := dda.Delete("pre")
		assert.NoError(t, err)
		val, err := dda.Get("pre")
		assert.Nil(t, val)
		assert.NoError(t, err)
		err = dda.Set("pre", []byte("pre"))
		assert.NoError(t, err)
		val, err = dda.Get("pre")
		assert.Equal(t, []byte("pre"), val)
		assert.NoError(t, err)
	})

	t.Run("ScanPrefix on pre* stopped", func(t *testing.T) {
		keys := make([]string, 0)
		err := dda.ScanPrefix("pre", func(key string, value []byte) bool {
			assert.Equal(t, key, string(value))
			keys = append(keys, key)
			return len(keys) != 1 // stop after first callback
		})
		assert.NoError(t, err)
		assert.Equal(t, []string{"pre"}, keys)
	})

	t.Run("ScanPrefix on pre*", func(t *testing.T) {
		keys := make([]string, 0)
		err := dda.ScanPrefix("pre", func(key string, value []byte) bool {
			assert.Equal(t, key, string(value))
			keys = append(keys, key)
			return true
		})
		assert.NoError(t, err)
		assert.Equal(t, []string{"pre", "pref", "prefix", "prefix1", "prefix2"}, keys)
	})

	t.Run("ScanPrefix on pref*", func(t *testing.T) {
		keys := make([]string, 0)
		err := dda.ScanPrefix("pref", func(key string, value []byte) bool {
			assert.Equal(t, key, string(value))
			keys = append(keys, key)
			return true
		})
		assert.NoError(t, err)
		assert.Equal(t, []string{"pref", "prefix", "prefix1", "prefix2"}, keys)
	})

	t.Run("ScanPrefix on prefix*", func(t *testing.T) {
		keys := make([]string, 0)
		err := dda.ScanPrefix("prefix", func(key string, value []byte) bool {
			assert.Equal(t, key, string(value))
			keys = append(keys, key)
			return true
		})
		assert.NoError(t, err)
		assert.Equal(t, []string{"prefix", "prefix1", "prefix2"}, keys)
	})

	t.Run("ScanRange [prefix,prefix2) stopped", func(t *testing.T) {
		keys := make([]string, 0)
		err := dda.ScanRange("prefix", "prefix2", func(key string, value []byte) bool {
			assert.Equal(t, key, string(value))
			keys = append(keys, key)
			return len(keys) != 1 // stop after first callback
		})
		assert.NoError(t, err)
		assert.Equal(t, []string{"prefix"}, keys)
	})

	t.Run("ScanRange [prefix,prefix2)", func(t *testing.T) {
		keys := make([]string, 0)
		err := dda.ScanRange("prefix", "prefix2", func(key string, value []byte) bool {
			assert.Equal(t, key, string(value))
			keys = append(keys, key)
			return true
		})
		assert.NoError(t, err)
		assert.Equal(t, []string{"prefix", "prefix1"}, keys)
	})

	t.Run("ScanRange [pre,pref)", func(t *testing.T) {
		keys := make([]string, 0)
		err := dda.ScanRange("pre", "pref", func(key string, value []byte) bool {
			assert.Equal(t, key, string(value))
			keys = append(keys, key)
			return true
		})
		assert.NoError(t, err)
		assert.Equal(t, []string{"pre"}, keys)
	})

	t.Run("ScanRange [pre,prf)", func(t *testing.T) {
		keys := make([]string, 0)
		err := dda.ScanRange("pre", "prf", func(key string, value []byte) bool {
			assert.Equal(t, key, string(value))
			keys = append(keys, key)
			return true
		})
		assert.NoError(t, err)
		assert.Equal(t, []string{"pre", "pref", "prefix", "prefix1", "prefix2"}, keys)
	})

	t.Run("ScanRange [prf,prd)", func(t *testing.T) {
		keys := make([]string, 0)
		err := dda.ScanRange("prf", "prd", func(key string, value []byte) bool {
			assert.Equal(t, key, string(value))
			keys = append(keys, key)
			return true
		})
		assert.NoError(t, err)
		assert.Equal(t, []string{}, keys)
	})

	t.Run("DeletePrefix prefix", func(t *testing.T) {
		err := dda.DeletePrefix("prefix")
		assert.NoError(t, err)
		err = dda.ScanPrefix("prefix", func(key string, value []byte) bool {
			assert.FailNow(t, "Scanning unexpected key")
			return true
		})
		assert.NoError(t, err)
	})

	t.Run("DeleteRange [pre,prf)", func(t *testing.T) {
		err := dda.DeleteRange("pre", "prf")
		assert.NoError(t, err)
		err = dda.ScanRange("pre", "prf", func(key string, value []byte) bool {
			assert.FailNow(t, "Scanning unexpected key")
			return true
		})
		assert.NoError(t, err)
	})

	t.Run("DeleteAll remaining", func(t *testing.T) {
		err := dda.DeleteAll()
		assert.NoError(t, err)
		err = dda.ScanRange("a", "z", func(key string, value []byte) bool {
			assert.FailNow(t, "Scanning unexpected key")
			return true
		})
		assert.NoError(t, err)
	})
}

// RunBenchStoreService runs all benchmarks on the given storage service.
func RunBenchStoreService(b *testing.B, srv *config.ConfigStoreService) {
	// This benchmark with setup will not be measured itself and called once for
	// each service with b.N=1

	cfg := testdata.NewStoreConfig(*srv)

	dda, err := testdata.OpenDdaWithConfig(cfg)
	if err != nil {
		b.FailNow()
	}

	defer testdata.CloseDda(dda)

	kvsc := 10000000 // number of keys
	kvsclen := len(fmt.Sprintf("%d", kvsc))
	kvs := make([]struct {
		k string
		v []byte
	}, kvsc)
	for i := 0; i < kvsc; i++ {
		s := fmt.Sprintf("%0*d", kvsclen, i)
		kvs[i] = struct {
			k string
			v []byte
		}{s, []byte(s)}
	}

	i := 0

	// This subbenchmark will be measured and invoked multiple times with
	// growing b.N
	b.Run("Sequential write", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			kv := kvs[i]
			if err := dda.Set(kv.k, kv.v); err != nil {
				b.FailNow()
			}
			i++
			if i == kvsc {
				i = 0
			}
		}
	})

	i = 0

	// This subbenchmark will be measured and invoked multiple times with
	// growing b.N
	b.Run("Sequential read", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			kv := kvs[i]
			if _, err := dda.Get(kv.k); err != nil {
				b.FailNow()
			}
			i++
			if i == kvsc {
				i = 0
			}
		}
	})

	// This subbenchmark will be measured and invoked multiple times with
	// growing b.N
	b.Run("Scan all", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			if err := dda.ScanRange(kvs[0].k, kvs[kvsc-1].k, func(k string, v []byte) bool {
				return true
			}); err != nil {
				b.FailNow()
			}
		}
	})

	// This subbenchmark will be measured and invoked multiple times with
	// growing b.N
	b.Run("Scan prefix all", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			if err := dda.ScanPrefix("0", func(k string, v []byte) bool {
				return true
			}); err != nil {
				b.FailNow()
			}
		}
	})

	// This subbenchmark will be measured and invoked multiple times with
	// growing b.N
	b.Run("Delete all", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			if err := dda.DeleteAll(); err != nil {
				b.FailNow()
			}
		}
	})

	rw := rand.New(rand.NewSource(int64(kvsc))) // fixed writing source

	// This subbenchmark will be measured and invoked multiple times with
	// growing b.N
	b.Run("Random write", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			kv := kvs[rw.Intn(kvsc)]
			if err := dda.Set(kv.k, kv.v); err != nil {
				b.FailNow()
			}
		}
	})

	rr := rand.New(rand.NewSource(int64(kvsc))) // fixed reading source

	// This subbenchmark will be measured and invoked multiple times with
	// growing b.N
	b.Run("Random read", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			kv := kvs[rr.Intn(kvsc)]
			if _, err := dda.Get(kv.k); err != nil {
				b.FailNow()
			}
		}
	})
}
