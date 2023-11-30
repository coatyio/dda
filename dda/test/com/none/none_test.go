//go:build testing

// SPDX-FileCopyrightText: © 2023 Siemens AG
// SPDX-License-Identifier: MIT

// Package none_test provides entry points for tests and benchmarks of
// communication services that use no authentication.
package none_test

import (
	"testing"

	"github.com/coatyio/dda/config"
	"github.com/coatyio/dda/dda/test/com"
	"github.com/coatyio/dda/testdata"
)

var testServices = map[string]config.ConfigComService{
	"MQTT5-QoS0-NoAuth": {
		Protocol: "mqtt5",
		Url:      "tcp://localhost:1889",
		Auth:     config.AuthOptions{},
		Opts: map[string]any{
			"debug":          true, // enable paho debug logging for task test-log
			"strictClientId": true,
		},
	},
	"MQTT5-QoS1-NoAuth": {
		Protocol: "mqtt5",
		Url:      "tcp://localhost:1889",
		Auth:     config.AuthOptions{},
		Opts: map[string]any{
			"debug":          true, // enable paho debug logging for task test-log
			"strictClientId": true,
			"qos":            1,
		},
	},
	"MQTT5-QoS2-NoAuth": {
		Protocol: "mqtt5",
		Url:      "tcp://localhost:1889",
		Auth:     config.AuthOptions{},
		Opts: map[string]any{
			"debug":          true, // enable paho debug logging for task test-log
			"strictClientId": true,
			"qos":            2,
		},
	},
}

var testComSetup = make(testdata.CommunicationSetup)

func init() {
	testComSetup["mqtt5"] = &testdata.CommunicationSetupOptions{
		SetupOpts: map[string]any{
			"brokerPort":          1889,
			"brokerWsPort":        0, // WebSocket connection setup not needed
			"brokerLogInfo":       false,
			"brokerLogDebugHooks": false,
		},
	}
}

func TestMain(m *testing.M) {
	testdata.RunMainWithComSetup(m, testComSetup)
}

func TestCom(t *testing.T) {
	for name, srv := range testServices {
		t.Run(name, func(t *testing.T) {
			com.RunTestComService(t, name, srv, testComSetup)
		})
	}
}

func BenchmarkCom(b *testing.B) {
	// This benchmark with setup will not be measured itself and called once
	// with b.N=1

	for name, srv := range testServices {
		// This subbenchmark with setup will not be measured itself and called
		// once with b.N=1
		b.Run(name, func(b *testing.B) {
			com.RunBenchComService(b, name, srv)
		})
	}
}
