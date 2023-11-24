// SPDX-FileCopyrightText: © 2023 Siemens AG
// SPDX-License-Identifier: MIT

package grpc

import (
	"crypto/tls"
	"fmt"
	"net/http"

	"github.com/coatyio/dda/config"
	"github.com/coatyio/dda/plog"
	rpcweb "github.com/improbable-eng/grpc-web/go/grpcweb"
	rpc "google.golang.org/grpc"
)

// grpcWebServer realizes a gRCP-Web HTTP proxy server that routes incoming
// gRPC-Web requests to the DDA gRPC server.
type grpcWebServer struct {
	httpSrv     *http.Server
	wrappedGrpc *rpcweb.WrappedGrpcServer
}

func (s *grpcWebServer) openWebServer(rs *rpc.Server, cfg *config.Config) error {
	if cfg.Apis.GrpcWeb.Disabled {
		return nil
	}

	// Set up gRPC-Web http server that wraps the gRPC server.

	options := []rpcweb.Option{}
	originFunc := s.makeHttpOriginFunc(cfg.Apis.GrpcWeb.AccessControlAllowOrigin)
	if originFunc != nil {
		options = append(options, rpcweb.WithOriginFunc(originFunc))
	}

	s.wrappedGrpc = rpcweb.WrapServer(rs, options...)

	var tlsConfig *tls.Config
	if cfg.Apis.Cert != "" && cfg.Apis.Key != "" {
		cert, err := tls.LoadX509KeyPair(cfg.Apis.Cert, cfg.Apis.Key)
		if err != nil {
			return fmt.Errorf("invalid or missing PEM file in DDA configuration under 'apis.cert/.key' : %w", err)
		}
		tlsConfig = &tls.Config{
			MinVersion:   tls.VersionTLS12,
			Certificates: []tls.Certificate{cert},
			ClientAuth:   tls.NoClientCert,
		}
	}
	webAddress := cfg.Apis.GrpcWeb.Address
	s.httpSrv = &http.Server{
		Addr:              webAddress,
		ErrorLog:          plog.WithPrefix("http.Server "),
		TLSConfig:         tlsConfig,
		ReadHeaderTimeout: 0, // no timeout to enable long-lived responses
		ReadTimeout:       0, // no timeout to enable long-lived responses
		WriteTimeout:      0, // no timeout to enable long-lived responses
		IdleTimeout:       0, // no timeout waiting for next request with keep-live
	}

	s.httpSrv.Handler = http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		// plog.Printf("gRPC-Web ServeHTTP %+v\n", req)
		s.wrappedGrpc.ServeHTTP(resp, req)
	})

	plog.Printf("Open gRPC-Web http server listening on address %s...\n", webAddress)

	go func() {
		if tlsConfig == nil {
			if err := s.httpSrv.ListenAndServe(); err != http.ErrServerClosed {
				plog.Printf("Unexpected gRPC-Web http server error: %v", err)
			}
		} else {
			if err := s.httpSrv.ListenAndServeTLS("", ""); err != http.ErrServerClosed {
				plog.Printf("Unexpected gRPC-Web https server error: %v", err)
			}
		}
	}()

	return nil
}

func (s *grpcWebServer) closeWebServer() {
	s.wrappedGrpc = nil
	if s.httpSrv != nil {
		if err := s.httpSrv.Close(); err != nil {
			plog.Printf("Closed gRPC-Web server with error: %v", err)
		} else {
			plog.Println("Closed gRPC-Web server")
		}
		s.httpSrv = nil
	}
}

func (s *grpcWebServer) makeHttpOriginFunc(origins []string) func(string) bool {
	if len(origins) == 0 {
		return func(origin string) bool {
			return true // all origins are allowed
		}
	}
	originSet := make(map[string]struct{}, len(origins))
	for _, origin := range origins {
		originSet[origin] = struct{}{}
	}
	return func(origin string) bool {
		_, ok := originSet[origin]
		return ok
	}
}
