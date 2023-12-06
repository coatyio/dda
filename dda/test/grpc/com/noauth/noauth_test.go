//go:build testing

// SPDX-FileCopyrightText: © 2023 Siemens AG
// SPDX-License-Identifier: MIT

// Package noauth_test provides entry points for tests and benchmarks of the
// gRPC Client communication API without authentication.
package noauth_test

import (
	"testing"

	"github.com/coatyio/dda/config"
	"github.com/coatyio/dda/dda/test/grpc/com"
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
	"MQTT5-QoS0-NoAuth-disabled": {
		Protocol: "mqtt5",
		Disabled: true,
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

var testComSetup = make(testdata.CommunicationSetup)

func init() {
	testComSetup["mqtt5"] = &testdata.CommunicationSetupOptions{
		SetupOpts: map[string]any{
			"brokerPort":          1910,
			"brokerWsPort":        0, // WebSocket connection setup not needed
			"brokerLogInfo":       false,
			"brokerLogDebugHooks": false,
		},
	}
}

func TestMain(m *testing.M) {
	testdata.RunWithComSetup(func() { m.Run() }, testComSetup)
}

func TestGrpcCom(t *testing.T) {
	for apiName, api := range testApis {
		for srvName, srv := range testServices {
			tn := testdata.GetTestName(apiName, srvName)
			t.Run(tn, func(t *testing.T) {
				com.RunTestGrpc(t, tn, api, srv)
			})
		}
	}
}

func BenchmarkGrpcCom(b *testing.B) {
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
				com.RunBenchGrpc(b, tn, api, srv)
			})
		}
	}
}
