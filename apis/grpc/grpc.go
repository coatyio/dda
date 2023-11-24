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
	stubs "github.com/coatyio/dda/apis/grpc/stubs/golang"
	"github.com/coatyio/dda/config"
	"github.com/coatyio/dda/plog"
	"github.com/coatyio/dda/services"
	comapi "github.com/coatyio/dda/services/com/api"
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
	stubs.UnimplementedComServiceServer
	comApi comapi.Api
	mu     sync.RWMutex // protects following fields
	srv    *rpc.Server
	grpcWebServer
	actionCallbacks map[string]comapi.ActionCallback
	queryCallbacks  map[string]comapi.QueryCallback
}

// New returns an apis.ApiServer interface that implements an uninitialized gRPC
// server exposing the peripheral DDA services to gRPC and gRPC-Web clients. To
// start the returned server, invoke Open with a gRPC-enabled DDA configuration.
func New(com comapi.Api) apis.ApiServer {
	return &grpcServer{
		comApi:          com,
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
		stubs.RegisterComServiceServer(s.srv, s)

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

func (s *grpcServer) PublishEvent(ctx context.Context, event *stubs.Event) (*stubs.Ack, error) {
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

	return &stubs.Ack{}, nil
}

func (s *grpcServer) SubscribeEvent(filter *stubs.SubscriptionFilter, stream stubs.ComService_SubscribeEventServer) error {
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
			if err := stream.Send(&stubs.Event{
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

func (s *grpcServer) PublishAction(action *stubs.Action, stream stubs.ComService_PublishActionServer) error {
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
			if err := stream.Send(&stubs.ActionResult{
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

func (s *grpcServer) SubscribeAction(filter *stubs.SubscriptionFilter, stream stubs.ComService_SubscribeActionServer) error {
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
			if err := stream.Send(&stubs.ActionCorrelated{
				Action: &stubs.Action{
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

func (s *grpcServer) PublishActionResult(ctx context.Context, result *stubs.ActionResultCorrelated) (*stubs.Ack, error) {
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

	return &stubs.Ack{}, nil
}

func (s *grpcServer) PublishQuery(query *stubs.Query, stream stubs.ComService_PublishQueryServer) error {
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
			if err := stream.Send(&stubs.QueryResult{
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

func (s *grpcServer) SubscribeQuery(filter *stubs.SubscriptionFilter, stream stubs.ComService_SubscribeQueryServer) error {
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
			if err := stream.Send(&stubs.QueryCorrelated{
				Query: &stubs.Query{
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

func (s *grpcServer) PublishQueryResult(ctx context.Context, result *stubs.QueryResultCorrelated) (*stubs.Ack, error) {
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

	return &stubs.Ack{}, nil
}

func (s *grpcServer) codeByError(err error) codes.Code {
	if services.IsRetryable(err) {
		return codes.Unavailable
	} else {
		return codes.InvalidArgument
	}
}
