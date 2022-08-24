// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package protoenc_test

import (
	"encoding/hex"
	"math/big"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protowire"

	"github.com/siderolabs/protoenc"
)

func TestByteOverwrite(t *testing.T) {
	t.Parallel()

	// This test ensures that if we append to a byte slice in buffer it doesn't affect others
	encoded := Pair[[]byte, []byte]{
		First:  []byte("test"),
		Second: []byte("end"),
	}
	buf, err := protoenc.Marshal(&encoded)
	require.NoError(t, err)

	var decoded Pair[[]byte, []byte]
	err = protoenc.Unmarshal(buf, &decoded)
	require.NoError(t, err)

	assert.Equal(t, []byte("test"), decoded.First)
	assert.Equal(t, []byte("end"), decoded.Second)
	assert.Equal(t, len(decoded.First), cap(decoded.First))
	assert.Equal(t, len(decoded.Second), cap(decoded.Second))

	b1 := append(decoded.First, "-lol"...) //nolint:gocritic
	assert.Equal(t, []byte("test-lol"), b1)
	assert.Equal(t, []byte("end"), decoded.Second)
}

type Pair[T, V any] struct {
	First  T `protobuf:"1"`
	Second V `protobuf:"2"`
}

func TestBinaryMarshaler(t *testing.T) {
	t.Parallel()

	encoded := StructWithMarshaller{&IntWrapper{Val: 150}}
	buf := must(protoenc.Marshal(&encoded))(t)

	var decoded StructWithMarshaller

	require.NoError(t, protoenc.Unmarshal(buf, &decoded))
	require.Equal(t, encoded.Field.Val, decoded.Field.Val)
}

type StructWithMarshaller struct {
	Field *IntWrapper `protobuf:"1"`
}

type IntWrapper struct {
	Val int
}

func (i *IntWrapper) MarshalBinary() ([]byte, error) {
	return protowire.AppendVarint(nil, uint64(i.Val)), nil
}

func (i *IntWrapper) UnmarshalBinary(data []byte) error {
	res, _ := protowire.ConsumeVarint(data)
	i.Val = int(res)

	return nil
}

func Test2dSlice(t *testing.T) {
	t.Parallel()

	t.Run("should fail on 2d int slice", func(t *testing.T) {
		t.Parallel()

		encoded := Value[[][]int]{V: [][]int{{1, 2, 3}, {4, 5, 6}}}
		_, err := protoenc.Marshal(&encoded)
		require.Error(t, err)
	})

	t.Run("should fail on 2d uint16 slice", func(t *testing.T) {
		t.Parallel()

		encoded := Value[[][]uint16]{V: [][]uint16{{1, 2, 3}, {4, 5, 6}}}
		_, err := protoenc.Marshal(&encoded)
		require.Error(t, err)
	})

	t.Run("should ok on 2d byte slice", func(t *testing.T) {
		t.Parallel()

		encoded := Value[[][]byte]{V: [][]byte{{1, 2, 3}, {4, 5, 6}}}
		buf := must(protoenc.Marshal(&encoded))(t)

		decoded := Value[[][]byte]{}
		require.NoError(t, protoenc.Unmarshal(buf, &decoded))

		require.Equal(t, encoded.V, decoded.V)
	})
}

func TestBigInt(t *testing.T) {
	t.Parallel()

	encoded := bigIntWrapper{Int: new(big.Int)}
	decoded := bigIntWrapper{Int: new(big.Int)}

	encoded.Int.SetUint64(150)
	buf, err := protoenc.Marshal(&encoded)
	require.NoError(t, err)
	assert.Equal(t, []byte{0, 150}, buf)
	err = protoenc.Unmarshal(buf, &decoded)
	require.NoError(t, err)
	assert.Equal(t, "150", decoded.Int.String())

	encoded.Int.SetInt64(-150)
	buf, err = protoenc.Marshal(&encoded)
	require.NoError(t, err)
	assert.Equal(t, []byte{1, 150}, buf)
	err = protoenc.Unmarshal(buf, &decoded)
	require.NoError(t, err)
	assert.Equal(t, "-150", decoded.Int.String())

	encoded.Int.SetString("238756834756284658865287462349857298752354", 10)
	buf, err = protoenc.Marshal(&encoded)
	require.NoError(t, err)
	assert.Equal(t, []byte{0x0, 0x2, 0xbd, 0xa4, 0xad, 0xbf, 0x98, 0xbd, 0x70, 0x26, 0xbd, 0x3b, 0x65, 0xe8, 0xae, 0xf3, 0xfa, 0xa3, 0x62}, buf)
	err = protoenc.Unmarshal(buf, &decoded)
	require.NoError(t, err)
	assert.Equal(t, "238756834756284658865287462349857298752354", decoded.Int.String())
}

