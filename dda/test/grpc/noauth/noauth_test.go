//go:build testing

// SPDX-FileCopyrightText: © 2023 Siemens AG
// SPDX-License-Identifier: MIT

// Package noauth_test provides entry points for tests and benchmarks of the gRPC
// Client API without authentication.
package noauth_test

import (
	"testing"

	"github.com/coatyio/dda/config"
	"github.com/coatyio/dda/dda/test/grpc"
	"github.com/coatyio/dda/testdata"
)

var testServices = map[string]config.ConfigComService{
	"MQTT5-QoS0-NoAuth": {
		Protocol: "mqtt5",
		Url:      "tcp://localhost:1910",
		Auth:     config.AuthOptions{},
		Opts: map[string]any{
			"debug":          true, // enable paho debug logging for task test-log
			"strictClientId": true,
			"qos":            0,
		},
	},
}

var testApis = map[string]config.ConfigApis{
	"gRPC-NoAuth": {
		Grpc: config.ConfigApi{
			Address:  ":8990",
			Disabled: false,
		},
	},
}

var testPubSubSetup = make(testdata.PubSubCommunicationSetup)

func init() {
	testPubSubSetup["mqtt5"] = &testdata.PubSubSetupOptions{
		SetupOpts: map[string]any{
			"brokerPort":          1910,
			"brokerWsPort":        0, // WebSocket connection setup not needed
			"brokerLogInfo":       false,
			"brokerLogDebugHooks": false,
		},
	}
}

func TestMain(m *testing.M) {
	testdata.RunMainWithSetup(m, testPubSubSetup)
}

func TestDda(t *testing.T) {
	for apiName, api := range testApis {
		for srvName, srv := range testServices {
			tn := testdata.GetTestName(apiName, srvName)
			t.Run(tn, func(t *testing.T) {
				grpc.RunTestGrpc(t, tn, api, srv, testPubSubSetup)
			})
		}
	}
}

func BenchmarkDda(b *testing.B) {
	// This benchmark with setup will not be measured itself and called once
	// with b.N=1

	for apiName, api := range testApis {
		for srvName, srv := range testServices {
			tn := testdata.GetTestName(apiName, srvName)

			// This benchmark with setup will not be measured itself and called
			// once with b.N=1
			b.Run(tn, func(b *testing.B) {
				grpc.RunBenchGrpc(b, tn, api, srv)
			})
		}
	}
}
