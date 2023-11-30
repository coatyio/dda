//go:build testing

// SPDX-FileCopyrightText: © 2023 Siemens AG
// SPDX-License-Identifier: MIT

// Package store_test provides entry points for tests and benchmarks of the gRPC
// Client store API without authentication.
package noauth_test

import (
	"testing"

	"github.com/coatyio/dda/config"
	"github.com/coatyio/dda/dda/test/grpc/store"
	"github.com/coatyio/dda/testdata"
)

var testServices = map[string]*config.ConfigStoreService{
	"Pebble-persistent": {
		Engine:   "pebble",
		Location: "dda-test-pebble",
		Disabled: false,
	},
	"Pebble-persistent-disabled": {
		Engine:   "pebble",
		Location: "dda-test-pebble",
		Disabled: true,
	},
}

var testApis = map[string]config.ConfigApis{
	"gRPC-NoAuth": {
		Grpc: config.ConfigApi{
			Address:  ":10990",
			Disabled: false,
		},
	},
}

var testStoreSetup = make(testdata.StoreSetup)

func init() {
	// Set up unique storage locations in temp directory.
	s := testServices["Pebble-persistent"]
	testStoreSetup["pebble"] = &testdata.StoreSetupOptions{
		StorageDir: testdata.CreateStoreLocation(s),
	}
	s = testServices["Pebble-persistent-disabled"]
	testStoreSetup["pebble-disabled"] = &testdata.StoreSetupOptions{
		StorageDir: testdata.CreateStoreLocation(s),
	}
}

func TestMain(m *testing.M) {
	testdata.RunMainWithStoreSetup(m, testStoreSetup)
}

func TestGrpcStore(t *testing.T) {
	for apiName, api := range testApis {
		for srvName, srv := range testServices {
			tn := testdata.GetTestName(apiName, srvName)
			t.Run(tn, func(t *testing.T) {
				store.RunTestGrpc(t, tn, api, *srv)
			})
		}
	}
}

func BenchmarkGrpcStore(b *testing.B) {
	// This benchmark with setup will not be measured itself and called once
	// with b.N=1

	for apiName, api := range testApis {
		for srvName, srv := range testServices {
			if srv.Disabled {
				continue // skip disabled services
			}

			tn := testdata.GetTestName(apiName, srvName)

			// This benchmark with setup will not be measured itself and called
			// once with b.N=1
			b.Run(tn, func(b *testing.B) {
				store.RunBenchGrpc(b, tn, api, *srv)
			})
		}
	}
}
