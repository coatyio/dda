//go:build testing

// SPDX-FileCopyrightText: © 2023 Siemens AG
// SPDX-License-Identifier: MIT

// Package store provides end-to-end test and benchmark functions of the gRPC
// client store API.
package store

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"testing"

	"github.com/coatyio/dda/apis/grpc/stubs/golang/store"
	"github.com/coatyio/dda/config"
	"github.com/coatyio/dda/dda/test/grpc"
	"github.com/coatyio/dda/testdata"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// RunTestGrpc runs all tests on a given gRPC Client store API.
func RunTestGrpc(t *testing.T, cluster string, clientApis config.ConfigApis, srv config.ConfigStoreService) {
	cfg := testdata.NewStoreConfig(srv)
	cfg.Apis = clientApis
	dda, err := testdata.OpenDdaWithConfig(cfg)
	if !assert.NoError(t, err) {
		assert.FailNow(t, "Couldn't open DDA")
	}

	client, closeClient, err := grpc.OpenGrpcClientStore(cfg.Apis.Grpc.Address, cfg.Apis.Cert)
	if !assert.NoError(t, err) {
		assert.FailNow(t, "Couldn't open DDA Client")
	}

	defer testdata.CloseDda(dda)
	defer closeClient()

	if srv.Disabled {
		t.Run("operations fail on disabled service", func(t *testing.T) {
			val, err := client.Get(context.Background(), &store.Key{Key: "foo"})
			assert.Nil(t, val)
			assert.Error(t, err)
			assert.Equal(t, codes.Unavailable, status.Code(err))
			ack, err := client.Set(context.Background(), &store.KeyValue{Key: "foo", Value: []byte("foo")})
			assert.Nil(t, ack)
			assert.Error(t, err)
			assert.Equal(t, codes.Unavailable, status.Code(err))
			ack, err = client.Delete(context.Background(), &store.Key{Key: "foo"})
			assert.Nil(t, ack)
			assert.Error(t, err)
			assert.Equal(t, codes.Unavailable, status.Code(err))
			ack, err = client.DeleteRange(context.Background(), &store.Range{Start: "foo", End: "bar"})
			assert.Nil(t, ack)
			assert.Error(t, err)
			assert.Equal(t, codes.Unavailable, status.Code(err))
			ack, err = client.DeleteAll(context.Background(), &store.DeleteAllParams{})
			assert.Nil(t, ack)
			assert.Error(t, err)
			assert.Equal(t, codes.Unavailable, status.Code(err))
			stream, err := client.ScanPrefix(context.Background(), &store.Key{Key: "foo"})
			assert.NotNil(t, stream)
			assert.NoError(t, err)
			kv, err := stream.Recv()
			assert.Nil(t, kv)
			assert.Equal(t, codes.Unavailable, status.Code(err))
			stream1, err := client.ScanRange(context.Background(), &store.Range{Start: "foo", End: "bar"})
			assert.NotNil(t, stream1)
			assert.NoError(t, err)
			kv, err = stream1.Recv()
			assert.Nil(t, kv)
			assert.Equal(t, codes.Unavailable, status.Code(err))
		})

		return
	}

	t.Run("Get non-existent key", func(t *testing.T) {
		val, err := client.Get(context.Background(), &store.Key{Key: "foo"})
		assert.NoError(t, err)
		assert.NotNil(t, val)
		assert.Equal(t, []byte(nil), val.Value) // explicitly present nil byte slice, not empty byte slice []byte{}
	})

	t.Run("Delete non-existent key", func(t *testing.T) {
		ack, err := client.Delete(context.Background(), &store.Key{Key: "foo"})
		assert.True(t, proto.Equal(&store.Ack{}, ack))
		assert.NoError(t, err)
	})

	t.Run("Set nil value", func(t *testing.T) {
		ack, err := client.Set(context.Background(), &store.KeyValue{Key: "foo", Value: []byte(nil)})
		assert.Nil(t, ack)
		assert.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("Set non-nil values", func(t *testing.T) {
		ack, err := client.Set(context.Background(), &store.KeyValue{Key: "prefix", Value: []byte("prefix")})
		assert.True(t, proto.Equal(&store.Ack{}, ack))
		assert.NoError(t, err)
		ack, err = client.Set(context.Background(), &store.KeyValue{Key: "prefix2", Value: []byte("prefix2")})
		assert.True(t, proto.Equal(&store.Ack{}, ack))
		assert.NoError(t, err)
		ack, err = client.Set(context.Background(), &store.KeyValue{Key: "prefix1", Value: []byte("prefix1")})
		assert.True(t, proto.Equal(&store.Ack{}, ack))
		assert.NoError(t, err)
	})

	t.Run("Get non-nil values", func(t *testing.T) {
		val, err := client.Get(context.Background(), &store.Key{Key: "prefix"})
		assert.Equal(t, []byte("prefix"), val.Value)
		assert.NoError(t, err)
		val, err = client.Get(context.Background(), &store.Key{Key: "prefix1"})
		assert.Equal(t, []byte("prefix1"), val.Value)
		assert.NoError(t, err)
		val, err = client.Get(context.Background(), &store.Key{Key: "prefix2"})
		assert.Equal(t, []byte("prefix2"), val.Value)
		assert.NoError(t, err)
	})

	t.Run("ScanPrefix on prefix*", func(t *testing.T) {
		stream, err := client.ScanPrefix(context.Background(), &store.Key{Key: "prefix"})
		assert.NotNil(t, stream)
		assert.NoError(t, err)

		kv, err := stream.Recv()
		assert.NoError(t, err)
		assert.Equal(t, "prefix", kv.GetKey())
		assert.Equal(t, []byte("prefix"), kv.GetValue())

		kv, err = stream.Recv()
		assert.NoError(t, err)
		assert.Equal(t, "prefix1", kv.GetKey())
		assert.Equal(t, []byte("prefix1"), kv.GetValue())

		kv, err = stream.Recv()
		assert.NoError(t, err)
		assert.Equal(t, "prefix2", kv.GetKey())
		assert.Equal(t, []byte("prefix2"), kv.GetValue())

		kv, err = stream.Recv()
		assert.Nil(t, kv)
		assert.Error(t, io.EOF, err)
	})

	t.Run("ScanPrefix on prefix* stopped", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		stream, err := client.ScanPrefix(ctx, &store.Key{Key: "prefix"})
		assert.NotNil(t, stream)
		assert.NoError(t, err)

		kv, err := stream.Recv()
		assert.NoError(t, err)
		assert.Equal(t, "prefix", kv.GetKey())
		assert.Equal(t, []byte("prefix"), kv.GetValue())

		cancel() // stop scanning on server side (asynchronously)

		kv, err = stream.Recv()
		assert.Error(t, io.EOF, err)
		assert.Condition(t, func() bool { // may or may not receive prefix1 along with error
			return kv == nil || (kv.GetKey() == "prefix1" && bytes.Equal(kv.GetValue(), []byte("prefix1")))
		})
	})

	t.Run("ScanRange [prefix,prefix2)", func(t *testing.T) {
		stream, err := client.ScanRange(context.Background(), &store.Range{Start: "prefix", End: "prefix2"})
		assert.NotNil(t, stream)
		assert.NoError(t, err)

		kv, err := stream.Recv()
		assert.NoError(t, err)
		assert.Equal(t, "prefix", kv.GetKey())
		assert.Equal(t, []byte("prefix"), kv.GetValue())

		kv, err = stream.Recv()
		assert.NoError(t, err)
		assert.Equal(t, "prefix1", kv.GetKey())
		assert.Equal(t, []byte("prefix1"), kv.GetValue())

		kv, err = stream.Recv()
		assert.Nil(t, kv)
		assert.Error(t, io.EOF, err)
	})

	t.Run("ScanRange [prefix,prefix2) stopped", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		stream, err := client.ScanRange(ctx, &store.Range{Start: "prefix", End: "prefix2"})
		assert.NotNil(t, stream)
		assert.NoError(t, err)

		kv, err := stream.Recv()
		assert.NoError(t, err)
		assert.Equal(t, "prefix", kv.GetKey())
		assert.Equal(t, []byte("prefix"), kv.GetValue())

		cancel() // stop scanning on server side (asynchronously)

		kv, err = stream.Recv()
		assert.Error(t, io.EOF, err)
		assert.Condition(t, func() bool { // may or may not receive prefix1 along with error
			return kv == nil || (kv.GetKey() == "prefix1" && bytes.Equal(kv.GetValue(), []byte("prefix1")))
		})
	})

	t.Run("Delete value", func(t *testing.T) {
		ack, err := client.Delete(context.Background(), &store.Key{Key: "prefix"})
		assert.True(t, proto.Equal(&store.Ack{}, ack))
		assert.NoError(t, err)
	})

	t.Run("DeleteRange [prefix1,prefix2)", func(t *testing.T) {
		ack, err := client.DeleteRange(context.Background(), &store.Range{Start: "prefix1", End: "prefix2"})
		assert.True(t, proto.Equal(&store.Ack{}, ack))
		assert.NoError(t, err)
	})

	t.Run("DeleteAll", func(t *testing.T) {
		ack, err := client.DeleteAll(context.Background(), &store.DeleteAllParams{})
		assert.True(t, proto.Equal(&store.Ack{}, ack))
		assert.NoError(t, err)
	})
}

