// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package protoenc_test

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/protoenc"
)

//nolint:govet
type Inner struct {
	ID   int32  `protobuf:"1"`
	Name string `protobuf:"2"`
}

type FloatingPoint struct {
	F *float64 `protobuf:"1"`
}

type MessageWithMap struct {
	NameMapping   map[uint32]string        `protobuf:"1"`
	ByteMapping   map[bool][]byte          `protobuf:"2"`
	MsgMapping    map[int64]*FloatingPoint `protobuf:"3"`
	StrToStr      map[string]string        `protobuf:"5"`
	StructMapping map[string]*Inner        `protobuf:"6"`
}

func TestMapFieldEncode(t *testing.T) {
	m := &MessageWithMap{
		NameMapping: map[uint32]string{
			1: "Rob",
			4: "Ian",
			8: "Dave",
		},
	}

	b, err := protoenc.Marshal(m)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	// b should be the concatenation of these three byte sequences in some order.
	parts := []string{
		"\n\a\b\x01\x12\x03Rob",
		"\n\a\b\x04\x12\x03Ian",
		"\n\b\b\x08\x12\x04Dave",
	}

	ok := false

	for i := range parts {
		for j := range parts {
			if j == i {
				continue
			}

			for k := range parts {
				if k == i || k == j {
					continue
				}

				try := parts[i] + parts[j] + parts[k]

				if bytes.Equal(b, []byte(try)) {
					ok = true

					break
				}
			}
		}
	}

	if !ok {
		t.Fatalf("Incorrect Encoding output.\n got %q\nwant %q (or a permutation of that)", b, parts[0]+parts[1]+parts[2])
	}

	t.Logf("FYI b: %q", b)
}

func TestMapFieldRoundTrips(t *testing.T) {
	Float := float64(2.0)
	m := &MessageWithMap{
		NameMapping: map[uint32]string{
			1: "Rob",
			4: "Ian",
			8: "Dave",
		},
		MsgMapping: map[int64]*FloatingPoint{
			0x7001: {F: &Float},
		},
		ByteMapping: map[bool][]byte{
			false: []byte("that's not right!"),
			true:  []byte("aye, 'tis true!"),
		},
		StrToStr: map[string]string{
			"a key":     "value",
			"other key": "other value",
		},
		StructMapping: map[string]*Inner{
			"first":  {ID: 1, Name: "one"},
			"second": {ID: 5, Name: "two"},
		},
	}

	b, err := protoenc.Marshal(m)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	t.Logf("FYI b: %q", b)

	m2 := new(MessageWithMap)
	if err := protoenc.Unmarshal(b, m2); err != nil {
		t.Fatalf("Decode: %v", err)
	}

	for _, pair := range [][2]interface{}{
		{m.NameMapping, m2.NameMapping},
		{m.MsgMapping, m2.MsgMapping},
		{m.ByteMapping, m2.ByteMapping},
		{m.StrToStr, m2.StrToStr},
		{m.StructMapping, m2.StructMapping},
	} {
		if !reflect.DeepEqual(pair[0], pair[1]) {
			t.Errorf("Map did not survive a round trip.\ninitial: %v\n  final: %v", pair[0], pair[1])
		}
	}
}

func TestMapFieldWithNil(t *testing.T) {
	m := &MessageWithMap{
		MsgMapping: map[int64]*FloatingPoint{
			1: nil,
		},
	}

	b, err := protoenc.Marshal(m)
	if err == nil {
		t.Fatalf("Marshal of bad map should have failed, got these bytes: %v", b)
	}
}

type WrongMap struct {
	Map map[string][]uint32
}

func TestMapWrongSliceValue(t *testing.T) {
	w := &WrongMap{}
	w.Map = make(map[string][]uint32)
	w.Map["hello"] = []uint32{1, 2, 3}
	w.Map["world"] = []uint32{4, 5, 6}

	_, err := protoenc.Marshal(w)
	assert.NotNil(t, err)
}
