import * as grpcWeb from 'grpc-web';

import * as store_pb from './store_pb';


export class StoreServiceClient {
  constructor (hostname: string,
               credentials?: null | { [index: string]: string; },
               options?: null | { [index: string]: any; });

  get(
    request: store_pb.Key,
    metadata: grpcWeb.Metadata | undefined,
    callback: (err: grpcWeb.RpcError,
               response: store_pb.Value) => void
  ): grpcWeb.ClientReadableStream<store_pb.Value>;

  set(
    request: store_pb.KeyValue,
    metadata: grpcWeb.Metadata | undefined,
    callback: (err: grpcWeb.RpcError,
               response: store_pb.Ack) => void
  ): grpcWeb.ClientReadableStream<store_pb.Ack>;

  delete(
    request: store_pb.Key,
    metadata: grpcWeb.Metadata | undefined,
    callback: (err: grpcWeb.RpcError,
               response: store_pb.Ack) => void
  ): grpcWeb.ClientReadableStream<store_pb.Ack>;

  deleteAll(
    request: store_pb.DeleteAllParams,
    metadata: grpcWeb.Metadata | undefined,
    callback: (err: grpcWeb.RpcError,
               response: store_pb.Ack) => void
  ): grpcWeb.ClientReadableStream<store_pb.Ack>;

  deletePrefix(
    request: store_pb.Key,
    metadata: grpcWeb.Metadata | undefined,
    callback: (err: grpcWeb.RpcError,
               response: store_pb.Ack) => void
  ): grpcWeb.ClientReadableStream<store_pb.Ack>;

  deleteRange(
    request: store_pb.Range,
    metadata: grpcWeb.Metadata | undefined,
    callback: (err: grpcWeb.RpcError,
               response: store_pb.Ack) => void
  ): grpcWeb.ClientReadableStream<store_pb.Ack>;

  scanPrefix(
    request: store_pb.Key,
    metadata?: grpcWeb.Metadata
  ): grpcWeb.ClientReadableStream<store_pb.KeyValue>;

  scanRange(
    request: store_pb.Range,
    metadata?: grpcWeb.Metadata
  ): grpcWeb.ClientReadableStream<store_pb.KeyValue>;

}

export class StoreServicePromiseClient {
  constructor (hostname: string,
               credentials?: null | { [index: string]: string; },
               options?: null | { [index: string]: any; });

  get(
    request: store_pb.Key,
    metadata?: grpcWeb.Metadata
  ): Promise<store_pb.Value>;

  set(
    request: store_pb.KeyValue,
    metadata?: grpcWeb.Metadata
  ): Promise<store_pb.Ack>;

  delete(
    request: store_pb.Key,
    metadata?: grpcWeb.Metadata
  ): Promise<store_pb.Ack>;

  deleteAll(
    request: store_pb.DeleteAllParams,
    metadata?: grpcWeb.Metadata
  ): Promise<store_pb.Ack>;

  deletePrefix(
    request: store_pb.Key,
    metadata?: grpcWeb.Metadata
  ): Promise<store_pb.Ack>;

  deleteRange(
    request: store_pb.Range,
    metadata?: grpcWeb.Metadata
  ): Promise<store_pb.Ack>;

  scanPrefix(
    request: store_pb.Key,
    metadata?: grpcWeb.Metadata
  ): grpcWeb.ClientReadableStream<store_pb.KeyValue>;

  scanRange(
    request: store_pb.Range,
    metadata?: grpcWeb.Metadata
  ): grpcWeb.ClientReadableStream<store_pb.KeyValue>;

}

