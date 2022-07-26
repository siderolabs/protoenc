// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package messages_test

import (
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/siderolabs/protoenc"
	"github.com/siderolabs/protoenc/messages"
)

// TODO: ensure that binary output is also the same

//nolint:govet
type BasicMessage struct {
	Int64      int64             `protobuf:"1"`
	UInt64     uint64            `protobuf:"3"`
	Fixed64    protoenc.FixedU64 `protobuf:"5"`
	SomeString string            `protobuf:"6"`
	SomeBytes  []byte            `protobuf:"7"`
}

func TestBasicMessage(t *testing.T) {
	t.Run("check that the outputs of both messages are the same", func(t *testing.T) {
		ourBasicMessage := BasicMessage{
			Int64:      1,
			UInt64:     2,
			Fixed64:    protoenc.FixedU64(3),
			SomeString: "some string",
			SomeBytes:  []byte("some bytes"),
		}

		encoded1 := must(protoenc.Marshal(&ourBasicMessage))(t)
		basicMessage := protoUnmarshal[messages.BasicMessage](t, encoded1)
		encoded2 := must(proto.Marshal(basicMessage))(t)
		decodedMessage := ourUnmarshal[BasicMessage](t, encoded2)

		shouldBeEqual(t, ourBasicMessage, decodedMessage)
	})

	t.Run("check that the outputs of both zero messages are the same", func(t *testing.T) {
		ourBasicMessage := BasicMessage{}
		encoded1 := must(protoenc.Marshal(&ourBasicMessage))(t)
		basicMessage := protoUnmarshal[messages.BasicMessage](t, encoded1)
		encoded2 := must(proto.Marshal(basicMessage))(t)
		decodedMessage := ourUnmarshal[BasicMessage](t, encoded2)

		shouldBeEqual(t, ourBasicMessage, decodedMessage)
	})

	t.Run("check that the outputs of both somewhat empty messages are the same", func(t *testing.T) {
		ourBasicMessage := BasicMessage{SomeString: "some string"}
		encoded1 := must(protoenc.Marshal(&ourBasicMessage))(t)
		basicMessage := protoUnmarshal[messages.BasicMessage](t, encoded1)
		encoded2 := must(proto.Marshal(basicMessage))(t)
		decodedMessage := ourUnmarshal[BasicMessage](t, encoded2)

		shouldBeEqual(t, ourBasicMessage, decodedMessage)
	})
}

type MessageRepeatedFields struct {
	Int64      []int64             `protobuf:"1"`
	UInt64     []uint64            `protobuf:"3"`
	Fixed64    []protoenc.FixedU64 `protobuf:"5"`
	SomeString []string            `protobuf:"6"`
	SomeBytes  [][]byte            `protobuf:"7"`
}

func TestMessageRepeatedFields(t *testing.T) {
	t.Run("check that the outputs of both messages are the same", func(t *testing.T) {
		originalMsg := MessageRepeatedFields{
			Int64:      []int64{1, 2, 3},
			UInt64:     []uint64{4, 5, 6},
			Fixed64:    []protoenc.FixedU64{7, 8, 9},
			SomeString: []string{"some string", "some string 2"},
			SomeBytes:  [][]byte{[]byte("some bytes"), []byte("some bytes 2")},
		}

		encoded1 := must(protoenc.Marshal(&originalMsg))(t)
		decodedMsg := protoUnmarshal[messages.MessageRepeatedFields](t, encoded1)
		encoded2 := must(proto.Marshal(decodedMsg))(t)
		resultMsg := ourUnmarshal[MessageRepeatedFields](t, encoded2)

		shouldBeEqual(t, originalMsg, resultMsg)
	})

	t.Run("check that the outputs of both zero messages are the same", func(t *testing.T) {
		ourBasicMessage := MessageRepeatedFields{}
		encoded1 := must(protoenc.Marshal(&ourBasicMessage))(t)
		basicMessage := protoUnmarshal[messages.MessageRepeatedFields](t, encoded1)
		encoded2 := must(proto.Marshal(basicMessage))(t)
		decodedMessage := ourUnmarshal[MessageRepeatedFields](t, encoded2)

		shouldBeEqual(t, ourBasicMessage, decodedMessage)
	})

	t.Run("check that the outputs of both somewhat empty messages are the same", func(t *testing.T) {
		ourBasicMessage := MessageRepeatedFields{SomeString: []string{"some string"}}
		encoded1 := must(protoenc.Marshal(&ourBasicMessage))(t)
		basicMessage := protoUnmarshal[messages.MessageRepeatedFields](t, encoded1)
		encoded2 := must(proto.Marshal(basicMessage))(t)
		decodedMessage := ourUnmarshal[MessageRepeatedFields](t, encoded2)

		shouldBeEqual(t, ourBasicMessage, decodedMessage)
	})
}

type BasicMessageRep struct {
	BasicMessage []BasicMessage `protobuf:"1"`
}

