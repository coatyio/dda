//go:build testing

// SPDX-FileCopyrightText: © 2023 Siemens AG
// SPDX-License-Identifier: MIT

// Package grpc provides end-to-end test and benchmark functions for the gRPC
// client API to be tested with different authentication methods over different
// communication bindings.
package grpc

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"

	stubs "github.com/coatyio/dda/apis/grpc/stubs/golang"
	"github.com/coatyio/dda/config"
	"github.com/coatyio/dda/testdata"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

var metadata_dda_suback = "dda-suback"

func openGrpcClient(address, caCertFile string) (stubs.ComServiceClient, func(), error) {
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
	return stubs.NewComServiceClient(conn),
		func() { defer conn.Close() },
		nil
}

func nextAddress(address string) string {
	i := strings.LastIndex(address, ":")
	if i != -1 {
		port, _ := strconv.ParseInt(address[i+1:], 10, 64)
		return fmt.Sprintf("%s:%d", address[:i], port+1)
	}
	port, _ := strconv.ParseInt(address, 10, 64)
	return fmt.Sprintf("%d", port+1)
}

// Runs all tests on a given gRPC Client API for the given communication service
// and pub-sub communication setup.
func RunTestGrpc(t *testing.T, cluster string, clientApis config.ConfigApis, comSrv config.ConfigComService, testPubSubSetup testdata.PubSubCommunicationSetup) {
	t.Run("Open with invalid gRPC server certificate/key", func(t *testing.T) {
		cfgInvalid := testdata.NewConfig(cluster, "ddaInvalid", comSrv)
		cfgInvalid.Apis = clientApis
		cfgInvalid.Apis.Cert = "foo.pem"
		cfgInvalid.Apis.Key = "bar.pem"
		ddaInvalid, err := testdata.OpenDdaWithConfig(cfgInvalid)
		assert.Nil(t, ddaInvalid)
		if !assert.Error(t, err) {
			assert.FailNow(t, "Could open invalid DDA")
		}
	})

	cfgPub := testdata.NewConfig(cluster, "ddaPub", comSrv)
	cfgPub.Apis = clientApis
	ddaPub, err := testdata.OpenDdaWithConfig(cfgPub)
	if !assert.NoError(t, err) {
		assert.FailNow(t, "Couldn't open DDA Pub")
	}

	cfgSub1 := testdata.NewConfig(cluster, "ddaSub1", comSrv)
	cfgSub1.Apis = clientApis
	cfgSub1.Apis.Grpc.Address = nextAddress(cfgSub1.Apis.Grpc.Address)
	ddaSub1, err := testdata.OpenDdaWithConfig(cfgSub1)
	if !assert.NoError(t, err) {
		assert.FailNow(t, "Couldn't open DDA Sub1")
	}

	cfgSub2 := testdata.NewConfig(cluster, "ddaSub2", comSrv)
	cfgSub2.Apis = cfgSub1.Apis
	cfgSub2.Apis.Grpc.Address = nextAddress(cfgSub2.Apis.Grpc.Address)
	ddaSub2, err := testdata.OpenDdaWithConfig(cfgSub2)
	if !assert.NoError(t, err) {
		assert.FailNow(t, "Couldn't open DDA Sub2")
	}

	// Use DDA test server certificates (with CA) to validate server connections by clients.
	clientPub, closeClientPub, err := openGrpcClient(cfgPub.Apis.Grpc.Address, cfgPub.Apis.Cert)
	if !assert.NoError(t, err) {
		assert.FailNow(t, "Couldn't open DDA Client Pub")
	}
	clientSub1, closeClientSub1, err := openGrpcClient(cfgSub1.Apis.Grpc.Address, cfgSub1.Apis.Cert)
	if !assert.NoError(t, err) {
		assert.FailNow(t, "Couldn't open DDA Client Sub1")
	}
	clientSub2, closeClientSub2, err := openGrpcClient(cfgSub2.Apis.Grpc.Address, cfgSub2.Apis.Cert)
	if !assert.NoError(t, err) {
		assert.FailNow(t, "Couldn't open DDA Client Sub2")
	}

	defer testdata.CloseDda(ddaPub)
	defer testdata.CloseDda(ddaSub1)
	defer testdata.CloseDda(ddaSub2)

	defer closeClientPub()
	defer closeClientSub1()
	defer closeClientSub2()

	evt := &stubs.Event{
		Type:            "hello",
		Id:              "42",
		Source:          "pub",
		Data:            []byte(`{"foo": 1, "bar": "baz"}`),
		DataContentType: "hellopub",
	}

	t.Run("PublishEvent with invalid Type", func(t *testing.T) {
		evtType := evt.Type
		evt.Type = "+-*"
		defer func() { evt.Type = evtType }()
		ack, err := clientPub.PublishEvent(context.Background(), evt)
		assert.Nil(t, ack)
		assert.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})
	t.Run("PublishEvent with empty Type", func(t *testing.T) {
		evtType := evt.Type
		evt.Type = ""
		defer func() { evt.Type = evtType }()
		ack, err := clientPub.PublishEvent(context.Background(), evt)
		assert.Nil(t, ack)
		assert.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})
	t.Run("PublishEvent with empty Id", func(t *testing.T) {
		evtId := evt.Id
		evt.Id = ""
		defer func() { evt.Id = evtId }()
		ack, err := clientPub.PublishEvent(context.Background(), evt)
		assert.Nil(t, ack)
		assert.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})
	t.Run("PublishEvent with empty Source", func(t *testing.T) {
		evtSrc := evt.Source
		evt.Source = ""
		defer func() { evt.Source = evtSrc }()
		ack, err := clientPub.PublishEvent(context.Background(), evt)
		assert.Nil(t, ack)
		assert.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})
	t.Run("SubscribeEvent with invalid Filter", func(t *testing.T) {
		stream, err := clientSub1.SubscribeEvent(context.Background(), &stubs.SubscriptionFilter{Type: "foo/bar"})
		assert.NoError(t, err)

		_, err = stream.Recv()
		assert.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})
	t.Run("Publish-SubscribeEvent", func(t *testing.T) {
		ctx1, cancel1 := context.WithCancel(context.Background())
		defer cancel1()
		stream1, err := clientSub1.SubscribeEvent(ctx1, &stubs.SubscriptionFilter{Type: evt.Type})
		assert.NoError(t, err)
		ctx2, cancel2 := context.WithCancel(context.Background())
		defer cancel2()
		stream2, err := clientSub2.SubscribeEvent(ctx2, &stubs.SubscriptionFilter{Type: evt.Type})
		assert.NoError(t, err)
		ctx3, cancel3 := context.WithCancel(context.Background())
		defer cancel3()
		stream3, err := clientSub2.SubscribeEvent(ctx3, &stubs.SubscriptionFilter{Type: evt.Type})
		assert.NoError(t, err)

		md1, err1 := stream1.Header() // await dda-suback
		assert.NoError(t, err1)
		assert.Contains(t, md1, metadata_dda_suback)
		md2, err2 := stream2.Header() // await dda-suback
		assert.NoError(t, err2)
		assert.Contains(t, md2, metadata_dda_suback)
		md3, err3 := stream3.Header() // await dda-suback
		assert.NoError(t, err3)
		assert.Contains(t, md3, metadata_dda_suback)

		ack, err := clientPub.PublishEvent(context.Background(), evt)
		assert.NoError(t, err)
		assert.True(t, proto.Equal(&stubs.Ack{}, ack))

		rcv, err := stream1.Recv()
		assert.NoError(t, err)
		assert.True(t, proto.Equal(evt, rcv))

		rcv, err = stream2.Recv()
		assert.NoError(t, err)
		assert.Equal(t, evt.Type, rcv.GetType())
		assert.Equal(t, evt.Id, rcv.GetId())
		assert.Equal(t, evt.Source, rcv.GetSource())
		assert.Equal(t, evt.Data, rcv.GetData())
		assert.Equal(t, evt.DataContentType, rcv.GetDataContentType())

		rcv, err = stream3.Recv()
		assert.NoError(t, err)
		assert.Equal(t, evt.Type, rcv.GetType())
		assert.Equal(t, evt.Id, rcv.GetId())
		assert.Equal(t, evt.Source, rcv.GetSource())
		assert.Equal(t, evt.Data, rcv.GetData())
		assert.Equal(t, evt.DataContentType, rcv.GetDataContentType())
	})

	act := &stubs.Action{
		Type:   "echo",
		Id:     "42",
		Source: "pub",
		Params: []byte(`[1, 2, 3, 4, 5]`),
	}

	t.Run("PublishAction with invalid Type", func(t *testing.T) {
		actType := act.Type
		act.Type = "+-*"
		defer func() { act.Type = actType }()
		stream, err := clientPub.PublishAction(context.Background(), act)
		assert.NotNil(t, stream)
		assert.NoError(t, err)
		rcv, err := stream.Recv()
		assert.Nil(t, rcv)
		assert.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})
	t.Run("PublishAction with empty Type", func(t *testing.T) {
		actType := act.Type
		act.Type = ""
		defer func() { act.Type = actType }()
		stream, err := clientPub.PublishAction(context.Background(), act)
		assert.NotNil(t, stream)
		assert.NoError(t, err)
		rcv, err := stream.Recv()
		assert.Nil(t, rcv)
		assert.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})
	t.Run("PublishAction with empty Id", func(t *testing.T) {
		actId := act.Id
		act.Id = ""
		defer func() { act.Id = actId }()
		stream, err := clientPub.PublishAction(context.Background(), act)
		assert.NotNil(t, stream)
		assert.NoError(t, err)
		rcv, err := stream.Recv()
		assert.Nil(t, rcv)
		assert.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})
	t.Run("PublishAction with empty Source", func(t *testing.T) {
		actSrc := act.Source
		act.Source = ""
		defer func() { act.Source = actSrc }()
		stream, err := clientPub.PublishAction(context.Background(), act)
		assert.NotNil(t, stream)
		assert.NoError(t, err)
		rcv, err := stream.Recv()
		assert.Nil(t, rcv)
		assert.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})
	t.Run("PublishAction with canceled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		stream, err := clientPub.PublishAction(ctx, act)
		assert.NotNil(t, stream)
		assert.NoError(t, err)

		md, err := stream.Header() // await dda-suback
		assert.NoError(t, err)
		assert.Contains(t, md, metadata_dda_suback)

		cancel()
		rcv, err := stream.Recv()
		assert.Nil(t, rcv)
		assert.Error(t, err)
		assert.Equal(t, codes.Canceled, status.Code(err))
	})
	t.Run("SubscribeAction with invalid Filter", func(t *testing.T) {
		stream, err := clientPub.SubscribeAction(context.Background(), &stubs.SubscriptionFilter{Type: "foo/bar"})
		assert.NotNil(t, stream)
		assert.NoError(t, err)
		rcv, err := stream.Recv()
		assert.Nil(t, rcv)
		assert.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})
	t.Run("Publish-SubscribeAction", func(t *testing.T) {
		ctx1, cancel1 := context.WithCancel(context.Background())
		defer cancel1()
		stream1, err := clientSub1.SubscribeAction(ctx1, &stubs.SubscriptionFilter{Type: act.Type})
		assert.NotNil(t, stream1)
		assert.NoError(t, err)
		ctx2, cancel2 := context.WithCancel(context.Background())
		defer cancel2()
		stream2, err := clientSub2.SubscribeAction(ctx2, &stubs.SubscriptionFilter{Type: act.Type})
		assert.NotNil(t, stream2)
		assert.NoError(t, err)

		md1, err1 := stream1.Header() // await dda-suback
		assert.NoError(t, err1)
		assert.Contains(t, md1, metadata_dda_suback)
		md2, err2 := stream2.Header() // await dda-suback
		assert.NoError(t, err2)
		assert.Contains(t, md2, metadata_dda_suback)

		stream, err := clientPub.PublishAction(context.Background(), act)
		assert.NotNil(t, stream)
		assert.NoError(t, err)

		rcv1, err := stream1.Recv()
		assert.NoError(t, err)
		assert.Equal(t, act.Type, rcv1.GetAction().GetType())
		assert.Equal(t, act.Id, rcv1.GetAction().GetId())
		assert.Equal(t, act.Source, rcv1.GetAction().GetSource())
		assert.Equal(t, act.Params, rcv1.GetAction().GetParams())

		ack, err := clientSub1.PublishActionResult(ctx1, &stubs.ActionResultCorrelated{
			CorrelationId: rcv1.GetCorrelationId(),
			Result: &stubs.ActionResult{
				Context:        "sub1",
				SequenceNumber: 1,
				Data:           rcv1.GetAction().GetParams(),
			},
		})
		assert.NoError(t, err)
		assert.True(t, proto.Equal(&stubs.Ack{}, ack))

		ack, err = clientSub1.PublishActionResult(ctx1, &stubs.ActionResultCorrelated{
			CorrelationId: rcv1.GetCorrelationId(),
			Result: &stubs.ActionResult{
				Context:        "sub1",
				SequenceNumber: 2, // to test deleting actionCallback on gRPC server close
				Data:           rcv1.GetAction().GetParams(),
			},
		})
		assert.NoError(t, err)
		assert.True(t, proto.Equal(&stubs.Ack{}, ack))

		rcv2, err := stream2.Recv()
		assert.NoError(t, err)
		assert.Equal(t, act.Type, rcv2.GetAction().GetType())
		assert.Equal(t, act.Id, rcv2.GetAction().GetId())
		assert.Equal(t, act.Source, rcv2.GetAction().GetSource())
		assert.Equal(t, act.Params, rcv2.GetAction().GetParams())

		ack, err = clientSub2.PublishActionResult(ctx2, &stubs.ActionResultCorrelated{
			CorrelationId: rcv2.GetCorrelationId(),
			Result: &stubs.ActionResult{
				Context:        "sub2",
				SequenceNumber: 0,
				Data:           rcv2.GetAction().GetParams(),
			},
		})
		assert.True(t, proto.Equal(&stubs.Ack{}, ack))
		assert.NoError(t, err)

		ack, err = clientSub2.PublishActionResult(ctx2, &stubs.ActionResultCorrelated{
			CorrelationId: rcv2.GetCorrelationId(),
			Result: &stubs.ActionResult{
				Context:        "sub2",
				SequenceNumber: 1,
				Data:           rcv2.GetAction().GetParams(),
			},
		})
		assert.Nil(t, ack)
		assert.Error(t, err) // correlation id already cleaned up by previous call
		assert.Equal(t, codes.InvalidArgument, status.Code(err))

		rcv, err := stream.Recv()
		assert.NoError(t, err)
		assert.Equal(t, "sub1", rcv.GetContext())
		assert.Equal(t, int64(1), rcv.GetSequenceNumber())
		assert.Equal(t, act.Params, rcv.GetData())

		rcv, err = stream.Recv()
		assert.NoError(t, err)
		assert.Equal(t, "sub1", rcv.GetContext())
		assert.Equal(t, int64(2), rcv.GetSequenceNumber())
		assert.Equal(t, act.Params, rcv.GetData())

		rcv, err = stream.Recv()
		assert.NoError(t, err)
		assert.Equal(t, "sub2", rcv.GetContext())
		assert.Equal(t, int64(0), rcv.GetSequenceNumber())
		assert.Equal(t, act.Params, rcv.GetData())
	})

	qryData := []byte(`{
		"query": "PartsCount($category Category) { parts(category: $category) { count }}",
		"operationName": "PartsCount",
		"variables": { "category": "foo" }
	}`)
	qry := &stubs.Query{
		Type:            "partsCount",
		Id:              "42",
		Source:          "pub",
		Data:            qryData,
		DataContentType: "application/graphql",
	}

	t.Run("PublishQuery with invalid Type", func(t *testing.T) {
		qryType := qry.Type
		qry.Type = "+-*"
		defer func() { qry.Type = qryType }()
		stream, err := clientPub.PublishQuery(context.Background(), qry)
		assert.NotNil(t, stream)
		assert.NoError(t, err)
		rcv, err := stream.Recv()
		assert.Nil(t, rcv)
		assert.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})
	t.Run("PublishQuery with empty Type", func(t *testing.T) {
		qryType := qry.Type
		qry.Type = ""
		defer func() { qry.Type = qryType }()
		stream, err := clientPub.PublishQuery(context.Background(), qry)
		assert.NotNil(t, stream)
		assert.NoError(t, err)
		rcv, err := stream.Recv()
		assert.Nil(t, rcv)
		assert.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})
	t.Run("PublishQuery with empty Id", func(t *testing.T) {
		qryId := qry.Id
		qry.Id = ""
		defer func() { qry.Id = qryId }()
		stream, err := clientPub.PublishQuery(context.Background(), qry)
		assert.NotNil(t, stream)
		assert.NoError(t, err)
		rcv, err := stream.Recv()
		assert.Nil(t, rcv)
		assert.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})
	t.Run("PublishQuery with empty Source", func(t *testing.T) {
		qrySrc := qry.Source
		qry.Source = ""
		defer func() { qry.Source = qrySrc }()
		stream, err := clientPub.PublishQuery(context.Background(), qry)
		assert.NotNil(t, stream)
		assert.NoError(t, err)
		rcv, err := stream.Recv()
		assert.Nil(t, rcv)
		assert.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})
	t.Run("PublishQuery with canceled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		stream, err := clientPub.PublishQuery(ctx, qry)
		assert.NotNil(t, stream)
		assert.NoError(t, err)

		md, err := stream.Header() // await dda-suback
		assert.NoError(t, err)
		assert.Contains(t, md, metadata_dda_suback)

		cancel()
		rcv, err := stream.Recv()
		assert.Nil(t, rcv)
		assert.Error(t, err)
		assert.Equal(t, codes.Canceled, status.Code(err))
	})
	t.Run("SubscribeQuery with invalid Filter", func(t *testing.T) {
		stream, err := clientPub.SubscribeQuery(context.Background(), &stubs.SubscriptionFilter{Type: "foo/bar"})
		assert.NotNil(t, stream)
		assert.NoError(t, err)
		rcv, err := stream.Recv()
		assert.Nil(t, rcv)
		assert.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})
	t.Run("Publish-SubscribeQuery", func(t *testing.T) {
		ctx1, cancel1 := context.WithCancel(context.Background())
		defer cancel1()
		stream1, err := clientSub1.SubscribeQuery(ctx1, &stubs.SubscriptionFilter{Type: qry.Type})
		assert.NotNil(t, stream1)
		assert.NoError(t, err)
		ctx2, cancel2 := context.WithCancel(context.Background())
		defer cancel2()
		stream2, err := clientSub2.SubscribeQuery(ctx2, &stubs.SubscriptionFilter{Type: qry.Type})
		assert.NotNil(t, stream2)
		assert.NoError(t, err)

		md1, err1 := stream1.Header() // await dda-suback
		assert.NoError(t, err1)
		assert.Contains(t, md1, metadata_dda_suback)
		md2, err2 := stream2.Header() // await dda-suback
		assert.NoError(t, err2)
		assert.Contains(t, md2, metadata_dda_suback)

		stream, err := clientPub.PublishQuery(context.Background(), qry)
		assert.NotNil(t, stream)
		assert.NoError(t, err)

		rcv1, err := stream1.Recv()
		assert.NoError(t, err)
		assert.Equal(t, qry.Type, rcv1.GetQuery().GetType())
		assert.Equal(t, qry.Id, rcv1.GetQuery().GetId())
		assert.Equal(t, qry.Source, rcv1.GetQuery().GetSource())
		assert.Equal(t, qry.DataContentType, rcv1.GetQuery().GetDataContentType())
		assert.Equal(t, qry.Data, rcv1.GetQuery().GetData())

		qryResData := []byte(`{ "data": { "parts": { "count" : 42 }}}`)
		qryResType := "application/graphql"
		ack, err := clientSub1.PublishQueryResult(ctx1, &stubs.QueryResultCorrelated{
			CorrelationId: rcv1.GetCorrelationId(),
			Result: &stubs.QueryResult{
				Context:         "sub1",
				SequenceNumber:  1,
				Data:            qryResData,
				DataContentType: qryResType,
			},
		})
		assert.NoError(t, err)
		assert.True(t, proto.Equal(&stubs.Ack{}, ack))

		ack, err = clientSub1.PublishQueryResult(ctx1, &stubs.QueryResultCorrelated{
			CorrelationId: rcv1.GetCorrelationId(),
			Result: &stubs.QueryResult{
				Context:         "sub1",
				SequenceNumber:  2, // to test deleting actionCallback on gRPC server close
				Data:            qryResData,
				DataContentType: qryResType,
			},
		})
		assert.NoError(t, err)
		assert.True(t, proto.Equal(&stubs.Ack{}, ack))

		rcv2, err := stream2.Recv()
		assert.NoError(t, err)
		assert.Equal(t, qry.Type, rcv2.GetQuery().GetType())
		assert.Equal(t, qry.Id, rcv2.GetQuery().GetId())
		assert.Equal(t, qry.Source, rcv2.GetQuery().GetSource())
		assert.Equal(t, qry.Data, rcv2.GetQuery().GetData())
		assert.Equal(t, qry.DataContentType, rcv2.GetQuery().GetDataContentType())

		ack, err = clientSub2.PublishQueryResult(ctx2, &stubs.QueryResultCorrelated{
			CorrelationId: rcv2.GetCorrelationId(),
			Result: &stubs.QueryResult{
				Context:         "sub2",
				SequenceNumber:  0,
				Data:            qryResData,
				DataContentType: qryResType,
			},
		})
		assert.True(t, proto.Equal(&stubs.Ack{}, ack))
		assert.NoError(t, err)

		ack, err = clientSub2.PublishQueryResult(ctx2, &stubs.QueryResultCorrelated{
			CorrelationId: rcv2.GetCorrelationId(),
			Result: &stubs.QueryResult{
				Context:         "sub2",
				SequenceNumber:  1,
				Data:            qryResData,
				DataContentType: qryResType,
			},
		})
		assert.Nil(t, ack)
		assert.Error(t, err) // correlation id already cleaned up by previous call
		assert.Equal(t, codes.InvalidArgument, status.Code(err))

		rcv, err := stream.Recv()
		assert.NoError(t, err)
		assert.Equal(t, "sub1", rcv.GetContext())
		assert.Equal(t, int64(1), rcv.GetSequenceNumber())
		assert.Equal(t, qryResData, rcv.GetData())
		assert.Equal(t, qryResType, rcv.GetDataContentType())

		rcv, err = stream.Recv()
		assert.NoError(t, err)
		assert.Equal(t, "sub1", rcv.GetContext())
		assert.Equal(t, int64(2), rcv.GetSequenceNumber())
		assert.Equal(t, qryResData, rcv.GetData())
		assert.Equal(t, qryResType, rcv.GetDataContentType())

		rcv, err = stream.Recv()
		assert.NoError(t, err)
		assert.Equal(t, "sub2", rcv.GetContext())
		assert.Equal(t, int64(0), rcv.GetSequenceNumber())
		assert.Equal(t, qryResData, rcv.GetData())
		assert.Equal(t, qryResType, rcv.GetDataContentType())
	})
}

