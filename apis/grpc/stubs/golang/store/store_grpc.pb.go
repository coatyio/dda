// SPDX-FileCopyrightText: © 2023 Siemens AG
// SPDX-License-Identifier: MIT

// Service definition of DDA local key-value storage API.

// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             v4.23.3
// source: store.proto

package store

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

const (
	StoreService_Get_FullMethodName          = "/dda.store.v1.StoreService/Get"
	StoreService_Set_FullMethodName          = "/dda.store.v1.StoreService/Set"
	StoreService_Delete_FullMethodName       = "/dda.store.v1.StoreService/Delete"
	StoreService_DeleteAll_FullMethodName    = "/dda.store.v1.StoreService/DeleteAll"
	StoreService_DeletePrefix_FullMethodName = "/dda.store.v1.StoreService/DeletePrefix"
	StoreService_DeleteRange_FullMethodName  = "/dda.store.v1.StoreService/DeleteRange"
	StoreService_ScanPrefix_FullMethodName   = "/dda.store.v1.StoreService/ScanPrefix"
	StoreService_ScanRange_FullMethodName    = "/dda.store.v1.StoreService/ScanRange"
)

// StoreServiceClient is the client API for StoreService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type StoreServiceClient interface {
	// Get returns the Value of the given Key. If the Key's key field is not
	// present, the empty key is used for lookup. If the store does not contain
	// the given key, it returns a Value with a value field that is NOT explicitly
	// present.
	//
	// In application code, always use language-specific generated "hasKey"
	// methods to check for key existence in the store, instead of comparing
	// default values.
	//
	// If the operation fails, a gRPC error with status code UNAVAILABLE (14) is
	// signaled.
	Get(ctx context.Context, in *Key, opts ...grpc.CallOption) (*Value, error)
	// Set sets the value for the given KeyValue pair. It overwrites any previous
	// value for that key.
	//
	// If a value is not present, a gRPC error with status code INVALID_ARGUMENT
	// (3) is signaled. Otherwise, if the operation fails, a gRPC error with
	// status code UNAVAILABLE (14) is signaled.
	Set(ctx context.Context, in *KeyValue, opts ...grpc.CallOption) (*Ack, error)
	// Delete deletes the value for the given Key.
	//
	// If the operation fails, a gRPC error with status code UNAVAILABLE (14) is
	// signaled.
	Delete(ctx context.Context, in *Key, opts ...grpc.CallOption) (*Ack, error)
	// DeleteAll deletes all key-value pairs in the store.
	//
	// If the operation fails, a gRPC error with status code UNAVAILABLE (14) is
	// signaled.
	DeleteAll(ctx context.Context, in *DeleteAllParams, opts ...grpc.CallOption) (*Ack, error)
	// DeletePrefix deletes all of the keys (and values) that start with the given
	// prefix. Key strings are ordered lexicographically by their underlying byte
	// representation, i.e. UTF-8 encoding.
	//
	// If the operation fails, a gRPC error with status code UNAVAILABLE (14) is
	// signaled.
	DeletePrefix(ctx context.Context, in *Key, opts ...grpc.CallOption) (*Ack, error)
	// DeleteRange deletes all of the keys (and values) in the right-open Range
	// [start,end) (inclusive on start, exclusive on end). Key strings are ordered
	// lexicographically by their underlying byte representation, i.e. UTF-8
	// encoding.
	//
	// If the operation fails, a gRPC error with status code UNAVAILABLE (14) is
	// signaled.
	DeleteRange(ctx context.Context, in *Range, opts ...grpc.CallOption) (*Ack, error)
	// ScanPrefix iterates over key-value pairs whose keys start with the given
	// prefix Key in key order. Key strings are ordered lexicographically by their
	// underlying byte representation, i.e. UTF-8 encoding.
	//
	// If the operation fails, a gRPC error with status code UNAVAILABLE (14) is
	// signaled.
	//
	// It is not safe to invoke Set, Delete, DeleteAll, DeletePrefix, and
	// DeleteRange operations while receiving data from the stream as such calls
	// may block until the stream is closed. Instead, accumulate key-value pairs
	// and issue such operations after scanning is finished.
	ScanPrefix(ctx context.Context, in *Key, opts ...grpc.CallOption) (StoreService_ScanPrefixClient, error)
	// ScanRange iterates over a given right-open Range of key-value pairs in key
	// order (inclusive on start, exclusive on end). Key strings are ordered
	// lexicographically by their underlying byte representation, i.e. UTF-8
	// encoding.
	//
	// For example, this function can be used to iterate over keys which represent
	// a time range with a sortable time encoding like RFC3339.
	//
	// If the operation fails, a gRPC error with status code UNAVAILABLE (14) is
	// signaled.
	//
	// It is not safe to invoke Set, Delete, DeleteAll, DeletePrefix, and
	// DeleteRange operations while receiving data from the stream as such calls
	// may block until the stream is closed. Instead, accumulate key-value pairs
	// and issue such operations after scanning is finished.
	ScanRange(ctx context.Context, in *Range, opts ...grpc.CallOption) (StoreService_ScanRangeClient, error)
}

type storeServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewStoreServiceClient(cc grpc.ClientConnInterface) StoreServiceClient {
	return &storeServiceClient{cc}
}

func (c *storeServiceClient) Get(ctx context.Context, in *Key, opts ...grpc.CallOption) (*Value, error) {
	out := new(Value)
	err := c.cc.Invoke(ctx, StoreService_Get_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *storeServiceClient) Set(ctx context.Context, in *KeyValue, opts ...grpc.CallOption) (*Ack, error) {
	out := new(Ack)
	err := c.cc.Invoke(ctx, StoreService_Set_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *storeServiceClient) Delete(ctx context.Context, in *Key, opts ...grpc.CallOption) (*Ack, error) {
	out := new(Ack)
	err := c.cc.Invoke(ctx, StoreService_Delete_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *storeServiceClient) DeleteAll(ctx context.Context, in *DeleteAllParams, opts ...grpc.CallOption) (*Ack, error) {
	out := new(Ack)
	err := c.cc.Invoke(ctx, StoreService_DeleteAll_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *storeServiceClient) DeletePrefix(ctx context.Context, in *Key, opts ...grpc.CallOption) (*Ack, error) {
	out := new(Ack)
	err := c.cc.Invoke(ctx, StoreService_DeletePrefix_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *storeServiceClient) DeleteRange(ctx context.Context, in *Range, opts ...grpc.CallOption) (*Ack, error) {
	out := new(Ack)
	err := c.cc.Invoke(ctx, StoreService_DeleteRange_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *storeServiceClient) ScanPrefix(ctx context.Context, in *Key, opts ...grpc.CallOption) (StoreService_ScanPrefixClient, error) {
	stream, err := c.cc.NewStream(ctx, &StoreService_ServiceDesc.Streams[0], StoreService_ScanPrefix_FullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	x := &storeServiceScanPrefixClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type StoreService_ScanPrefixClient interface {
	Recv() (*KeyValue, error)
	grpc.ClientStream
}

type storeServiceScanPrefixClient struct {
	grpc.ClientStream
}

func (x *storeServiceScanPrefixClient) Recv() (*KeyValue, error) {
	m := new(KeyValue)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *storeServiceClient) ScanRange(ctx context.Context, in *Range, opts ...grpc.CallOption) (StoreService_ScanRangeClient, error) {
	stream, err := c.cc.NewStream(ctx, &StoreService_ServiceDesc.Streams[1], StoreService_ScanRange_FullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	x := &storeServiceScanRangeClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type StoreService_ScanRangeClient interface {
	Recv() (*KeyValue, error)
	grpc.ClientStream
}

type storeServiceScanRangeClient struct {
	grpc.ClientStream
}

func (x *storeServiceScanRangeClient) Recv() (*KeyValue, error) {
	m := new(KeyValue)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// StoreServiceServer is the server API for StoreService service.
// All implementations must embed UnimplementedStoreServiceServer
// for forward compatibility
type StoreServiceServer interface {
	// Get returns the Value of the given Key. If the Key's key field is not
	// present, the empty key is used for lookup. If the store does not contain
	// the given key, it returns a Value with a value field that is NOT explicitly
	// present.
	//
	// In application code, always use language-specific generated "hasKey"
	// methods to check for key existence in the store, instead of comparing
	// default values.
	//
	// If the operation fails, a gRPC error with status code UNAVAILABLE (14) is
	// signaled.
	Get(context.Context, *Key) (*Value, error)
	// Set sets the value for the given KeyValue pair. It overwrites any previous
	// value for that key.
	//
	// If a value is not present, a gRPC error with status code INVALID_ARGUMENT
	// (3) is signaled. Otherwise, if the operation fails, a gRPC error with
	// status code UNAVAILABLE (14) is signaled.
	Set(context.Context, *KeyValue) (*Ack, error)
	// Delete deletes the value for the given Key.
	//
	// If the operation fails, a gRPC error with status code UNAVAILABLE (14) is
	// signaled.
	Delete(context.Context, *Key) (*Ack, error)
	// DeleteAll deletes all key-value pairs in the store.
	//
	// If the operation fails, a gRPC error with status code UNAVAILABLE (14) is
	// signaled.
	DeleteAll(context.Context, *DeleteAllParams) (*Ack, error)
	// DeletePrefix deletes all of the keys (and values) that start with the given
	// prefix. Key strings are ordered lexicographically by their underlying byte
	// representation, i.e. UTF-8 encoding.
	//
	// If the operation fails, a gRPC error with status code UNAVAILABLE (14) is
	// signaled.
	DeletePrefix(context.Context, *Key) (*Ack, error)
	// DeleteRange deletes all of the keys (and values) in the right-open Range
	// [start,end) (inclusive on start, exclusive on end). Key strings are ordered
	// lexicographically by their underlying byte representation, i.e. UTF-8
	// encoding.
	//
	// If the operation fails, a gRPC error with status code UNAVAILABLE (14) is
	// signaled.
	DeleteRange(context.Context, *Range) (*Ack, error)
	// ScanPrefix iterates over key-value pairs whose keys start with the given
	// prefix Key in key order. Key strings are ordered lexicographically by their
	// underlying byte representation, i.e. UTF-8 encoding.
	//
	// If the operation fails, a gRPC error with status code UNAVAILABLE (14) is
	// signaled.
	//
	// It is not safe to invoke Set, Delete, DeleteAll, DeletePrefix, and
	// DeleteRange operations while receiving data from the stream as such calls
	// may block until the stream is closed. Instead, accumulate key-value pairs
	// and issue such operations after scanning is finished.
	ScanPrefix(*Key, StoreService_ScanPrefixServer) error
	// ScanRange iterates over a given right-open Range of key-value pairs in key
	// order (inclusive on start, exclusive on end). Key strings are ordered
	// lexicographically by their underlying byte representation, i.e. UTF-8
	// encoding.
	//
	// For example, this function can be used to iterate over keys which represent
	// a time range with a sortable time encoding like RFC3339.
	//
	// If the operation fails, a gRPC error with status code UNAVAILABLE (14) is
	// signaled.
	//
	// It is not safe to invoke Set, Delete, DeleteAll, DeletePrefix, and
	// DeleteRange operations while receiving data from the stream as such calls
	// may block until the stream is closed. Instead, accumulate key-value pairs
	// and issue such operations after scanning is finished.
	ScanRange(*Range, StoreService_ScanRangeServer) error
	mustEmbedUnimplementedStoreServiceServer()
}

// UnimplementedStoreServiceServer must be embedded to have forward compatible implementations.
type UnimplementedStoreServiceServer struct {
}

func (UnimplementedStoreServiceServer) Get(context.Context, *Key) (*Value, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Get not implemented")
}
func (UnimplementedStoreServiceServer) Set(context.Context, *KeyValue) (*Ack, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Set not implemented")
}
func (UnimplementedStoreServiceServer) Delete(context.Context, *Key) (*Ack, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Delete not implemented")
}
func (UnimplementedStoreServiceServer) DeleteAll(context.Context, *DeleteAllParams) (*Ack, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteAll not implemented")
}
func (UnimplementedStoreServiceServer) DeletePrefix(context.Context, *Key) (*Ack, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeletePrefix not implemented")
}
func (UnimplementedStoreServiceServer) DeleteRange(context.Context, *Range) (*Ack, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteRange not implemented")
}
func (UnimplementedStoreServiceServer) ScanPrefix(*Key, StoreService_ScanPrefixServer) error {
	return status.Errorf(codes.Unimplemented, "method ScanPrefix not implemented")
}
func (UnimplementedStoreServiceServer) ScanRange(*Range, StoreService_ScanRangeServer) error {
	return status.Errorf(codes.Unimplemented, "method ScanRange not implemented")
}
func (UnimplementedStoreServiceServer) mustEmbedUnimplementedStoreServiceServer() {}

// UnsafeStoreServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to StoreServiceServer will
// result in compilation errors.
type UnsafeStoreServiceServer interface {
	mustEmbedUnimplementedStoreServiceServer()
}

func RegisterStoreServiceServer(s grpc.ServiceRegistrar, srv StoreServiceServer) {
	s.RegisterService(&StoreService_ServiceDesc, srv)
}

func _StoreService_Get_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Key)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(StoreServiceServer).Get(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: StoreService_Get_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(StoreServiceServer).Get(ctx, req.(*Key))
	}
	return interceptor(ctx, in, info, handler)
}

func _StoreService_Set_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(KeyValue)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(StoreServiceServer).Set(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: StoreService_Set_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(StoreServiceServer).Set(ctx, req.(*KeyValue))
	}
	return interceptor(ctx, in, info, handler)
}

func _StoreService_Delete_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Key)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(StoreServiceServer).Delete(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: StoreService_Delete_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(StoreServiceServer).Delete(ctx, req.(*Key))
	}
	return interceptor(ctx, in, info, handler)
}

func _StoreService_DeleteAll_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeleteAllParams)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(StoreServiceServer).DeleteAll(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: StoreService_DeleteAll_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(StoreServiceServer).DeleteAll(ctx, req.(*DeleteAllParams))
	}
	return interceptor(ctx, in, info, handler)
}

func _StoreService_DeletePrefix_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Key)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(StoreServiceServer).DeletePrefix(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: StoreService_DeletePrefix_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(StoreServiceServer).DeletePrefix(ctx, req.(*Key))
	}
	return interceptor(ctx, in, info, handler)
}

func _StoreService_DeleteRange_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Range)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(StoreServiceServer).DeleteRange(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: StoreService_DeleteRange_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(StoreServiceServer).DeleteRange(ctx, req.(*Range))
	}
	return interceptor(ctx, in, info, handler)
}