type bigIntWrapper struct {
	Int *big.Int
}

func (w *bigIntWrapper) MarshalBinary() ([]byte, error) {
	sign := []byte{0}
	if w.Int.Cmp(zero) < 0 {
		sign[0] = 1
	}

	return append(sign, w.Int.Bytes()...), nil
}

func (w *bigIntWrapper) UnmarshalBinary(in []byte) error {
	if len(in) < 1 {
		w.Int.SetInt64(0)

		return nil
	}

	w.Int.SetBytes(in[1:])

	if in[0] != 0 {
		w.Int.Mul(w.Int, negone)
	}

	return nil
}

var (
	zero   = new(big.Int)
	negone = new(big.Int).SetInt64(-1)
)

func TestMapVsGeneratedMap(t *testing.T) {
	goMap := map[string]bool{"test": true}

	generatedMap := TestRequest{Something: goMap}

	generatedEncoded, err := generatedMap.MarshalVT()
	assert.NoError(t, err)

	type customType struct {
		Field map[string]bool `protobuf:"1"`
	}

	customMap := &customType{Field: goMap}

	customEncoded, err := protoenc.Marshal(customMap)
	assert.NoError(t, err)

	assert.Equal(t, generatedEncoded, customEncoded)
	t.Log(hex.Dump(generatedEncoded))
	t.Log(hex.Dump(customEncoded))
}

func TestStringKey(t *testing.T) {
	// TODO: test map[string]struct aka map of empty messages
	const (
		k1 = ""
		k2 = "test"
		k3 = "another"
	)

	type customType struct {
		Field map[string]bool `protobuf:"1"`
	}

	customMap := &customType{
		Field: map[string]bool{
			k1: true,
			k2: true,
		},
	}

	customEncoded, err := protoenc.Marshal(customMap)
	require.NoError(t, err)
	t.Log(hex.Dump(customEncoded))
	assert.Equal(t, 16, len(customEncoded))

	var customDecoded customType
	err = protoenc.Unmarshal(customEncoded, &customDecoded)
	require.NoError(t, err)

	assert.True(t, customDecoded.Field[k1])
	assert.True(t, customDecoded.Field[k2])
	assert.False(t, customDecoded.Field[k3])
}

func TestInternalStructMarshal(t *testing.T) {
	encoded := Pair[Sequence[string], int]{
		First:  Sequence[string]{field: "test for tests"},
		Second: 150,
	}

	buf, err := protoenc.Marshal(&encoded)
	require.NoError(t, err)

	var decoded Pair[Sequence[string], int]
	err = protoenc.Unmarshal(buf, &decoded)
	require.NoError(t, err)

	assert.Equal(t, encoded, decoded)
}

func TestInternalStructMarshalSlice(t *testing.T) {
	encoded := Pair[[]Sequence[string], int]{
		First: []Sequence[string]{
			{field: "test for tests"},
			{field: "test for another tests"},
		},
		Second: 153,
	}

	buf, err := protoenc.Marshal(&encoded)
	require.NoError(t, err)

	var decoded Pair[[]Sequence[string], int]
	err = protoenc.Unmarshal(buf, &decoded)
	require.NoError(t, err)

	assert.Equal(t, encoded, decoded)
}

func TestSequence(t *testing.T) {
	encoded := Sequence[string]{field: "test for tests"}

	buf, err := protoenc.Marshal(&encoded)
	require.NoError(t, err)

	var decoded Sequence[string]
	err = protoenc.Unmarshal(buf, &decoded)
	require.NoError(t, err)

	assert.Equal(t, encoded, decoded)
}

type Sequence[T string | []byte] struct {
	field T
}

func (cm *Sequence[T]) MarshalBinary() ([]byte, error) {
	return []byte(cm.field), nil
}

func (cm *Sequence[T]) UnmarshalBinary(data []byte) error {
	cm.field = T(data)

	return nil
}

func TestMarshalEmpty(t *testing.T) {
	type Empty struct{}

	buf := must(protoenc.Marshal(&Empty{}))(t)
	require.Len(t, buf, 0)

	buf = must(protoenc.Marshal(&Value[Empty]{}))(t)
	require.Equal(t, []byte{0x0a, 0x00}, buf)
}

