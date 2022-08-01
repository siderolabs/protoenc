// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package protoenc_test

import (
	"encoding"
	"encoding/hex"
	"math/big"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protowire"

	"github.com/siderolabs/protoenc"
)

func TestByteOverwrite(t *testing.T) {
	t.Parallel()

	// This test ensures that if we append to a byte slice in buffer it doesn't affect others
	encoded := TwoFields{
		Buf1: []byte("test"),
		Buf2: []byte("end"),
	}
	buf, err := protoenc.Marshal(&encoded)
	require.NoError(t, err)

	var decoded TwoFields
	err = protoenc.Unmarshal(buf, &decoded)
	require.NoError(t, err)

	assert.Equal(t, []byte("test"), decoded.Buf1)
	assert.Equal(t, []byte("end"), decoded.Buf2)
	assert.Equal(t, len(decoded.Buf1), cap(decoded.Buf1))
	assert.Equal(t, len(decoded.Buf2), cap(decoded.Buf2))

	b1 := append(decoded.Buf1, "-lol"...) //nolint:gocritic
	assert.Equal(t, []byte("test-lol"), b1)
	assert.Equal(t, []byte("end"), decoded.Buf2)
}

type TwoFields struct {
	Buf1 []byte `protobuf:"1"`
	Buf2 []byte `protobuf:"2"`
}

func TestBinaryMarshaler(t *testing.T) {
	t.Parallel()

	encoded := StructWithInterface{&intWrapper{val: 150}}
	buf := must(protoenc.Marshal(&encoded))(t)

	decoded := StructWithInterface{&intWrapper{val: 0}}
	require.NoError(t, protoenc.Unmarshal(buf, &decoded))

	require.Equal(t, encoded.Field.Val(), decoded.Field.Val())
}

type StructWithInterface struct {
	Field ValueM[int] `protobuf:"1"`
}

type ValueM[T any] interface {
	encoding.BinaryMarshaler
	encoding.BinaryUnmarshaler

	Val() T
}

type intWrapper struct {
	val int
}

func (i *intWrapper) Val() int {
	return i.val
}

func (i *intWrapper) MarshalBinary() ([]byte, error) {
	return protowire.AppendVarint(nil, uint64(i.val)), nil
}

func (i *intWrapper) UnmarshalBinary(data []byte) error {
	res, _ := protowire.ConsumeVarint(data)
	i.val = int(res)

	return nil
}

func TestNoBinaryMarshaler(t *testing.T) {
	t.Parallel()

	encoded := WrapperNoMarshal[string]{&ValueWrapper[string]{V: "test-string"}}
	buf := must(protoenc.Marshal(&encoded))(t)

	decoded := WrapperNoMarshal[string]{&ValueWrapper[string]{V: ""}}
	require.NoError(t, protoenc.Unmarshal(buf, &decoded))

	require.Equal(t, encoded.Field.Val(), decoded.Field.Val())
}

type WrapperNoMarshal[T any] struct {
	Field Value[T] `protobuf:"1"`
}

type Value[T any] interface {
	Val() T
}

type ValueWrapper[T any] struct {
	V T `protobuf:"1"`
}

func (vw *ValueWrapper[T]) Val() T {
	return vw.V
}

func Test2dSlice(t *testing.T) {
	t.Parallel()

	t.Run("should fail on 2d int slice", func(t *testing.T) {
		encoded := Slice[int]{Values: [][]int{{1, 2, 3}, {4, 5, 6}}}
		_, err := protoenc.Marshal(&encoded)
		require.Error(t, err)
	})

	t.Run("should fail on 2d uint16 slice", func(t *testing.T) {
		encoded := Slice[uint16]{Values: [][]uint16{{1, 2, 3}, {4, 5, 6}}}
		_, err := protoenc.Marshal(&encoded)
		require.Error(t, err)
	})

	t.Run("should ok on 2d byte slice", func(t *testing.T) {
		encoded := Slice[byte]{Values: [][]byte{{1, 2, 3}, {4, 5, 6}}}
		buf := must(protoenc.Marshal(&encoded))(t)

		decoded := Slice[byte]{}
		require.NoError(t, protoenc.Unmarshal(buf, &decoded))

		require.Equal(t, encoded.Values, decoded.Values)
	})
}

type Slice[T any] struct {
	Values [][]T `protobuf:"1"`
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
	encoded := hasInternalCanMarshal[string]{
		Field:  Sequence[string]{field: "test for tests"},
		Field2: 150,
	}

	buf, err := protoenc.Marshal(&encoded)
	require.NoError(t, err)

	var decoded hasInternalCanMarshal[string]
	err = protoenc.Unmarshal(buf, &decoded)
	require.NoError(t, err)

	assert.Equal(t, encoded, decoded)
}

type hasInternalCanMarshal[T string | []byte] struct {
	Field  Sequence[T] `protobuf:"1"`
	Field2 int         `protobuf:"2"`
}

type Sequence[T string | []byte] struct {
	field T
}