// RunBenchGrpc runs all benchmarks on a given gRPC Client store API.
func RunBenchGrpc(b *testing.B, cluster string, clientApis config.ConfigApis, srv config.ConfigStoreService) {
	// This benchmark with setup will not be measured itself and called once for
	// each combination of client API and communication service with b.N=1

	cfg := testdata.NewStoreConfig(srv)
	cfg.Apis = clientApis
	dda, err := testdata.OpenDdaWithConfig(cfg)
	if err != nil {
		b.FailNow()
	}

	client, closeClient, err := grpc.OpenGrpcClientStore(cfg.Apis.Grpc.Address, cfg.Apis.Cert)
	if err != nil {
		b.FailNow()
	}

	defer testdata.CloseDda(dda)
	defer closeClient()

	kvsc := 10000000 // number of keys
	kvsclen := len(fmt.Sprintf("%d", kvsc))
	kvs := make([]struct {
		k string
		v []byte
	}, kvsc)
	for i := 0; i < kvsc; i++ {
		s := fmt.Sprintf("%0*d", kvsclen, i)
		kvs[i] = struct {
			k string
			v []byte
		}{s, []byte(s)}
	}

	rw := rand.New(rand.NewSource(int64(kvsc))) // fixed writing source

	// This subbenchmark will be measured and invoked multiple times with
	// growing b.N
	b.Run("Random write", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			kv := kvs[rw.Intn(kvsc)]
			if _, err := client.Set(context.Background(), &store.KeyValue{Key: kv.k, Value: kv.v}); err != nil {
				b.FailNow()
			}
		}
	})

	rr := rand.New(rand.NewSource(int64(kvsc))) // fixed writing source

	// This subbenchmark will be measured and invoked multiple times with
	// growing b.N
	b.Run("Random read", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			kv := kvs[rr.Intn(kvsc)]
			if _, err := client.Get(context.Background(), &store.Key{Key: kv.k}); err != nil {
				b.FailNow()
			}
		}
	})

}
