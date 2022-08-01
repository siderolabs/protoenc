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

	for i := 0; i < b.N; i++ {
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

	o := OneFieldStruct[CustomEncoderStruct]{
		Field: CustomEncoderStruct{
			Value: 150,
		},
	}

	protoenc.RegisterEncoderDecoder(encodeCustomEncoderStruct, decodeCustomEncoderStruct)

	encoded, err := protoenc.Marshal(&o)
	require.NoError(b, err)

	b.ResetTimer()
	b.ReportAllocs()

	target := &OneFieldStruct[CustomEncoderStruct]{}
	for i := 0; i < b.N; i++ {
		*target = OneFieldStruct[CustomEncoderStruct]{}

		err := protoenc.Unmarshal(encoded, target)
		if err != nil {
			b.Fatal(err)
		}
	}
}
