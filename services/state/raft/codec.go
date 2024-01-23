// SPDX-FileCopyrightText: © 2024 Siemens AG
// SPDX-License-Identifier: MIT

// Package raft exports [MessagePack] codec functionality to serialize log
// entries, LogStore items, and FSM snaphosts.
//
// [MessagePack]: https://msgpack.org/
package raft

import (
	"io"

	"github.com/hashicorp/go-msgpack/v2/codec"
)

// DecodeMsgPack decodes from a MessagePack encoded byte slice.
func DecodeMsgPack(b []byte, out any) error {
	var hd codec.MsgpackHandle
	dec := codec.NewDecoderBytes(b, &hd)
	return dec.Decode(out)
}

// DecodeMsgPackFromReader decodes from a MessagePack encoded reader.
func DecodeMsgPackFromReader(r io.ReadCloser, out any) error {
	var hd codec.MsgpackHandle
	dec := codec.NewDecoder(r, &hd)
	return dec.Decode(out)
}

// EncodeMsgPack returns an encoded MessagePack object as a byte slice.
func EncodeMsgPack(in any) ([]byte, error) {
	var b []byte
	var hd codec.MsgpackHandle
	enc := codec.NewEncoderBytes(&b, &hd)
	err := enc.Encode(in)
	return b, err
}
