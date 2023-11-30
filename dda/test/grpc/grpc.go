//go:build testing

// SPDX-FileCopyrightText: © 2023 Siemens AG
// SPDX-License-Identifier: MIT

// Package grpc provides utility functions for end-to-end test and benchmark
// functions of all gRPC client APIs.
package grpc

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/coatyio/dda/apis/grpc/stubs/golang/com"
	"github.com/coatyio/dda/apis/grpc/stubs/golang/store"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// NextAddress increments the port number of a given address and returns it.
func NextAddress(address string) string {
	i := strings.LastIndex(address, ":")
	if i != -1 {
		port, _ := strconv.ParseInt(address[i+1:], 10, 64)
		return fmt.Sprintf("%s:%d", address[:i], port+1)
	}
	port, _ := strconv.ParseInt(address, 10, 64)
	return fmt.Sprintf("%d", port+1)
}

// OpenGrpcClientCom creates and connects a gRPC client on the given address and
// returns the communication service client and a function to close it later.
func OpenGrpcClientCom(address, caCertFile string) (com.ComServiceClient, func(), error) {
	conn, closer, err := OpenGrpcClient(address, caCertFile)
	if err != nil {
		return nil, nil, err
	}
	return com.NewComServiceClient(conn), closer, err
}

// OpenGrpcClientStore creates and connects a gRPC client on the given address
// and returns the store service client and a function to close it later.
func OpenGrpcClientStore(address, caCertFile string) (store.StoreServiceClient, func(), error) {
	conn, closer, err := OpenGrpcClient(address, caCertFile)
	if err != nil {
		return nil, nil, err
	}
	return store.NewStoreServiceClient(conn), closer, err
}

func OpenGrpcClient(address, caCertFile string) (*grpc.ClientConn, func(), error) {
	var opts []grpc.DialOption
	if caCertFile != "" {
		creds, err := credentials.NewClientTLSFromFile(caCertFile, "")
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create gRPC Client TLS credentials: %v", err)
		}
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.Dial(address, opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to dial gRPC Client on address %s: %v", address, err)
	}
	return conn, func() { defer conn.Close() }, nil
}
