//go:build testing

// SPDX-FileCopyrightText: © 2023 Siemens AG
// SPDX-License-Identifier: MIT

// Package tls_test provides entry points for tests and benchmarks of
// communication services that use TLS authentication.
package tls_test

import (
	"testing"

	"github.com/coatyio/dda/config"
	"github.com/coatyio/dda/dda/test/com"
	"github.com/coatyio/dda/testdata"
)

var testServices = map[string]config.ConfigComService{
	"MQTT5-QoS0-TLSAuth": {
		Protocol: "mqtt5",
		Url:      "tls://localhost:1901",
		Auth: config.AuthOptions{
			Method: "tls",
			Cert:   "../../../../testdata/certs/dda-client-cert.pem",
			Key:    "../../../../testdata/certs/dda-client-key.pem",
			Verify: false,
		},
		Opts: map[string]any{
			"debug":          true, // enable paho debug logging for task test-log
			"strictClientId": true,
			"qos":            0,
		},
	},
}

var testComSetup = make(testdata.CommunicationSetup)

func init() {
	testComSetup["mqtt5"] = &testdata.CommunicationSetupOptions{
		SetupOpts: map[string]any{
			"brokerPort":          1901,
			"brokerWsPort":        0, // WebSocket connection setup not needed
			"brokerCert":          "../../../../testdata/certs/dda-server-cert.pem",
			"brokerKey":           "../../../../testdata/certs/dda-server-key.pem",
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
