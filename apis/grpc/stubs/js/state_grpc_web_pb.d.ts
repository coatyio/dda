import * as grpcWeb from 'grpc-web';

import * as state_pb from './state_pb';


export class StateServiceClient {
  constructor (hostname: string,
               credentials?: null | { [index: string]: string; },
               options?: null | { [index: string]: any; });

  proposeInput(
    request: state_pb.Input,
    metadata: grpcWeb.Metadata | undefined,
    callback: (err: grpcWeb.RpcError,
               response: state_pb.Ack) => void
  ): grpcWeb.ClientReadableStream<state_pb.Ack>;

  observeStateChange(
    request: state_pb.ObserveStateChangeParams,
    metadata?: grpcWeb.Metadata
  ): grpcWeb.ClientReadableStream<state_pb.Input>;

  observeMembershipChange(
    request: state_pb.ObserveMembershipChangeParams,
    metadata?: grpcWeb.Metadata
  ): grpcWeb.ClientReadableStream<state_pb.MembershipChange>;

}

export class StateServicePromiseClient {
  constructor (hostname: string,
               credentials?: null | { [index: string]: string; },
               options?: null | { [index: string]: any; });

  proposeInput(
    request: state_pb.Input,
    metadata?: grpcWeb.Metadata
  ): Promise<state_pb.Ack>;

  observeStateChange(
    request: state_pb.ObserveStateChangeParams,
    metadata?: grpcWeb.Metadata
  ): grpcWeb.ClientReadableStream<state_pb.Input>;

  observeMembershipChange(
    request: state_pb.ObserveMembershipChangeParams,
    metadata?: grpcWeb.Metadata
  ): grpcWeb.ClientReadableStream<state_pb.MembershipChange>;

}

