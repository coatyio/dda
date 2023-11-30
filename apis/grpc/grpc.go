// SPDX-FileCopyrightText: © 2023 Siemens AG
// SPDX-License-Identifier: MIT

// Package grpc provides a gRPC server that exposes the DDA peripheral services
// to gRPC and gRPC-Web clients.
package grpc

import (
	"context"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/coatyio/dda/apis"
	"github.com/coatyio/dda/apis/grpc/stubs/golang/com"
	"github.com/coatyio/dda/apis/grpc/stubs/golang/store"
	"github.com/coatyio/dda/config"
	"github.com/coatyio/dda/plog"
	"github.com/coatyio/dda/services"
	comapi "github.com/coatyio/dda/services/com/api"
	storeapi "github.com/coatyio/dda/services/store/api"
	"github.com/google/uuid"
	rpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

var metadata_dda_suback = metadata.Pairs("dda-suback", "OK")

// grpcServer realizes a gRPC server that exposes the peripheral DDA services to
// gRPC and gRPC-Web clients. grpcServer implements the apis.ApiServer interface
// and all the ServiceServer interfaces of the generated gRPC stubs.
type grpcServer struct {
	com.UnimplementedComServiceServer
	comApi comapi.Api
	store.UnimplementedStoreServiceServer
	storeApi storeapi.Api
	mu       sync.RWMutex // protects following fields
	srv      *rpc.Server
	grpcWebServer
	actionCallbacks map[string]comapi.ActionCallback
	queryCallbacks  map[string]comapi.QueryCallback
}

// New returns an apis.ApiServer interface that implements an uninitialized gRPC
// server exposing the peripheral DDA services to gRPC and gRPC-Web clients. To
// start the returned server, invoke Open with a gRPC-enabled DDA configuration.
func New(com comapi.Api, store storeapi.Api) apis.ApiServer {
	return &grpcServer{
		comApi:          com,
		storeApi:        store,
		actionCallbacks: make(map[string]comapi.ActionCallback),
		queryCallbacks:  make(map[string]comapi.QueryCallback),
	}
}

// Open creates a ready-to-run gRPC server with the configured server
// credentials that accepts client requests over gRPC/gRPC-Web on the configured
// addresses. In case of gRPC connections supported protocols include TCP and
// Unix domain sockets.
func (s *grpcServer) Open(cfg *config.Config) error {
	address := cfg.Apis.Grpc.Address

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.srv == nil {

		// Set up gRPC server.

		srvOpts := []rpc.ServerOption{}
		if cfg.Apis.Cert != "" && cfg.Apis.Key != "" {
			creds, err := credentials.NewServerTLSFromFile(cfg.Apis.Cert, cfg.Apis.Key)
			if err != nil {
				return err
			}
			srvOpts = append(srvOpts, rpc.Creds(creds))
		}

		// Use configurable values for Keepalive Server Parameters.
		//
		// https://pkg.go.dev/google.golang.org/grpc/keepalive
		// https://grpc.io/docs/guides/keepalive/
		// https://github.com/grpc/grpc-go/blob/v1.53.0/Documentation/keepalive.md
		srvOpts = append(srvOpts, rpc.KeepaliveParams(keepalive.ServerParameters{
			Time: cfg.Apis.Grpc.Keepalive,
		}))

		s.srv = rpc.NewServer(srvOpts...)
		com.RegisterComServiceServer(s.srv, s)
		store.RegisterStoreServiceServer(s.srv, s)

		plog.Printf("Open gRPC server listening on address %s...\n", address)

		protocol := "tcp"
		if sockfile, isUnix := strings.CutPrefix(address, "unix:"); isUnix {
			protocol = "unix"
			address = sockfile

			// Remove existing socket file to clean up in crash situations where
			// the listener is not being closed.
			if err := os.Remove(sockfile); err != nil && !os.IsNotExist(err) {
				return err
			}
		}

		lis, err := net.Listen(protocol, address)
		if err != nil {
			return err
		}

		go s.srv.Serve(lis)

		return s.openWebServer(s.srv, cfg)
	}

	return nil
}

// Close stops the gRPC server, canceling all active RPCs on the server side,
// and closing all open connections. Pending RPCs on the client side will get
// notified by connection errors.
func (s *grpcServer) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.srv != nil {
		s.srv.Stop()
		s.srv = nil
		s.closeWebServer()
		for k := range s.actionCallbacks {
			delete(s.actionCallbacks, k)
		}
		for k := range s.queryCallbacks {
			delete(s.queryCallbacks, k)
		}
		plog.Println("Closed gRPC server")
	}
}