func TestEmbedding(t *testing.T) {
	type EmbeddedStruct struct {
		Value  int    `protobuf:"1"`
		Value2 uint32 `protobuf:"2"`
	}

	type AnotherEmbeddedStruct struct {
		Value1 int    `protobuf:"3"`
		Value2 uint32 `protobuf:"4"`
	}

	structs := map[string]struct {
		fn func(t *testing.T)
	}{
		"should embed struct": {
			fn: testEncodeDecode(struct {
				EmbeddedStruct
			}{
				EmbeddedStruct: EmbeddedStruct{
					Value:  0x11,
					Value2: 0x12,
				},
			}),
		},
		"should embed struct pointer": {
			fn: testEncodeDecode(struct {
				*EmbeddedStruct
			}{
				EmbeddedStruct: &EmbeddedStruct{
					Value:  0x15,
					Value2: 0x16,
				},
			}),
		},
		"should embed nil pointer struct and not nil pointer struct": {
			fn: testEncodeDecode(struct {
				*EmbeddedStruct
				*AnotherEmbeddedStruct
			}{
				EmbeddedStruct: nil,
				AnotherEmbeddedStruct: &AnotherEmbeddedStruct{
					Value1: 0x21,
					Value2: 0x22,
				},
			}),
		},
		"should embed struct with marshaller": {
			fn: testEncodeDecode(struct {
				Sequence[string]
			}{
				Sequence: Sequence[string]{
					"test",
				},
			}),
		},
		"should not embed nil struct pointer": {
			fn: testIncorrectEncode(struct {
				*EmbeddedStruct
			}{
				EmbeddedStruct: nil,
			}),
		},
		"should not embed simple type": {
			fn: testIncorrectEncode(struct {
				int
			}{
				0x11,
			}),
		},
		"should not embed pointer to simple type": {
			fn: testIncorrectEncode(struct {
				*int
			}{
				int: new(int),
			}),
		},
	}

	for name, test := range structs {
		t.Run(name, test.fn)
	}
}

func testEncodeDecode[V any](v V) func(t *testing.T) {
	return func(t *testing.T) {
		t.Helper()
		encoded := must(protoenc.Marshal(&v))(t)

		t.Logf("\n%s", hex.Dump(encoded))

		var result V

		require.NoError(t, protoenc.Unmarshal(encoded, &result))
		require.Equal(t, v, result)
	}
}

func testIncorrectEncode[V any](v V) func(t *testing.T) {
	return func(t *testing.T) {
		t.Helper()

		_, err := protoenc.Marshal(&v)

		require.Error(t, err)
	}
}

func TestCustomEncoders(t *testing.T) {
	tests := map[string]struct {
		fn func(t *testing.T)
	}{
		"should use custom encoder": {
			testCustomEncodersDecoders(
				encodeCustomEncoderStruct,
				decodeCustomEncoderStruct,
				Value[CustomEncoderStruct]{
					V: CustomEncoderStruct{
						Value: 150,
					},
				},
				Value[CustomEncoderStruct]{
					V: CustomEncoderStruct{
						Value: 152,
					},
				},
			),
		},
		"should use custom encoder on pointer": {
			testCustomEncodersDecoders(
				encodeCustomEncoderStructPtr,
				decodeCustomEncoderStructPtr,
				Value[*CustomEncoderStruct]{
					V: &CustomEncoderStruct{
						Value: 150,
					},
				},
				Value[*CustomEncoderStruct]{
					V: &CustomEncoderStruct{
						Value: 156,
					},
				},
			),
		},
		"should use custom encoder on slice": {
			testCustomEncodersDecoders(
				encodeCustomEncoderStruct,
				decodeCustomEncoderStruct,
				Value[[]CustomEncoderStruct]{
					V: []CustomEncoderStruct{
						{Value: 150},
						{Value: 151},
					},
				},
				Value[[]CustomEncoderStruct]{
					V: []CustomEncoderStruct{
						{Value: 152},
						{Value: 153},
					},
				},
			),
		},
	}

	for name, test := range tests {
		t.Run(name, test.fn)
	}
}

type CustomEncoderStruct struct {
	Value int
}

func encodeCustomEncoderStruct(v CustomEncoderStruct) ([]byte, error) {
	return []byte(strconv.Itoa(v.Value + 1)), nil
}