//nolint:revive
func (cm *Sequence[T]) MarshalBinary() ([]byte, error) {
	return []byte(cm.field), nil
}

//nolint:revive
func (cm *Sequence[T]) UnmarshalBinary(data []byte) error {
	cm.field = T(data)

	return nil
}

type A struct {
	Value int `protobuf:"1"`
}

func (a *A) MarshalBinary() ([]byte, error) {
	res := protowire.AppendTag(nil, 1, protowire.VarintType)

	return protowire.AppendVarint(res, uint64(a.Value)), nil
}

func (a *A) Print() string {
	return ""
}

type B struct {
	AValue A   `protobuf:"1"`
	AInt   int `protobuf:"2"`
}

func TestMarshal(t *testing.T) {
	a := A{-149}
	b := B{a, 300}

	bufA := must(protoenc.Marshal(&a))(t)
	bufB := must(protoenc.Marshal(&b))(t)

	t.Logf("%s", hex.Dump(bufA))
	t.Logf("%s", hex.Dump(bufB))

	testA := A{}
	testB := B{}

	require.NoError(t, protoenc.Unmarshal(bufA, &testA))
	require.NoError(t, protoenc.Unmarshal(bufB, &testB))

	assert.Equal(t, a, testA)
	assert.Equal(t, b, testB)
}

type EmbeddedStruct struct {
	Value  int    `protobuf:"1"`
	Value2 uint32 `protobuf:"2"`
}

type AnotherEmbeddedStruct struct {
	Value1 int    `protobuf:"3"`
	Value2 uint32 `protobuf:"4"`
}

func TestEmbedding(t *testing.T) {
	structs := map[string]struct {
		fn func(t *testing.T)
	}{
		"should embed struct": {
			fn: makeEmbedTest(struct {
				EmbeddedStruct
			}{
				EmbeddedStruct: EmbeddedStruct{
					Value:  0x11,
					Value2: 0x12,
				},
			}),
		},
		"should embed struct pointer": {
			fn: makeEmbedTest(struct {
				*EmbeddedStruct
			}{
				EmbeddedStruct: &EmbeddedStruct{
					Value:  0x15,
					Value2: 0x16,
				},
			}),
		},
		"should embed nil pointer struct and not nil pointer struct": {
			fn: makeEmbedTest(struct {
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
			fn: makeEmbedTest(struct {
				Sequence[string]
			}{
				Sequence: Sequence[string]{
					"test",
				},
			}),
		},
		"should not embed nil struct pointer": {
			fn: makeIncorrectEmbedTest(struct {
				*EmbeddedStruct
			}{
				EmbeddedStruct: nil,
			}),
		},
		"should not embed simple type": {
			fn: makeIncorrectEmbedTest(struct {
				int
			}{
				0x11,
			}),
		},
		"should not embed pointer to simple type": {
			fn: makeIncorrectEmbedTest(struct {
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

func makeEmbedTest[V any](v V) func(t *testing.T) {
	return func(t *testing.T) {
		t.Helper()
		encoded := must(protoenc.Marshal(&v))(t)

		t.Logf("\n%s", hex.Dump(encoded))

		var result V

		require.NoError(t, protoenc.Unmarshal(encoded, &result))
		require.Equal(t, v, result)
	}
}

func makeIncorrectEmbedTest[V any](v V) func(t *testing.T) {
	return func(t *testing.T) {
		t.Helper()

		_, err := protoenc.Marshal(&v)

		require.Error(t, err)
	}
}

func TestCustomEcnoders(t *testing.T) {
	tests := map[string]struct {
		fn func(t *testing.T)
	}{
		"should use custom encoder": {
			testCustomEncodersDecoders(
				encodeCustomEncoderStruct,
				decodeCustomEncoderStruct,
				OneFieldStruct[CustomEncoderStruct]{
					Field: CustomEncoderStruct{
						Value: 150,
					},
				},
				OneFieldStruct[CustomEncoderStruct]{
					Field: CustomEncoderStruct{
						Value: 152,
					},
				},
			),
		},
		"should use custom encoder on pointer": {
			testCustomEncodersDecoders(
				encodeCustomEncoderStruct,
				decodeCustomEncoderStruct,
				OneFieldStruct[*CustomEncoderStruct]{
					Field: &CustomEncoderStruct{
						Value: 150,
					},
				},
				OneFieldStruct[*CustomEncoderStruct]{
					Field: &CustomEncoderStruct{
						Value: 152,
					},
				},
			),
		},
		"should use custom encoder on slice": {
			testCustomEncodersDecoders(
				encodeCustomEncoderStruct,
				decodeCustomEncoderStruct,
				OneFieldStruct[[]CustomEncoderStruct]{
					Field: []CustomEncoderStruct{
						{Value: 150},
						{Value: 151},
					},
				},
				OneFieldStruct[[]CustomEncoderStruct]{
					Field: []CustomEncoderStruct{
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

type OneFieldStruct[T any] struct {
	Field T `protobuf:"1"`
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
