// SPDX-FileCopyrightText: © 2023 Siemens AG
// SPDX-License-Identifier: MIT

// Package broker provides an MQTT 5 broker listening on port 1883 and WS port
// 9883. It is intended to be used for development and testing purposes. This
// package is not included in any DDA binary.
package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/coatyio/dda/testdata"
)

func main() {
	setup := make(testdata.CommunicationSetup)
	setup["mqtt5"] = &testdata.CommunicationSetupOptions{
		SetupOpts: map[string]any{
			"brokerPort":          1883,
			"brokerWsPort":        9883,
			"brokerLogInfo":       true,
			"brokerLogDebugHooks": false,
		},
	}
	testdata.RunWithComSetup(func() {
		sigCh := make(chan os.Signal, 2)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		<-sigCh
	}, setup)
}