// Runs all benchmarks on a given gRPC Client API for the given communication
// service.
func RunBenchGrpc(b *testing.B, cluster string, clientApis config.ConfigApis, comSrv config.ConfigComService) {
	// This benchmark with setup will not be measured itself and called once for
	// each combination of client API and communication service with b.N=1

	cfgPub := testdata.NewConfig(cluster, "ddaPub", comSrv)
	cfgPub.Apis = clientApis
	ddaPub, err := testdata.OpenDdaWithConfig(cfgPub)
	if err != nil {
		b.FailNow()
	}

	cfgSub := testdata.NewConfig(cluster, "ddaSub", comSrv)
	cfgSub.Apis = clientApis
	cfgSub.Apis.Grpc.Address = nextAddress(cfgSub.Apis.Grpc.Address)
	ddaSub, err := testdata.OpenDdaWithConfig(cfgSub)
	if err != nil {
		b.FailNow()
	}

	clientPub, closeClientPub, err := openGrpcClient(cfgPub.Apis.Grpc.Address, cfgPub.Apis.Cert)
	if err != nil {
		b.FailNow()
	}

	clientSub, closeClientSub, err := openGrpcClient(cfgSub.Apis.Grpc.Address, cfgSub.Apis.Cert)
	if err != nil {
		b.FailNow()
	}

	defer testdata.CloseDda(ddaPub)
	defer testdata.CloseDda(ddaSub)

	defer closeClientPub()
	defer closeClientSub()

	evt := &stubs.Event{
		Type:   "hello",
		Id:     "42",
		Source: "pub",
		Data:   []byte(`"` + "Hello from pub " + ddaPub.Identity().Id + `"`),
	}

	ctxEvt, cancelEvt := context.WithCancel(context.Background())
	defer cancelEvt()
	streamEvt, err := clientSub.SubscribeEvent(ctxEvt, &stubs.SubscriptionFilter{Type: evt.Type})
	if err != nil {
		b.FailNow()
	}

	streamEvt.Header() // await dda-suback

	// This subbenchmark will be measured and invoked multiple times with
	// growing b.N
	b.Run("Pub-Sub One-way", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			if _, err := clientPub.PublishEvent(context.Background(), evt); err != nil {
				b.FailNow()
			}
			if _, err := streamEvt.Recv(); err != nil {
				b.FailNow()
			}
		}
	})

	act := &stubs.Action{
		Type:   "echo",
		Id:     "42",
		Source: "pub",
		Params: []byte(`[1, 2, 3, 4, 5]`),
	}

	ctxAct, cancelAct := context.WithCancel(context.Background())
	defer cancelAct()
	streamAct, err := clientSub.SubscribeAction(ctxAct, &stubs.SubscriptionFilter{Type: act.Type})
	if err != nil {
		b.FailNow()
	}

	streamAct.Header() // await dda-suback

	go func() {
		for {
			if rcv, err := streamAct.Recv(); err == nil {
				if _, err := clientSub.PublishActionResult(ctxAct, &stubs.ActionResultCorrelated{
					CorrelationId: rcv.GetCorrelationId(),
					Result: &stubs.ActionResult{
						Context:        "sub",
						SequenceNumber: 0,
						Data:           rcv.GetAction().GetParams(),
					},
				}); err != nil {
					break
				}
			} else {
				break
			}
		}
	}()

	// This subbenchmark will be measured and invoked multiple times with
	// growing b.N
	b.Run("Pub-Sub Two-way", func(b *testing.B) {
		for n := 0; n < b.N; n++ {
			act.Id = fmt.Sprintf("%d", n)
			ctx, cancel := context.WithCancel(context.Background())
			stream, err := clientPub.PublishAction(ctx, act)
			if err != nil {
				b.FailNow()
			}
			_, err = stream.Recv()
			if err != nil {
				b.FailNow()
			}

			// To measure a complete cycle including unsubscription of the
			// response topic, we must cancel the server streaming call so that
			// the next invocation of PublishAction sets up a new response
			// subscription.
			cancel()
		}
	})
}