func decodeCustomEncoderStruct(slc []byte) (CustomEncoderStruct, error) {
	res, err := strconv.Atoi(string(slc))
	if err != nil {
		return CustomEncoderStruct{}, err
	}

	return CustomEncoderStruct{
		Value: res + 1,
	}, err
}

func encodeCustomEncoderStructPtr(v *CustomEncoderStruct) ([]byte, error) {
	return []byte(strconv.Itoa(v.Value + 3)), nil
}

func decodeCustomEncoderStructPtr(slc []byte) (*CustomEncoderStruct, error) {
	res, err := strconv.Atoi(string(slc))
	if err != nil {
		return &CustomEncoderStruct{}, err
	}

	return &CustomEncoderStruct{
		Value: res + 3,
	}, err
}

func testCustomEncodersDecoders[V any, T any](
	enc func(T) ([]byte, error),
	dec func([]byte) (T, error),
	original V,
	expected V,
) func(t *testing.T) {
	return func(t *testing.T) {
		t.Cleanup(func() {
			protoenc.CleanEncoderDecoder()
		})

		protoenc.RegisterEncoderDecoder(enc, dec)

		encoded := must(protoenc.Marshal(&original))(t)

		var result V

		require.NoError(t, protoenc.Unmarshal(encoded, &result))
		require.Equal(t, expected, result)
	}
}

func TestIncorrectCustomEncoders(t *testing.T) {
	t.Cleanup(func() {
		protoenc.CleanEncoderDecoder()
	})

	require.Panics(t, func() {
		protoenc.RegisterEncoderDecoder(
			func(v []CustomEncoderStruct) ([]byte, error) { return nil, nil },
			func(slc []byte) ([]CustomEncoderStruct, error) { return nil, nil },
		)
	})

	require.Panics(t, func() {
		protoenc.RegisterEncoderDecoder(
			func(v string) ([]byte, error) { return nil, nil },
			func(slc []byte) (string, error) { return "", nil },
		)
	})
}

func TestMarshalMapInterface(t *testing.T) {
	// json decodes numeric values as float64s
	// json decoder do not support slices
	testEncodeDecode(Value[map[string]interface{}]{
		V: map[string]interface{}{
			"a": 1.0,
			"b": "2",
			"c": true,
			"e": map[string]interface{}{
				"a": 1.0,
				"g": 10.10,
			},
		},
	})(t)
}

func TestMarshalTime(t *testing.T) {
	// json decodes numeric values as float64s
	// json decoder do not support slices
	testEncodeDecode(Value[time.Time]{
		V: time.Date(2019, time.January, 1, 0, 0, 0, 0, time.UTC),
	})(t)
}

func TestResursiveTypes(t *testing.T) {
	t.Run("test recursive struct", func(t *testing.T) {
		type Recursive struct {
			Next  *Recursive `protobuf:"2"`
			Value int        `protobuf:"1"`
		}
		testEncodeDecode(Recursive{
			Value: 1,
			Next: &Recursive{
				Value: 2,
				Next: &Recursive{
					Value: 3,
				},
			},
		})(t)
	})

	t.Run("test recursive struct with slice", func(t *testing.T) {
		type Recursive struct {
			Next  []Recursive `protobuf:"2"`
			Value int         `protobuf:"1"`
		}
		testEncodeDecode(Recursive{
			Value: 1,
			Next: []Recursive{
				{
					Value: 1,
					Next: []Recursive{
						{
							Value: 2,
							Next: []Recursive{
								{
									Value: 3,
								},
							},
						},
						{
							Value: 10,
							Next: []Recursive{
								{
									Value: 11,
								},
							},
						},
					},
				},
			},
		})(t)
	})

	t.Run("test recursive struct with map", func(t *testing.T) {
		type Recursive struct {
			Next  map[string]Recursive `protobuf:"2"`
			Value int                  `protobuf:"1"`
		}
		testEncodeDecode(
			Recursive{
				Value: 1,
				Next: map[string]Recursive{
					"a": {
						Value: 1,
						Next: map[string]Recursive{
							"a": {
								Value: 2,
								Next: map[string]Recursive{
									"a": {
										Value: 3,
									},
								},
							},
							"b": {
								Value: 10,
								Next: map[string]Recursive{
									"a": {
										Value: 11,
									},
								},
							},
						},
					},
				},
			},
		)(t)
	})
}