func TestBasicMessageRep(t *testing.T) {
	t.Run("check that the outputs of both messages are the same", func(t *testing.T) {
		originalMsg := BasicMessageRep{
			BasicMessage: []BasicMessage{
				{
					Int64:      1,
					UInt64:     2,
					Fixed64:    protoenc.FixedU64(3),
					SomeString: "some string",
					SomeBytes:  []byte("some bytes"),
				},
				{
					Int64:      2,
					UInt64:     3,
					Fixed64:    protoenc.FixedU64(5),
					SomeString: "hot string",
					SomeBytes:  []byte("hot bytes"),
				},
			},
		}
		encoded1 := must(protoenc.Marshal(&originalMsg))(t)
		decodedMsg := protoUnmarshal[messages.BasicMessageRep](t, encoded1)
		encoded2 := must(proto.Marshal(decodedMsg))(t)
		resultMsg := ourUnmarshal[BasicMessageRep](t, encoded2)

		shouldBeEqual(t, originalMsg, resultMsg)
	})

	t.Run("check that the outputs of both zero messages are the same", func(t *testing.T) {
		ourBasicMessage := BasicMessageRep{}
		encoded1 := must(protoenc.Marshal(&ourBasicMessage))(t)
		basicMessage := protoUnmarshal[messages.BasicMessageRep](t, encoded1)
		encoded2 := must(proto.Marshal(basicMessage))(t)
		decodedMessage := ourUnmarshal[BasicMessageRep](t, encoded2)

		shouldBeEqual(t, ourBasicMessage, decodedMessage)
	})

	t.Run("check that the outputs of both somewhat empty messages are the same", func(t *testing.T) {
		ourBasicMessage := BasicMessageRep{
			BasicMessage: []BasicMessage{
				{
					Fixed64: protoenc.FixedU64(3),
				},
			},
		}
		encoded1 := must(protoenc.Marshal(&ourBasicMessage))(t)
		basicMessage := protoUnmarshal[messages.BasicMessageRep](t, encoded1)
		encoded2 := must(proto.Marshal(basicMessage))(t)
		decodedMessage := ourUnmarshal[BasicMessageRep](t, encoded2)

		shouldBeEqual(t, ourBasicMessage, decodedMessage)
	})
}

type MessageComplexFields struct {
	MapToMsg     map[string]BasicMessage    `protobuf:"1"`
	MapToMsgs    map[string]BasicMessageRep `protobuf:"2"`
	PrimitiveMap map[string]int64           `protobuf:"3"`
}

func TestMessageComplexFields(t *testing.T) {
	t.Run("check that the outputs of both messages are the same", func(t *testing.T) {
		originalMsg := MessageComplexFields{
			MapToMsg: map[string]BasicMessage{
				"key": {
					Int64:      1,
					UInt64:     2,
					Fixed64:    protoenc.FixedU64(3),
					SomeString: "some string",
					SomeBytes:  []byte("some bytes"),
				},
			},
			MapToMsgs: map[string]BasicMessageRep{
				"key": {
					BasicMessage: []BasicMessage{
						{
							Int64:      1,
							UInt64:     2,
							Fixed64:    protoenc.FixedU64(3),
							SomeString: "some string",
							SomeBytes:  []byte("some bytes"),
						},
						{
							Int64:      2,
							UInt64:     3,
							Fixed64:    protoenc.FixedU64(5),
							SomeString: "hot string",
							SomeBytes:  []byte("hot bytes"),
						},
					},
				},
				"another key": {
					BasicMessage: []BasicMessage{
						{
							Int64:      12,
							UInt64:     13,
							Fixed64:    protoenc.FixedU64(15),
							SomeString: "another string",
							SomeBytes:  []byte("another bytes"),
						},
						{
							Int64:      15,
							UInt64:     17,
							Fixed64:    protoenc.FixedU64(19),
							SomeString: "another hot string",
							SomeBytes:  []byte("another hot bytes"),
						},
					},
				},
			},
			PrimitiveMap: map[string]int64{
				"key":   1,
				"key2":  2,
				"empty": 0,
			},
		}
		encoded1 := must(protoenc.Marshal(&originalMsg))(t)
		decodedMsg := protoUnmarshal[messages.MessageComplexFields](t, encoded1)
		encoded2 := must(proto.Marshal(decodedMsg))(t)
		resultMsg := ourUnmarshal[MessageComplexFields](t, encoded2)

		shouldBeEqual(t, originalMsg, resultMsg)
	})

	t.Run("check that the outputs of both zero messages are the same", func(t *testing.T) {
		ourBasicMessage := MessageComplexFields{}
		encoded1 := must(protoenc.Marshal(&ourBasicMessage))(t)
		basicMessage := protoUnmarshal[messages.MessageComplexFields](t, encoded1)
		encoded2 := must(proto.Marshal(basicMessage))(t)
		decodedMessage := ourUnmarshal[MessageComplexFields](t, encoded2)

		shouldBeEqual(t, ourBasicMessage, decodedMessage)
	})

	t.Run("check that the outputs of both somewhat empty messages are the same", func(t *testing.T) {
		originalMsg := MessageComplexFields{
			MapToMsg: map[string]BasicMessage{
				"key": {
					Int64: 1,
				},
			},
			MapToMsgs: map[string]BasicMessageRep{
				"key": {
					BasicMessage: []BasicMessage{
						{
							Int64:     1,
							SomeBytes: []byte("some bytes"),
						},
						{
							Fixed64: protoenc.FixedU64(5),
						},
					},
				},
				"another key": {
					BasicMessage: []BasicMessage{
						{
							SomeBytes: []byte("another bytes"),
						},
						{
							Int64:  15,
							UInt64: 17,
						},
					},
				},
			},
			PrimitiveMap: map[string]int64{
				"key": 1,
			},
		}
		encoded1 := must(protoenc.Marshal(&originalMsg))(t)
		decodedMsg := protoUnmarshal[messages.MessageComplexFields](t, encoded1)
		encoded2 := must(proto.Marshal(decodedMsg))(t)
		resultMsg := ourUnmarshal[MessageComplexFields](t, encoded2)

		shouldBeEqual(t, originalMsg, resultMsg)
	})
}
