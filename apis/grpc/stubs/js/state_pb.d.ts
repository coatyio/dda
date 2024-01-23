import * as jspb from 'google-protobuf'



export class Ack extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): Ack.AsObject;
  static toObject(includeInstance: boolean, msg: Ack): Ack.AsObject;
  static serializeBinaryToWriter(message: Ack, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): Ack;
  static deserializeBinaryFromReader(message: Ack, reader: jspb.BinaryReader): Ack;
}

export namespace Ack {
  export type AsObject = {
  }
}

export class Input extends jspb.Message {
  getOp(): InputOperation;
  setOp(value: InputOperation): Input;

  getKey(): string;
  setKey(value: string): Input;

  getValue(): Uint8Array | string;
  getValue_asU8(): Uint8Array;
  getValue_asB64(): string;
  setValue(value: Uint8Array | string): Input;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): Input.AsObject;
  static toObject(includeInstance: boolean, msg: Input): Input.AsObject;
  static serializeBinaryToWriter(message: Input, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): Input;
  static deserializeBinaryFromReader(message: Input, reader: jspb.BinaryReader): Input;
}

export namespace Input {
  export type AsObject = {
    op: InputOperation,
    key: string,
    value: Uint8Array | string,
  }
}

export class ObserveStateChangeParams extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ObserveStateChangeParams.AsObject;
  static toObject(includeInstance: boolean, msg: ObserveStateChangeParams): ObserveStateChangeParams.AsObject;
  static serializeBinaryToWriter(message: ObserveStateChangeParams, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ObserveStateChangeParams;
  static deserializeBinaryFromReader(message: ObserveStateChangeParams, reader: jspb.BinaryReader): ObserveStateChangeParams;
}

export namespace ObserveStateChangeParams {
  export type AsObject = {
  }
}

export class ObserveMembershipChangeParams extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ObserveMembershipChangeParams.AsObject;
  static toObject(includeInstance: boolean, msg: ObserveMembershipChangeParams): ObserveMembershipChangeParams.AsObject;
  static serializeBinaryToWriter(message: ObserveMembershipChangeParams, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ObserveMembershipChangeParams;
  static deserializeBinaryFromReader(message: ObserveMembershipChangeParams, reader: jspb.BinaryReader): ObserveMembershipChangeParams;
}

export namespace ObserveMembershipChangeParams {
  export type AsObject = {
  }
}

export class MembershipChange extends jspb.Message {
  getId(): string;
  setId(value: string): MembershipChange;

  getJoined(): boolean;
  setJoined(value: boolean): MembershipChange;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MembershipChange.AsObject;
  static toObject(includeInstance: boolean, msg: MembershipChange): MembershipChange.AsObject;
  static serializeBinaryToWriter(message: MembershipChange, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MembershipChange;
  static deserializeBinaryFromReader(message: MembershipChange, reader: jspb.BinaryReader): MembershipChange;
}

export namespace MembershipChange {
  export type AsObject = {
    id: string,
    joined: boolean,
  }
}

export enum InputOperation { 
  INPUT_OPERATION_UNSPECIFIED = 0,
  INPUT_OPERATION_SET = 1,
  INPUT_OPERATION_DELETE = 2,
}
