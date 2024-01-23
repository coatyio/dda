//go:build testing

// SPDX-FileCopyrightText: © 2024 Siemens AG
// SPDX-License-Identifier: MIT

// Package noauth_test provides entry points for tests and benchmarks of the
// gRPC Client state API without authentication.
package noauth_test

import (
	"testing"

	"github.com/coatyio/dda/config"
	"github.com/coatyio/dda/dda/test/grpc/state"
	"github.com/coatyio/dda/testdata"
)

var testServices = map[string]config.ConfigStateService{
	"Raft": {
		Protocol:  "raft",
		Store:     "", // in-memory
		Bootstrap: false,
		Disabled:  false,
		Opts: map[string]any{
			"debug": true, // enable Raft debug logging for task test-log
		},
	},
	"Raft-disabled": {
		Protocol:  "raft",
		Store:     "", // in-memory
		Bootstrap: false,
		Disabled:  true,
		Opts: map[string]any{
			"debug": true, // enable Raft debug logging for task test-log
		},
	},
}

var requiredService = config.ConfigComService{
	Protocol: "mqtt5",
	Url:      "tcp://localhost:1920",
	Auth:     config.AuthOptions{},
	Opts: map[string]any{
		"debug":          true, // enable paho debug logging for task test-log
		"strictClientId": true,
	},
}

var testApis = map[string]config.ConfigApis{
	"gRPC-NoAuth": {
		Grpc: config.ConfigApi{
			Address:  ":11990",
			Disabled: false,
		},
	},
}

var testComSetup = make(testdata.CommunicationSetup)

func init() {
	// Set up required pub-sub communication infrastructure.
	testComSetup["mqtt5"] = &testdata.CommunicationSetupOptions{
		SetupOpts: map[string]any{
			"brokerPort":          1920,
			"brokerWsPort":        0, // WebSocket connection setup not needed
			"brokerLogInfo":       false,
			"brokerLogDebugHooks": false,
		},
	}
}

func TestMain(m *testing.M) {
	testdata.RunWithComSetup(func() { m.Run() }, testComSetup)
}

func TestGrpcState(t *testing.T) {
	for apiName, api := range testApis {
		for srvName, srv := range testServices {
			tn := testdata.GetTestName(apiName, srvName)
			t.Run(tn, func(t *testing.T) {
				state.RunTestGrpc(t, tn, api, srv, requiredService)
			})
		}
	}
}

func BenchmarkGrpcState(b *testing.B) {
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
				state.RunBenchGrpc(b, tn, api, srv, requiredService)
			})
		}
	}
}
