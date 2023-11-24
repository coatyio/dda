//go:build testing

// SPDX-FileCopyrightText: © 2023 Siemens AG
// SPDX-License-Identifier: MIT

// Package basic_test provides entry points for tests and benchmarks of
// communication services that use basic authentication.
package basic_test

import (
	"testing"

	"github.com/coatyio/dda/config"
	"github.com/coatyio/dda/dda/test/com"
	"github.com/coatyio/dda/testdata"
)

var testServices = map[string]config.ConfigComService{
	"MQTT5-QoS0-BasicAuth": {
		Protocol: "mqtt5",
		Url:      "tcp://localhost:1900",
		Auth: config.AuthOptions{
			Method:   "none",
			Username: "foo",
			Password: "bar",
		},
		Opts: map[string]any{
			"debug":          true, // enable paho debug logging for task test-log
			"strictClientId": true,
			"qos":            0,
		},
	},
}

var testPubSubSetup = make(testdata.PubSubCommunicationSetup)

func init() {
	testPubSubSetup["mqtt5"] = &testdata.PubSubSetupOptions{
		SetupOpts: map[string]any{
			"brokerPort":          1900,
			"brokerWsPort":        0, // WebSocket connection setup not needed
			"brokerLogInfo":       false,
			"brokerLogDebugHooks": false,
			"brokerBasicAuth":     map[string]string{"foo": "bar"},
		},
	}
}

func TestMain(m *testing.M) {
	testdata.RunMainWithSetup(m, testPubSubSetup)
}

func TestDda(t *testing.T) {
	for name, srv := range testServices {
		t.Run(name, func(t *testing.T) {
			com.RunTestComService(t, name, srv, testPubSubSetup)
		})
	}
}

func BenchmarkDda(b *testing.B) {
	// This benchmark with setup will not be measured itself and called once
	// with b.N=1

	for name, srv := range testServices {
		// This benchmark with setup will not be measured itself and called once
		// with b.N=1
		b.Run(name, func(b *testing.B) {
			com.RunBenchComService(b, name, srv)
		})
	}
}