func _StoreService_ScanPrefix_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(Key)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(StoreServiceServer).ScanPrefix(m, &storeServiceScanPrefixServer{stream})
}

type StoreService_ScanPrefixServer interface {
	Send(*KeyValue) error
	grpc.ServerStream
}

type storeServiceScanPrefixServer struct {
	grpc.ServerStream
}

func (x *storeServiceScanPrefixServer) Send(m *KeyValue) error {
	return x.ServerStream.SendMsg(m)
}

func _StoreService_ScanRange_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(Range)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(StoreServiceServer).ScanRange(m, &storeServiceScanRangeServer{stream})
}

type StoreService_ScanRangeServer interface {
	Send(*KeyValue) error
	grpc.ServerStream
}

type storeServiceScanRangeServer struct {
	grpc.ServerStream
}

func (x *storeServiceScanRangeServer) Send(m *KeyValue) error {
	return x.ServerStream.SendMsg(m)
}

// StoreService_ServiceDesc is the grpc.ServiceDesc for StoreService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var StoreService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "dda.store.v1.StoreService",
	HandlerType: (*StoreServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Get",
			Handler:    _StoreService_Get_Handler,
		},
		{
			MethodName: "Set",
			Handler:    _StoreService_Set_Handler,
		},
		{
			MethodName: "Delete",
			Handler:    _StoreService_Delete_Handler,
		},
		{
			MethodName: "DeleteAll",
			Handler:    _StoreService_DeleteAll_Handler,
		},
		{
			MethodName: "DeletePrefix",
			Handler:    _StoreService_DeletePrefix_Handler,
		},
		{
			MethodName: "DeleteRange",
			Handler:    _StoreService_DeleteRange_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "ScanPrefix",
			Handler:       _StoreService_ScanPrefix_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "ScanRange",
			Handler:       _StoreService_ScanRange_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "store.proto",
}
