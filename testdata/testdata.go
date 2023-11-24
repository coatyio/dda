// SPDX-FileCopyrightText: © 2023 Siemens AG
// SPDX-License-Identifier: MIT

// Package testdata provides common building blocks at the package level that
// may be imported into the TestMain function of testing packages. It enables
// end-to-end testing and benchmarking of peripheral DDA services over supported
// communication bindings.
//
// This package MUST only be imported and used inside Go testing files so that
// its contents are omitted from a DDA binary.
package testdata

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/coatyio/dda/config"
	"github.com/coatyio/dda/dda"
	"github.com/coatyio/dda/plog"
	comapi "github.com/coatyio/dda/services/com/api"
	cmqtt "github.com/coatyio/dda/services/com/mqtt5"
	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/hooks/debug"
	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/mochi-mqtt/server/v2/packets"
)

// GetTestName returns a name for a test or benchmark joined from the given
// names.
func GetTestName(names ...string) string {
	return strings.Join(names, "-")
}

// PubSubSetupOptions defines options for setting up a pub-sub communication
// network.
type PubSubSetupOptions struct {

	// Setup options for a specific pub-sub communication infrastructure (e.g. a
	// broker).
	SetupOpts map[string]any

	// A function configured by RunMainWithSetup to allow test code to
	// disconnect a communication binding while testing to force a reconnect and
	// resubscribe attempt. delay specifies how long to wait for the
	// communication binding to complete reconnection.
	DisconnectBindingFunc func(binding comapi.Api, delay time.Duration)
}

// PubSubCommunicationSetup defines options to set up pub-sub communication
// infrastructure, such as a broker, indexed by communication binding protocol
// name.
type PubSubCommunicationSetup map[string]*PubSubSetupOptions

// RunMain runs the tests without pub-sub Communication setup, then exits.
func RunMain(m *testing.M) {
	RunMainWithSetup(m, make(PubSubCommunicationSetup))
}

// RunMainWithSetup runs the tests with the given given pub-sub communication
// setup, then exits with the exit code returned by m.Run.
func RunMainWithSetup(m *testing.M, setup PubSubCommunicationSetup) {
	packageTeardown := TestSetup(setup)
	defer packageTeardown()
	m.Run()
}

// TestSetup sets up pub-sub end-to-end package testing and returns a function
// to tear down package testing.
//
// TestSetup sets up the pub-sub communication infrastructure, shuts it down on
// teardown, and provides functions to disconnect the communication binding from
// the pub-sub communication network in the *PubSubSetupOptions stuctures. It
// also enables or disables plog logging according to the DDA_TEST_LOG
// environment variable.
func TestSetup(setup PubSubCommunicationSetup) func() {
	ddaLog := os.Getenv("DDA_TEST_LOG") == "true"
	enableLog := func() {}
	if !ddaLog {
		enableLog = disableLog()
	}

	cleanupPubSub := make(map[string]func())
	for proto, val := range setup {
		switch proto {
		case "mqtt5":
			server := startMqtt5Broker(val.SetupOpts)
			val.DisconnectBindingFunc = func(bnd comapi.Api, delay time.Duration) {
				if server != nil {
					switch bnd := bnd.(type) {
					case *cmqtt.Mqtt5Binding:
						_ = disconnectMqtt5ClientByBroker(server, bnd.ClientId())
						time.Sleep(delay)
					}
				}
			}
			cleanupPubSub[proto] = func() {
				stopMqtt5Broker(server)
			}
		}
	}

	return func() {
		for _, cleanFunc := range cleanupPubSub {
			cleanFunc()
		}
		enableLog()
	}
}

// Create new Config with the given cluster, identity name, and communication
// service. By default, Client API services are disabled in the returned
// configuration.
func NewConfig(cluster string, identityName string, comSrv config.ConfigComService) *config.Config {
	cfg := config.New()
	cfg.Identity.Name = identityName
	cfg.Cluster = cluster
	cfg.Services.Com = comSrv
	cfg.Apis.Grpc.Disabled = true
	cfg.Apis.GrpcWeb.Disabled = true
	return cfg
}

// OpenDda creates and opens a new Dda with the configuration as returned by
// NewConfig.
func OpenDda(cluster string, identityName string, comSrv config.ConfigComService) (*dda.Dda, error) {
	// The cluster name is configured with the Name of the passed testing.T so
	// that DDA instances configured with this Config are isolated within the
	// given testing context.
	cfg := NewConfig(cluster, identityName, comSrv)
	return OpenDdaWithConfig(cfg)
}

// OpenDdaWithConfig creates and opens a new Dda with the given configuration.
func OpenDdaWithConfig(cfg *config.Config) (*dda.Dda, error) {
	dda, err := dda.New(cfg)
	if err != nil {
		return nil, err
	}
	err = dda.Open(3 * time.Second)
	if err != nil {
		return nil, err
	}
	return dda, nil
}

// CloseDda closes the given Dda.
func CloseDda(dda *dda.Dda) {
	dda.Close()
}

func disableLog() func() {
	plog.Disable()
	return func() {
		plog.Enable()
	}
}

