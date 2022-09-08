// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package messages_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"

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
	t.Parallel()

	t.Run("check that the outputs of both messages are the same", func(t *testing.T) {
		t.Parallel()

		runTestPipe[messages.BasicMessage](t, BasicMessage{
			Int64:      1,
			UInt64:     2,
			Fixed64:    protoenc.FixedU64(3),
			SomeString: "some string",
			SomeBytes:  []byte("some bytes"),
		})
	})

	t.Run("check that the outputs of both zero messages are the same", func(t *testing.T) {
		t.Parallel()

		runTestPipe[messages.BasicMessage](t, BasicMessage{})
	})

	t.Run("check that the outputs of both somewhat empty messages are the same", func(t *testing.T) {
		t.Parallel()

		runTestPipe[messages.BasicMessage](t, BasicMessage{SomeString: "some string"})
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
	t.Parallel()

	t.Run("check that the outputs of both messages are the same", func(t *testing.T) {
		t.Parallel()

		runTestPipe[messages.MessageRepeatedFields](t, MessageRepeatedFields{
			Int64:      []int64{1, 2, 3},
			UInt64:     []uint64{4, 5, 6},
			Fixed64:    []protoenc.FixedU64{7, 8, 9},
			SomeString: []string{"some string", "some string 2"},
			SomeBytes:  [][]byte{[]byte("some bytes"), []byte("some bytes 2")},
		})
	})

	t.Run("check that the outputs of both zero messages are the same", func(t *testing.T) {
		t.Parallel()

		runTestPipe[messages.MessageRepeatedFields](t, MessageRepeatedFields{})
	})

	t.Run("check that the outputs of both somewhat empty messages are the same", func(t *testing.T) {
		t.Parallel()

		runTestPipe[messages.MessageRepeatedFields](t, MessageRepeatedFields{
			SomeString: []string{"some string"},
		})
	})
}

type BasicMessageRep struct {
	BasicMessage []BasicMessage `protobuf:"1"`
}

func TestBasicMessageRep(t *testing.T) {
	t.Parallel()

	t.Run("check that the outputs of both messages are the same", func(t *testing.T) {
		t.Parallel()

		runTestPipe[messages.BasicMessageRep](t, BasicMessageRep{
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
		})
	})

	t.Run("check that the outputs of both zero messages are the same", func(t *testing.T) {
		t.Parallel()

		runTestPipe[messages.BasicMessageRep](t, BasicMessageRep{})
	})

	t.Run("check that the outputs of both somewhat empty messages are the same", func(t *testing.T) {
		t.Parallel()

		runTestPipe[messages.BasicMessageRep](t, BasicMessageRep{
			BasicMessage: []BasicMessage{
				{
					Fixed64: protoenc.FixedU64(3),
				},
			},
		})
	})
}

type MessageComplexFields struct {
	MapToMsg     map[string]BasicMessage    `protobuf:"1"`
	MapToMsgs    map[string]BasicMessageRep `protobuf:"2"`
	PrimitiveMap map[string]int64           `protobuf:"3"`
}

func TestMessageComplexFields(t *testing.T) {
	t.Parallel()

	t.Run("check that the outputs of both messages are the same", func(t *testing.T) {
		t.Parallel()

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

		runTestPipe[messages.MessageComplexFields](t, originalMsg)
	})

	t.Run("check that the outputs of both zero messages are the same", func(t *testing.T) {
		t.Parallel()

		runTestPipe[messages.MessageComplexFields](t, MessageComplexFields{})
	})

	t.Run("check that the outputs of both somewhat empty messages are the same", func(t *testing.T) {
		t.Parallel()

		originalMsg := MessageComplexFields{
			MapToMsg: map[string]BasicMessage{
				"key": {
					Int64: 1,
				},
				"": {
					Int64: 30,
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

		runTestPipe[messages.MessageComplexFields](t, originalMsg)
	})
}

func TestEmptyMessage(t *testing.T) {
	t.Parallel()

	t.Run("empty message", func(t *testing.T) {
		t.Parallel()

		type emptyMessage struct{}

		runTestPipe[emptypb.Empty](t, emptyMessage{})
	})

	t.Run("slice of empty messages", func(t *testing.T) {
		t.Parallel()

		type emptyMessage struct{}

		type emptyMessageRep struct {
			EmptyMessage []emptyMessage `protobuf:"1"`
		}

		runTestPipe[messages.EmptyMessageRep](t, emptyMessageRep{
			EmptyMessage: make([]emptyMessage, 10),
		})
	})

	t.Run("test message containing empty message", func(t *testing.T) {
		t.Parallel()

		type emptyMessage struct{}

		type messageWithEmptpy struct { //nolint:govet
			BasicMessage BasicMessage `protobuf:"1"`
			EmptyMessage emptyMessage `protobuf:"2"`
		}

		runTestPipe[messages.MessageWithEmptpy](t, messageWithEmptpy{
			BasicMessage: BasicMessage{
				Int64:      1,
				UInt64:     2,
				Fixed64:    protoenc.FixedU64(3),
				SomeString: "some string",
				SomeBytes:  []byte("some bytes"),
			},
			EmptyMessage: emptyMessage{},
		})
	})
}

func TestEnumMessage_CompatibleOldScheme(t *testing.T) {
	// This test ensures that we can decode a message with an enum field encoded by previus version of our encoder.
	t.Parallel()

	encoded := []byte{0x0d, 0x01, 0x00, 0x00, 0x00}

	type Enum int8

	type EnumMessage struct {
		EnumField Enum `protobuf:"1"`
	}

	dest := EnumMessage{}

	err := protoenc.Unmarshal(encoded, &dest)
	require.NoError(t, err)

	require.EqualValues(t, dest.EnumField, 1)
}

func TestEnumMessage(t *testing.T) {
	t.Parallel()

	type Enum int8

	type EnumMessage struct {
		EnumField Enum `protobuf:"1"`
	}

	runTestPipe[messages.EnumMessage](t, EnumMessage{
		EnumField: 1,
	})
}
