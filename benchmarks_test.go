// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package protoenc_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/protoenc"
)

//nolint:govet
type MyStruct struct {
	A int32   `protobuf:"1"`
	B int64   `protobuf:"2"`
	C string  `protobuf:"3"`
	D bool    `protobuf:"4"`
	E float64 `protobuf:"5"`
}

var Store []byte

func BenchmarkEncode(b *testing.B) {
	var s MyStruct

	b.ReportAllocs()

	for i := range b.N {
		s = MyStruct{
			A: int32(i),
			B: int64(i),
			C: "benchmark",
			D: true,
			E: float64(i),
		}

		result, err := protoenc.Marshal(&s)
		if err != nil {
			b.Fatal(err)
		}

		Store = result
	}
}

func BenchmarkCustom(b *testing.B) {
	b.Cleanup(func() {
		protoenc.CleanEncoderDecoder()
	})

	o := Value[CustomEncoderStruct]{
		V: CustomEncoderStruct{
			Value: 150,
		},
	}

	protoenc.RegisterEncoderDecoder(encodeCustomEncoderStruct, decodeCustomEncoderStruct)

	encoded, err := protoenc.Marshal(&o)
	require.NoError(b, err)

	b.ResetTimer()
	b.ReportAllocs()

	target := &Value[CustomEncoderStruct]{}
	for range b.N {
		*target = Value[CustomEncoderStruct]{}

		err := protoenc.Unmarshal(encoded, target)
		if err != nil {
			b.Fatal(err)
		}
	}

	require.Equal(b, o.V.Value+2, target.V.Value)
}

func BenchmarkSlice(b *testing.B) {
	type structWithSlice struct {
		Field []int `protobuf:"1"`
	}

	type structType struct {
		Field structWithSlice `protobuf:"1"`
	}

	o := structType{
		Field: structWithSlice{
			Field: []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 1500, 1600},
		},
	}

	encoded, err := protoenc.Marshal(&o)
	require.NoError(b, err)

	b.ResetTimer()
	b.ReportAllocs()

	target := &structType{}
	for range b.N {
		*target = structType{}

		err := protoenc.Unmarshal(encoded, target)
		if err != nil {
			b.Fatal(err)
		}
	}

	require.Equal(b, o, *target)
}

func BenchmarkString(b *testing.B) {
	type structWithString struct {
		Field string `protobuf:"1"`
	}

	type structType struct {
		Field structWithString `protobuf:"1"`
	}

	o := structType{
		Field: structWithString{
			Field: "stuff to benchmark",
		},
	}

	encoded, err := protoenc.Marshal(&o)
	require.NoError(b, err)

	b.ResetTimer()
	b.ReportAllocs()

	target := &structType{}
	for range b.N {
		*target = structType{}

		err := protoenc.Unmarshal(encoded, target)
		if err != nil {
			b.Fatal(err)
		}
	}

	require.Equal(b, o, *target)
}