// Com Api

func (s *grpcServer) PublishEvent(ctx context.Context, event *com.Event) (*com.Ack, error) {
	if s.comApi == nil {
		return nil, s.serviceDisabledError("com")
	}
	if err := s.comApi.PublishEvent(comapi.Event{
		Type:            event.GetType(),
		Id:              event.GetId(),
		Source:          event.GetSource(),
		Time:            event.GetTime(),
		Data:            event.GetData(),
		DataContentType: event.GetDataContentType(),
	}); err != nil {
		err = status.Errorf(s.codeByError(err), "failed publishing Event: %v", err)
		plog.Println(err)
		return nil, err
	}

	return &com.Ack{}, nil
}

func (s *grpcServer) SubscribeEvent(filter *com.SubscriptionFilter, stream com.ComService_SubscribeEventServer) error {
	if s.comApi == nil {
		return s.serviceDisabledError("com")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	evts, err := s.comApi.SubscribeEvent(ctx, comapi.SubscriptionFilter{
		Type:  filter.GetType(),
		Share: filter.GetShare(),
	})
	if err != nil {
		err = status.Errorf(s.codeByError(err), "failed subscribing Event: %v", err)
		plog.Println(err)
		return err
	}

	// Send header with custom metadata to signal acknowledgment of the DDA
	// subscription. The gRPC client should await this acknowledgment to prevent
	// race conditions with subsequent related publications that may be
	// delivered and responded before this subscription is being fully
	// established by the pub-sub server.
	if err := stream.SendHeader(metadata_dda_suback); err != nil {
		plog.Println(err)
		return err
	}

	for {
		select {
		case evt, ok := <-evts:
			if !ok {
				// End stream if channel has been closed by communication service.
				return nil
			}
			if err := stream.Send(&com.Event{
				Type:            evt.Type,
				Id:              evt.Id,
				Source:          evt.Source,
				Time:            evt.Time,
				Data:            evt.Data,
				DataContentType: evt.DataContentType,
			}); err != nil {
				// Do not return err, but keep stream alive for further transmissions.
				plog.Println(err)
			}
		case <-stream.Context().Done(): // server streaming call canceled by client
			return stream.Context().Err()
		}
	}
}

func (s *grpcServer) PublishAction(action *com.Action, stream com.ComService_PublishActionServer) error {
	if s.comApi == nil {
		return s.serviceDisabledError("com")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	results, err := s.comApi.PublishAction(ctx, comapi.Action{
		Type:            action.GetType(),
		Id:              action.GetId(),
		Source:          action.GetSource(),
		Params:          action.GetParams(),
		DataContentType: action.GetDataContentType(),
	})
	if err != nil {
		err = status.Errorf(s.codeByError(err), "failed publishing Action: %v", err)
		plog.Println(err)
		return err
	}

	// Send header with custom metadata to signal acknowledgment of the DDA
	// response subscription and the publication. The gRPC client may await this
	// acknowledgment to prevent race conditions with subsequent related
	// publications that may be delivered and responded on the response
	// subscription before it is being fully established by the pub-sub server.
	if err := stream.SendHeader(metadata_dda_suback); err != nil {
		plog.Println(err)
		return err
	}

	for {
		select {
		case res, ok := <-results:
			if !ok {
				// End stream if channel has been closed by communication service.
				return nil
			}
			if err := stream.Send(&com.ActionResult{
				Context:         res.Context,
				Data:            res.Data,
				DataContentType: res.DataContentType,
				SequenceNumber:  res.SequenceNumber,
			}); err != nil {
				// Do not return err, but keep stream alive for further transmissions.
				plog.Println(err)
			}
		case <-stream.Context().Done(): // server streaming call canceled by client
			return stream.Context().Err()
		}
	}
}

func (s *grpcServer) SubscribeAction(filter *com.SubscriptionFilter, stream com.ComService_SubscribeActionServer) error {
	if s.comApi == nil {
		return s.serviceDisabledError("com")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	acts, err := s.comApi.SubscribeAction(ctx, comapi.SubscriptionFilter{
		Type:  filter.GetType(),
		Share: filter.GetShare(),
	})
	if err != nil {
		err = status.Errorf(s.codeByError(err), "failed subscribing Action: %v", err)
		plog.Println(err)
		return err
	}

	// Send header with custom metadata to signal acknowledgment of the DDA
	// subscription. The gRPC client should await this acknowledgment to prevent
	// race conditions with subsequent related publications that may be
	// delivered and responded before this subscription is being fully
	// established by the pub-sub server.
	if err := stream.SendHeader(metadata_dda_suback); err != nil {
		plog.Println(err)
		return err
	}

	for {
		select {
		case act, ok := <-acts:
			if !ok {
				// End stream if channel has been closed by communication service.
				return nil
			}
			cid := uuid.NewString()
			s.mu.Lock()
			s.actionCallbacks[cid] = act.Callback
			s.mu.Unlock()
			if err := stream.Send(&com.ActionCorrelated{
				Action: &com.Action{
					Type:            act.Type,
					Id:              act.Id,
					Source:          act.Source,
					Params:          act.Params,
					DataContentType: act.DataContentType,
				},
				CorrelationId: cid,
			}); err != nil {
				// Do not return err, but keep stream alive for further transmissions.
				plog.Println(err)
			}
		case <-stream.Context().Done(): // Server streaming call canceled by client
			return stream.Context().Err()
		}
	}
}

func (s *grpcServer) PublishActionResult(ctx context.Context, result *com.ActionResultCorrelated) (*com.Ack, error) {
	if s.comApi == nil {
		return nil, s.serviceDisabledError("com")
	}

	s.mu.RLock()
	cb, ok := s.actionCallbacks[result.GetCorrelationId()]
	s.mu.RUnlock()
	if !ok {
		err := status.Errorf(codes.InvalidArgument, "failed publishing ActionResult: unknown correlation id")
		plog.Println(err)
		return nil, err
	}

	res := result.GetResult()
	if res.GetSequenceNumber() <= 0 {
		s.mu.Lock()
		delete(s.actionCallbacks, result.GetCorrelationId()) // no more results will follow
		s.mu.Unlock()
	}

	if err := cb(comapi.ActionResult{
		Context:         res.GetContext(),
		Data:            res.GetData(),
		DataContentType: res.GetDataContentType(),
		SequenceNumber:  res.GetSequenceNumber(),
	}); err != nil {
		err = status.Errorf(s.codeByError(err), "failed publishing ActionResult: %v", err)
		plog.Println(err)
		return nil, err
	}

	return &com.Ack{}, nil
}

func (s *grpcServer) PublishQuery(query *com.Query, stream com.ComService_PublishQueryServer) error {
	if s.comApi == nil {
		return s.serviceDisabledError("com")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	results, err := s.comApi.PublishQuery(ctx, comapi.Query{
		Type:            query.GetType(),
		Id:              query.GetId(),
		Source:          query.GetSource(),
		Data:            query.GetData(),
		DataContentType: query.GetDataContentType(),
	})
	if err != nil {
		err = status.Errorf(s.codeByError(err), "failed publishing Query: %v", err)
		plog.Println(err)
		return err
	}

	// Send header with custom metadata to signal acknowledgment of the DDA
	// response subscription and the publication. The gRPC client may await this
	// acknowledgment to prevent race conditions with subsequent related
	// publications that may be delivered and responded on the response
	// subscription before it is being fully established by the pub-sub server.
	if err := stream.SendHeader(metadata_dda_suback); err != nil {
		plog.Println(err)
		return err
	}

	for {
		select {
		case res, ok := <-results:
			if !ok {
				// End stream if channel has been closed by communication service.
				return nil
			}
			if err := stream.Send(&com.QueryResult{
				Context:         res.Context,
				Data:            res.Data,
				DataContentType: res.DataContentType,
				SequenceNumber:  res.SequenceNumber,
			}); err != nil {
				// Do not return err, but keep stream alive for further transmissions.
				plog.Println(err)
			}
		case <-stream.Context().Done(): // server streaming call canceled by client
			return stream.Context().Err()
		}
	}
}

func (s *grpcServer) SubscribeQuery(filter *com.SubscriptionFilter, stream com.ComService_SubscribeQueryServer) error {
	if s.comApi == nil {
		return s.serviceDisabledError("com")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	qrys, err := s.comApi.SubscribeQuery(ctx, comapi.SubscriptionFilter{
		Type:  filter.GetType(),
		Share: filter.GetShare(),
	})
	if err != nil {
		err = status.Errorf(s.codeByError(err), "failed subscribing Query: %v", err)
		plog.Println(err)
		return err
	}

	// Send header with custom metadata to signal acknowledgment of the DDA
	// subscription. The gRPC client should await this acknowledgment to prevent
	// race conditions with subsequent code that indirectly triggers a remote
	// publication on the subscription which can be lost if the subscription has
	// not yet been fully established by the pub-sub infrastructure.
	if err := stream.SendHeader(metadata_dda_suback); err != nil {
		plog.Println(err)
		return err
	}

	for {
		select {
		case qry, ok := <-qrys:
			if !ok {
				// End stream if channel has been closed by communication service.
				return nil
			}
			cid := uuid.NewString()
			s.mu.Lock()
			s.queryCallbacks[cid] = qry.Callback
			s.mu.Unlock()
			if err := stream.Send(&com.QueryCorrelated{
				Query: &com.Query{
					Type:            qry.Type,
					Id:              qry.Id,
					Source:          qry.Source,
					Data:            qry.Data,
					DataContentType: qry.DataContentType,
				},
				CorrelationId: cid,
			}); err != nil {
				// Do not return err, but keep stream alive for further transmissions.
				plog.Println(err)
			}
		case <-stream.Context().Done(): // server streaming call canceled by client
			return stream.Context().Err()
		}
	}
}

func (s *grpcServer) PublishQueryResult(ctx context.Context, result *com.QueryResultCorrelated) (*com.Ack, error) {
	if s.comApi == nil {
		return nil, s.serviceDisabledError("com")
	}

	s.mu.RLock()
	cb, ok := s.queryCallbacks[result.GetCorrelationId()]
	s.mu.RUnlock()
	if !ok {
		err := status.Errorf(codes.InvalidArgument, "failed publishing QueryResult: unknown correlation id")
		plog.Println(err)
		return nil, err
	}

	res := result.GetResult()
	if res.GetSequenceNumber() <= 0 {
		s.mu.Lock()
		delete(s.queryCallbacks, result.GetCorrelationId()) // no more results will follow
		s.mu.Unlock()
	}

	if err := cb(comapi.QueryResult{
		Context:         res.GetContext(),
		Data:            res.GetData(),
		DataContentType: res.GetDataContentType(),
		SequenceNumber:  res.GetSequenceNumber(),
	}); err != nil {
		err = status.Errorf(s.codeByError(err), "failed publishing QueryResult: %v", err)
		plog.Println(err)
		return nil, err
	}

	return &com.Ack{}, nil
}

// Store API

func (s *grpcServer) Get(ctx context.Context, key *store.Key) (*store.Value, error) {
	if s.storeApi == nil {
		return nil, s.serviceDisabledError("store")
	}
	val, err := s.storeApi.Get(key.GetKey())
	if err != nil {
		err = status.Errorf(s.codeByError(err), "failed: %v", err)
		plog.Println(err)
		return nil, err
	}
	if val == nil {
		return &store.Value{}, nil // non-existing store key not explicit present
	}
	return &store.Value{Value: val}, nil
}

func (s *grpcServer) Set(ctx context.Context, kv *store.KeyValue) (*store.Ack, error) {
	if s.storeApi == nil {
		return nil, s.serviceDisabledError("store")
	}
	err := s.storeApi.Set(kv.GetKey(), kv.GetValue())
	if err != nil {
		err = status.Errorf(s.codeByError(err), "failed: %v", err)
		plog.Println(err)
		return nil, err
	}
	return &store.Ack{}, nil
}

func (s *grpcServer) Delete(ctx context.Context, key *store.Key) (*store.Ack, error) {
	if s.storeApi == nil {
		return nil, s.serviceDisabledError("store")
	}
	err := s.storeApi.Delete(key.GetKey())
	if err != nil {
		err = status.Errorf(s.codeByError(err), "failed: %v", err)
		plog.Println(err)
		return nil, err
	}
	return &store.Ack{}, nil
}

func (s *grpcServer) DeleteAll(ctx context.Context, p *store.DeleteAllParams) (*store.Ack, error) {
	if s.storeApi == nil {
		return nil, s.serviceDisabledError("store")
	}
	err := s.storeApi.DeleteAll()
	if err != nil {
		err = status.Errorf(s.codeByError(err), "failed: %v", err)
		plog.Println(err)
		return nil, err
	}
	return &store.Ack{}, nil
}

func (s *grpcServer) DeleteRange(ctx context.Context, r *store.Range) (*store.Ack, error) {
	if s.storeApi == nil {
		return nil, s.serviceDisabledError("store")
	}
	err := s.storeApi.DeleteRange(r.GetStart(), r.GetEnd())
	if err != nil {
		err = status.Errorf(s.codeByError(err), "failed: %v", err)
		plog.Println(err)
		return nil, err
	}
	return &store.Ack{}, nil
}

func (s *grpcServer) ScanPrefix(key *store.Key, stream store.StoreService_ScanPrefixServer) error {
	if s.storeApi == nil {
		return s.serviceDisabledError("store")
	}
	err := s.storeApi.ScanPrefix(key.GetKey(), func(key string, value []byte) error {
		select {
		case <-stream.Context().Done(): // stop scanning if server streaming call canceled by client
			return stream.Context().Err()
		default:
		}
		err := stream.Send(&store.KeyValue{
			Key:   key,
			Value: value,
		})
		if err != nil {
			plog.Println(err) // stop scanning on first failure
		}
		return err
	})
	if err != nil {
		err = status.Errorf(s.codeByError(err), "failed: %v", err)
		plog.Println(err)
	}
	return err
}

func (s *grpcServer) ScanRange(r *store.Range, stream store.StoreService_ScanRangeServer) error {
	if s.storeApi == nil {
		return s.serviceDisabledError("store")
	}
	err := s.storeApi.ScanRange(r.GetStart(), r.GetEnd(), func(key string, value []byte) error {
		select {
		case <-stream.Context().Done(): // stop scanning if server streaming call canceled by client
			return stream.Context().Err()
		default:
		}
		err := stream.Send(&store.KeyValue{
			Key:   key,
			Value: value,
		})
		if err != nil {
			plog.Println(err) // stop scanning on first failure
		}
		return err
	})
	if err != nil {
		err = status.Errorf(s.codeByError(err), "failed: %v", err)
		plog.Println(err)
	}
	return err
}

// Utils

func (s *grpcServer) serviceDisabledError(srv string) error {
	err := services.RetryableErrorf("service %s is disabled", srv)
	err = status.Errorf(s.codeByError(err), "failed: %v", err)
	plog.Println(err)
	return err
}

func (s *grpcServer) codeByError(err error) codes.Code {
	if services.IsRetryable(err) {
		return codes.Unavailable
	} else {
		return codes.InvalidArgument
	}
}