// disconnectMqtt5ClientByBroker causes the MQTT broker to gracefully close the
// connection to the MQTT client with the given client ID so that the client
// attempts to reconnect and resubscribe.
func disconnectMqtt5ClientByBroker(server *mqtt.Server, clientId string) error {
	client, ok := server.Clients.Get(clientId)
	if !ok {
		server.Log.Info("Couldn't disconnect client %s from MQTT Server", clientId)
		return fmt.Errorf("unknown clientId %s", clientId)
	}
	return server.DisconnectClient(client, packets.CodeDisconnect)
}

func startMqtt5Broker(setupOpts map[string]any) *mqtt.Server {
	brokerPort, ok := setupOpts["brokerPort"].(int)
	if !ok {
		brokerPort = 1883
	}
	brokerWsPort, ok := setupOpts["brokerWsPort"].(int)
	if !ok {
		brokerWsPort = 9883
	}
	brokerLogInfo, ok := setupOpts["brokerLogInfo"].(bool)
	if !ok {
		brokerLogInfo = false
	}
	brokerLogDebugHooks, ok := setupOpts["brokerLogDebugHooks"].(bool)
	if !ok {
		brokerLogDebugHooks = false
	}
	brokerCert, ok := setupOpts["brokerCert"].(string)
	if !ok {
		brokerCert = ""
	}
	brokerKey, ok := setupOpts["brokerKey"].(string)
	if !ok {
		brokerKey = ""
	}
	brokerBasicAuth, ok := setupOpts["brokerBasicAuth"].(map[string]string)
	if !ok {
		brokerBasicAuth = nil
	}

	useTls := brokerCert != "" && brokerKey != ""

	plog.Printf("Starting MQTT Server on TCP port %d and WS Port %d with TLS %t", brokerPort, brokerWsPort, useTls)

	// By default, use ERROR level for server including hooks.
	slogLevel := slog.LevelError
	if brokerLogInfo {
		slogLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slogLevel,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				a = slog.Attr{} // Omit time attribute
			}
			return a
		},
	}))
	opts := mqtt.Options{Logger: logger}
	server := mqtt.New(&opts)

	if brokerBasicAuth != nil {
		// Restrict client connections to the given basic authentications.
		authRules := make(auth.AuthRules, 0, len(brokerBasicAuth))
		for user, pwd := range brokerBasicAuth {
			authRules = append(authRules, auth.AuthRule{
				Username: auth.RString(user),
				Password: auth.RString(pwd),
				Allow:    true,
			})
		}
		err := server.AddHook(new(auth.Hook), &auth.Options{
			Ledger: &auth.Ledger{Auth: authRules},
		})
		if err != nil {
			logger.Error("Couldn't start MQTT Server Auth Hook for testing", "error", err)
			os.Exit(1)
		}
	} else {
		// Allow all client connnctions.
		_ = server.AddHook(new(auth.AllowHook), nil)
	}

	// Debug packet flows (requires brokerLogInfo enabled).
	if brokerLogDebugHooks {
		dbgHook := new(debug.Hook)
		dbgHook.Log = logger
		err := server.AddHook(dbgHook, &debug.Options{
			ShowPacketData: true,
			ShowPings:      false,
			ShowPasswords:  false,
		})
		if err != nil {
			logger.Error("Couldn't start MQTT Server Debug Hooks for testing", "error", err)
			os.Exit(1)
		}
	}

	// Configure TLS if server certificate and private key are given.
	var tlsConfig *listeners.Config = nil
	if useTls {
		// Create TLS config for mutual-TLS use.
		cert, err := tls.LoadX509KeyPair(brokerCert, brokerKey)
		if err != nil {
			logger.Error("Couldn't start MQTT Server with TLS cert/key", "error", err)
			os.Exit(1)
		}
		tlsConfig = &listeners.Config{
			TLSConfig: &tls.Config{
				MinVersion:   tls.VersionTLS12,
				Certificates: []tls.Certificate{cert},
				ClientAuth:   tls.NoClientCert, // To support WS authenication by non-mutual TLS
			},
		}
	}

	// Create a TCP listener on a local testing address.
	addr := ":" + strconv.Itoa(brokerPort)
	tcp := listeners.NewTCP("t1", addr, tlsConfig)
	err := server.AddListener(tcp)
	if err != nil {
		logger.Error("Couldn't start MQTT Server on given TCP address for testing", "error", err, "address", addr)
		os.Exit(1)
	}

	// Create a WebSocket listener on a local testing address.
	if brokerWsPort > 0 {
		addr := ":" + strconv.Itoa(brokerWsPort)
		ws := listeners.NewWebsocket("ws1", addr, tlsConfig)
		err = server.AddListener(ws)
		if err != nil {
			logger.Error("Couldn't start MQTT Server on given WS address for testing", "error", err, "address", addr)
			os.Exit(1)
		}
	}

	err = server.Serve()
	if err != nil {
		logger.Error("Couldn't start MQTT Server for testing", "error", err)
		os.Exit(1)
	}

	return server
}

func stopMqtt5Broker(server *mqtt.Server) {
	if server != nil {
		_ = server.Close()
	}
}
