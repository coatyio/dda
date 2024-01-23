//go:build testing

// SPDX-FileCopyrightText: © 2024 Siemens AG
// SPDX-License-Identifier: MIT

// Package state provides end-to-end test and benchmark functions of the gRPC
// client state API.
package state

import (
	"context"
	"fmt"
	"testing"

	"github.com/coatyio/dda/apis/grpc/stubs/golang/state"
	"github.com/coatyio/dda/config"
	"github.com/coatyio/dda/dda/test/grpc"
	"github.com/coatyio/dda/testdata"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// RunTestGrpc runs all tests on a given gRPC Client communication API.
func RunTestGrpc(t *testing.T, cluster string, clientApis config.ConfigApis, srv config.ConfigStateService, req config.ConfigComService) {
	cfg1 := testdata.NewStateConfig(cluster, srv, req)
	cfg1.Apis = clientApis
	cfg1.Services.State.Bootstrap = true // first member creates state synchronization group as leader
	dda1, err := testdata.OpenDdaWithConfig(cfg1)
	if !assert.NoError(t, err) {
		assert.FailNow(t, "Couldn't open bootstrapping dda1")
	}

	client1, closeClient1, err := grpc.OpenGrpcClientState(cfg1.Apis.Grpc.Address, cfg1.Apis.Cert)
	if !assert.NoError(t, err) {
		assert.FailNow(t, "Couldn't open client for dda1")
	}

	defer testdata.CloseDda(dda1)
	defer closeClient1()

	if srv.Disabled {
		t.Run("operations fail on disabled service", func(t *testing.T) {
			ack, err := client1.ProposeInput(context.Background(), &state.Input{Op: state.InputOperation_INPUT_OPERATION_SET, Key: "foo", Value: []byte("foo")})
			assert.Nil(t, ack)
			assert.Error(t, err)
			assert.Equal(t, codes.Unavailable, status.Code(err))

			streamSc, err := client1.ObserveStateChange(context.Background(), &state.ObserveStateChangeParams{})
			assert.NotNil(t, streamSc)
			assert.NoError(t, err)
			in, err := streamSc.Recv()
			assert.Nil(t, in)
			assert.Equal(t, codes.Unavailable, status.Code(err))

			streamMc, err := client1.ObserveMembershipChange(context.Background(), &state.ObserveMembershipChangeParams{})
			assert.NotNil(t, streamMc)
			assert.NoError(t, err)
			mc, err := streamMc.Recv()
			assert.Nil(t, mc)
			assert.Equal(t, codes.Unavailable, status.Code(err))
		})

		return
	}

	var memberChangeStream1 state.StateService_ObserveMembershipChangeClient
	memberChangeCtx1, memberChangeCancel1 := context.WithCancel(context.Background())
	defer memberChangeCancel1()

	t.Run("ObserveMemberChange on leader dda1", func(t *testing.T) {
		memberChangeStream1, err = client1.ObserveMembershipChange(memberChangeCtx1, &state.ObserveMembershipChangeParams{})
		assert.NotNil(t, memberChangeStream1)
		assert.NoError(t, err)
		mc, err := memberChangeStream1.Recv()
		assert.NotNil(t, mc)
		assert.NoError(t, err)
		assert.Equal(t, dda1.Identity().Id, mc.Id)
		assert.True(t, mc.Joined)
	})

	var stateChangeStream1 state.StateService_ObserveStateChangeClient
	stateChangeCtx1, stateChangeCancel1 := context.WithCancel(context.Background())
	defer stateChangeCancel1()

	t.Run("ObserveStateChange on leader dda1", func(t *testing.T) {
		stateChangeStream1, err = client1.ObserveStateChange(stateChangeCtx1, &state.ObserveStateChangeParams{})
		assert.NotNil(t, stateChangeStream1)
		assert.NoError(t, err)
	})

	t.Run("ProposeInput InputOpSet on leader dda1", func(t *testing.T) {
		val := []byte("foo")
		in := &state.Input{Op: state.InputOperation_INPUT_OPERATION_SET, Key: "foo", Value: val}
		ack, err := client1.ProposeInput(context.Background(), in)
		assert.NotNil(t, ack)
		assert.NoError(t, err)
		sc, err := stateChangeStream1.Recv()
		assert.NoError(t, err)
		assert.True(t, proto.Equal(sc, in))
	})

	cfg2 := testdata.NewStateConfig(cluster, srv, req)
	cfg2.Apis = clientApis
	cfg2.Apis.Grpc.Address = grpc.NextAddress(cfg2.Apis.Grpc.Address)
	dda2, err := testdata.OpenDdaWithConfig(cfg2)
	if !assert.NoError(t, err) {
		assert.FailNow(t, "Couldn't open follower dda2")
	}

	client2, closeClient2, err := grpc.OpenGrpcClientState(cfg1.Apis.Grpc.Address, cfg2.Apis.Cert)
	if !assert.NoError(t, err) {
		assert.FailNow(t, "Couldn't open client for dda2")
	}

	defer testdata.CloseDda(dda2)
	defer closeClient2()

	t.Run("Membership change by dda2 on leader dda1", func(t *testing.T) {
		mc, err := memberChangeStream1.Recv()
		assert.NotNil(t, mc)
		assert.NoError(t, err)
		assert.Equal(t, dda2.Identity().Id, mc.Id)
		assert.True(t, mc.Joined)
	})

	var memberChangeStream2 state.StateService_ObserveMembershipChangeClient
	memberChangeCtx2, memberChangeCancel2 := context.WithCancel(context.Background())
	defer memberChangeCancel2()

	t.Run("ObserveMemberChange on follower dda2", func(t *testing.T) {
		memberChangeStream2, err = client2.ObserveMembershipChange(memberChangeCtx2, &state.ObserveMembershipChangeParams{})
		assert.NotNil(t, memberChangeStream2)
		assert.NoError(t, err)
		expectedIds := []string{dda1.Identity().Id, dda2.Identity().Id}
		mc1, err := memberChangeStream2.Recv()
		assert.NotNil(t, mc1)
		assert.NoError(t, err)
		assert.Contains(t, expectedIds, mc1.Id)
		assert.True(t, mc1.Joined)
		mc2, err := memberChangeStream2.Recv()
		assert.NotNil(t, mc2)
		assert.NoError(t, err)
		assert.Contains(t, expectedIds, mc2.Id)
		assert.True(t, mc2.Joined)
		assert.NotEqual(t, mc1.Id, mc2.Id)
	})

	var stateChangeStream2 state.StateService_ObserveStateChangeClient
	stateChangeCtx2, stateChangeCancel2 := context.WithCancel(context.Background())
	defer stateChangeCancel2()

	t.Run("ObserveStateChange on follower dda2", func(t *testing.T) {
		stateChangeStream2, err = client2.ObserveStateChange(stateChangeCtx2, &state.ObserveStateChangeParams{})
		assert.NotNil(t, stateChangeStream2)
		assert.NoError(t, err)
		sc, err := stateChangeStream2.Recv()
		assert.NoError(t, err)
		assert.True(t, proto.Equal(sc, &state.Input{Op: state.InputOperation_INPUT_OPERATION_SET, Key: "foo", Value: []byte("foo")}))
	})

	t.Run("Propose InputOpDelete on follower dda2", func(t *testing.T) {
		in := &state.Input{Op: state.InputOperation_INPUT_OPERATION_DELETE, Key: "foo"}
		ack, err := client2.ProposeInput(context.Background(), in)
		assert.NotNil(t, ack)
		assert.NoError(t, err)
		sc, err := stateChangeStream2.Recv()
		assert.NoError(t, err)
		assert.True(t, proto.Equal(sc, in))
		sc, err = stateChangeStream1.Recv()
		assert.NoError(t, err)
		assert.True(t, proto.Equal(sc, in))
	})
}

// RunBenchGrpc runs all benchmarks on a given gRPC Client communication API.
func RunBenchGrpc(b *testing.B, cluster string, clientApis config.ConfigApis, srv config.ConfigStateService, req config.ConfigComService) {
	// This benchmark with setup will not be measured itself and called once for
	// each combination of client API and communication service with b.N=1

	cfg1 := testdata.NewStateConfig(cluster, srv, req)
	cfg1.Apis = clientApis
	cfg1.Services.State.Bootstrap = true // first member creates state synchronization group as leader
	dda1, err := testdata.OpenDdaWithConfig(cfg1)
	if err != nil {
		b.FailNow()
	}

	client1, closeClient1, err := grpc.OpenGrpcClientState(cfg1.Apis.Grpc.Address, cfg1.Apis.Cert)
	if err != nil {
		b.FailNow()
	}

	defer testdata.CloseDda(dda1)
	defer closeClient1()

	cfg2 := testdata.NewStateConfig(cluster, srv, req)
	cfg2.Apis = clientApis
	cfg2.Apis.Grpc.Address = grpc.NextAddress(cfg2.Apis.Grpc.Address)
	dda2, err := testdata.OpenDdaWithConfig(cfg2)
	if err != nil {
		b.FailNow()
	}

	client2, closeClient2, err := grpc.OpenGrpcClientState(cfg1.Apis.Grpc.Address, cfg2.Apis.Cert)
	if err != nil {
		b.FailNow()
	}

	defer testdata.CloseDda(dda2)
	defer closeClient2()

	// This subbenchmark will be measured and invoked multiple times with
	// growing b.N
	b.Run("ProposeInput by leader", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			key := fmt.Sprintf("leader%d", n)
			in := &state.Input{Op: state.InputOperation_INPUT_OPERATION_SET, Key: key, Value: []byte(key)}
			_, err := client1.ProposeInput(context.Background(), in)
			if err != nil {
				b.FailNow()
			}
		}
	})

	// This subbenchmark will be measured and invoked multiple times with
	// growing b.N
	b.Run("ProposeInput by follower", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			key := fmt.Sprintf("follower%d", n)
			in := &state.Input{Op: state.InputOperation_INPUT_OPERATION_SET, Key: key, Value: []byte(key)}
			_, err := client2.ProposeInput(context.Background(), in)
			if err != nil {
				b.FailNow()
			}
		}
	})
}
