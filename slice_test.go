// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package protoenc_test

import (
	"encoding/hex"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/protoenc"
)

func TestSliceEncodingDecoding(t *testing.T) {
	t.Parallel()

	//nolint:govet
	type localType struct {
		A float32 `protobuf:"1"`
		B struct {
			C []int `protobuf:"1"`
		} `protobuf:"2"`
		C string                                  `protobuf:"3"`
		D map[protoenc.FixedS32]protoenc.FixedS64 `protobuf:"4"`
		E map[protoenc.FixedU64]protoenc.FixedU32 `protobuf:"5"`
		F map[float64]*struct {
			G int64 `protobuf:"1"`
		} `protobuf:"6"`
	}

	type localMap struct {
		A map[int]int `protobuf:"1"`
	}

	tests := map[string]struct {
		fn func(t *testing.T)
	}{
		"bool":                   {testSlice[bool]},
		"string":                 {testSlice[string]},
		"int":                    {testSlice[int]},
		"int32":                  {testSlice[int32]},
		"int64":                  {testSlice[int64]},
		"uint":                   {testSlice[uint]},
		"uint32":                 {testSlice[uint32]},
		"uint64":                 {testSlice[uint64]},
		"float32":                {testSlice[float32]},
		"float64":                {testSlice[float64]},
		"sliceWrapper[int64]":    {testSlice[sliceWrapper[int64]]},
		"sliceWrapper[[]uint8]":  {testSlice[sliceWrapper[[]uint8]]},
		"sliceWrapper[*[]uint8]": {testSlice[sliceWrapper[*[]uint8]]},
		"FixedS32":               {testSlice[protoenc.FixedS32]},
		"FixedS64":               {testSlice[protoenc.FixedS64]},
		"FixedU32":               {testSlice[protoenc.FixedU32]},
		"FixedU64":               {testSlice[protoenc.FixedU64]},
		"time.Time":              {testSlice[time.Time]},
		"Struct":                 {testSlice[localType]},
		"sliceWrapper[localMap]": {testSlice[sliceWrapper[localMap]]},
		"time.Duration":          {testSlice[time.Duration]},
	}

	for name, test := range tests {
		t.Run(name, test.fn)
	}
}

func testSlice[T any](t *testing.T) {
	t.Parallel()

	// This is our best-effort attempt to generate a random slice of values.
	for i := range 100 {
		original := sliceWrapper[T]{}
		faker := gofakeit.New(Seed + uint64(i))

		require.NoError(t, faker.Struct(&original))

		// This is needed because faker cannot fill time.Time in slices.
		if timeSlice, ok := any(&original.Arr).(*[]time.Time); ok {
			for i := range *timeSlice {
				(*timeSlice)[i] = faker.Date()
			}
		}

		buf := must(protoenc.Marshal(&original))(t)
		target := sliceWrapper[T]{}

		err := protoenc.Unmarshal(buf, &target)
		if err != nil {
			t.Log(original)

			require.FailNow(t, "", "%d iteration: %v", i, err)
		}

		if !assert.Equal(t, original, target) {
			t.Log(hex.Dump(buf))
			t.FailNow()
		}
	}
}

func TestSliceEncodingResult(t *testing.T) {
	t.Parallel()

	encodedInts := hexToBytes(t, "0a 03 01 02 03")
	encodedFixedU32s := hexToBytes(t, "0a 0c 01 00 00 00 02 00 00 00 03 00 00 00")
	encodedFloat32s := hexToBytes(t, "0a 0c 00 00 80 3f 00 00 00 40 00 00 40 40 ")

	// here 0a (which is field=1, type=2 encoded) begin to repeat because of the protbuf specification
	encodedStrings := hexToBytes(t, "[0a 02 [61 62]] [0a 02 [62 63]] [0a 03 [63 64 65]]")
	encodedWrappedInts := hexToBytes(t, "[0a 01 [01]] [0a 01 [02]] [0a 01 [03]]")

	// here 08 is also begin to repeat because we use inner structure
	encodedLocalType := hexToBytes(t, "[0a 02 [08 01]] [0a 02 [08 02]] [0a 02 [08 03]]")

	encodedTime := hexToBytes(t, "[0a 02 [10 01]] [0a 02 [10 02]] [0a 02 [10 03]]")

	encodedDuration := hexToBytes(t, "[0a 02 [08 10]] [0a 02 [08 11]] [0a 02 [08 12]]")

	emptyStructs := hexToBytes(t, "[0a 00] [0a 00] [0a 00]")

	type localType struct {
		A int `protobuf:"1"`
	}

	type localEmptyType struct{}

	tests := []struct { //nolint:govet
		name string
		fn   func(t *testing.T)
	}{
		{
			"ints should be encoded in 'packed' form",
			testSliceEncodingResult([]int{1, 2, 3}, encodedInts),
		},
		{
			"uints should be encoded in 'packed' form",
			testSliceEncodingResult([]uint{1, 2, 3}, encodedInts),
		},
		{
			"float32s should be encoded in 'packed' form",
			testSliceEncodingResult([]float32{1, 2, 3}, encodedFloat32s),
		},
		{
			"FixedU32s should be encoded in 'packed' form",
			testSliceEncodingResult([]protoenc.FixedU32{1, 2, 3}, encodedFixedU32s),
		},
		{
			"strings should be encoded in normal form",
			testSliceEncodingResult([]string{"ab", "bc", "cde"}, encodedStrings),
		},
		{
			"wrapped values should be encoded in normal form",
			testSliceEncodingResult([]IntWrapper{{1}, {2}, {3}}, encodedWrappedInts),
		},
		{
			"wrapped values with no marshallers should be encoded in normal form",
			testSliceEncodingResult([]localType{{1}, {2}, {3}}, encodedLocalType),
		},
		{
			"time values should be encoded in normal form",
			testSliceEncodingResult([]time.Time{time.Unix(0, 1), time.Unix(0, 2), time.Unix(0, 3)}, encodedTime),
		},
		{
			"duration values should be encoded in normal form",
			testSliceEncodingResult([]time.Duration{16 * time.Second, 17 * time.Second, 18 * time.Second}, encodedDuration),
		},
		{
			"nil pointers will be skipped, and it should be encoded in normal form",
			testSliceEncodingResult([]*string{nil, ptr("ab"), ptr("bc"), nil, ptr("cde")}, encodedStrings),
		},
		{
			"empty structs should return only tags",
			testSliceEncodingResult([]localEmptyType{{}, {}, {}}, emptyStructs),
		},
	}

	for _, test := range tests {
		t.Run(test.name, test.fn)
	}
}

func testSliceEncodingResult[T any](slc []T, expected []byte) func(t *testing.T) {
	return func(t *testing.T) {
		t.Parallel()

		original := sliceWrapper[T]{Arr: slc}
		buf := must(protoenc.Marshal(&original))(t)

		assert.Equal(t, expected, buf)
	}
}

func TestSmallIntegers(t *testing.T) {
	t.Parallel()

	encodedBytes := hexToBytes(t, "0A 03 01 FF 03")
	encodedFixed := hexToBytes(t, "0A 04 01 FF 01 03")
	encodedFixedNegative := hexToBytes(t, "0A 0C [01] [FF FF FF FF FF FF FF FF FF 01] [03]") //nolint:dupword
	encodedUint16s := hexToBytes(t, "0A 05 [01] [FF FF 03] [03]")

	type customByte byte

	type customSlice []byte

	type customType struct {
		Int16      int16      `protobuf:"1"`
		Uint16     uint16     `protobuf:"3"`
		Int8       int8       `protobuf:"2"`
		Uint8      uint8      `protobuf:"4"`
		CustomByte customByte `protobuf:"5"`
	}

	encodedCustomType := hexToBytes(t, "0a 20 [08 [FF FF FF FF FF FF FF FF FF 01] 18 [FF FF 03] 10 [FF FF FF FF FF FF FF FF FF 01] 20 [FF 01] 28 [FF 01]]") //nolint:dupword

	tests := []struct { //nolint:govet
		name string
		fn   func(t *testing.T)
	}{
		{
			"array of bytes should be encoded in 'bytes' form",
			testEncodeDecodeWrapped([...]byte{1, 0xFF, 3}, encodedBytes),
		},
		{
			"array of custom byte type should be encoded in 'varint' form",
			testEncodeDecodeWrapped([...]customByte{1, 0xFF, 3}, encodedFixed),
		},
		{
			"slice of custom byte type should be encoded in 'varint' form",
			testEncodeDecodeWrapped([]customByte{1, 0xFF, 3}, encodedFixed),
		},
		{
			"slice of int8 should be encoded in 'varint' form",
			testEncodeDecodeWrapped([]int8{1, -1, 3}, encodedFixedNegative),
		},
		{
			"slice of int16 type should be encoded in 'varint' form",
			testEncodeDecodeWrapped([]int16{1, -1, 3}, encodedFixedNegative),
		},
		{
			"slice of uint16 type should be encoded in 'varint' form",
			testEncodeDecodeWrapped([]uint16{1, 0xFFFF, 3}, encodedUint16s),
		},
		{
			"customSlice should be encoded in 'bytes' form",
			testEncodeDecodeWrapped(customSlice{1, 0xFF, 3}, encodedBytes),
		},
		{
			"customType should be encoded in 'varint' form",
			testEncodeDecodeWrapped(customType{
				Int16:      -1,
				Uint16:     0xFFFF,
				Int8:       -1,
				Uint8:      0xFF,
				CustomByte: 0xFF,
			}, encodedCustomType),
		},
	}

	for _, test := range tests {
		t.Run(test.name, test.fn)
	}
}

func testEncodeDecodeWrapped[T any](slc T, expected []byte) func(t *testing.T) {
	return func(t *testing.T) {
		t.Helper()
		t.Parallel()

		original := Value[T]{V: slc}
		buf := must(protoenc.Marshal(&original))(t)

		require.Equal(t, expected, buf)

		var decoded Value[T]

		require.NoError(t, protoenc.Unmarshal(buf, &decoded))
		require.Equal(t, original, decoded)
	}
}

// newSliceWrapper returns a new wrapper type around slice field with the given elements.
func newSliceWrapper[T any](elements ...T) *sliceWrapper[T] {
	return &sliceWrapper[T]{elements}
}

type sliceWrapper[T any] struct {
	Arr []T `protobuf:"1"`
}

type Valuer[T any] interface {
	Val() T
}

type Value[T any] struct {
	V T `protobuf:"1"`
}

func makeValue[T any](t T) Value[T] {
	return Value[T]{V: t}
}

func (v Value[T]) Val() T {
	return v.V
}

func TestDisallowedTypes(t *testing.T) {
	t.Parallel()

	type localMapOfSlices struct {
		A map[int][]int `protobuf:"1"`
	}

	type complexKey struct {
		A int `protobuf:"1"`
	}

	type localMapWithComplexKey struct {
		A map[complexKey]int `protobuf:"1"`
	}

	type localMapWithPtrKey struct {
		A map[*int]int `protobuf:"1"`
	}

	tests := map[string]struct {
		fn func(t *testing.T)
	}{
		"sliceWrapper[map[string]string]": {
			fn: testDisallowedTypes[sliceWrapper[map[string]string]],
		},
		"sliceWrapper[localMapOfSlices]": {
			fn: testDisallowedTypes[sliceWrapper[localMapOfSlices]],
		},
		"sliceWrapper[localMapWithComplexKey]": {
			fn: testDisallowedTypes[sliceWrapper[localMapWithComplexKey]],
		},
		"sliceWrapper[localMapWithPtrKey]": {
			fn: testDisallowedTypes[sliceWrapper[localMapWithPtrKey]],
		},
		"map[string]string": {
			fn: testDisallowedTypes[map[string]string],
		},
		"[]map[string]string": {
			fn: testDisallowedTypes[[]map[string]string],
		},
		"sliceWrapper[*int]": {
			fn: testDisallowedTypes[sliceWrapper[*int]],
		},
		"sliceWrapper[*int8]": {
			fn: testDisallowedTypes[sliceWrapper[*int8]],
		},
		"sliceWrapper[*[]int8]": {
			fn: testDisallowedTypes[sliceWrapper[*[]int8]],
		},
		"sliceWrapper[*[]int16]": {
			fn: testDisallowedTypes[sliceWrapper[*[]int16]],
		},
		"sliceWrapper[*[]int]": {
			fn: testDisallowedTypes[sliceWrapper[*[]int]],
		},
		"sliceWrapper[[][]int]": {
			fn: testDisallowedTypes[sliceWrapper[[][]int]],
		},
		"sliceWrapper[*[][]int]": {
			fn: testDisallowedTypes[sliceWrapper[*[][]int]],
		},
		"sliceWrapper[Valuer[int]]": {
			fn: func(t *testing.T) {
				t.Parallel()

				arr := newSliceWrapper[Valuer[int]](&Value[int]{1}, &Value[int]{1})
				_, err := protoenc.Marshal(arr)
				require.Error(t, err)
			},
		},
	}

	for name, test := range tests {
		t.Run(name, test.fn)
	}
}

func testDisallowedTypes[T any](t *testing.T) {
	t.Parallel()

	faker := gofakeit.New(Seed)

	var original T

	require.NoError(t, faker.Struct(&original))

	_, err := protoenc.Marshal(&original)
	require.Error(t, err)
	assert.Regexp(t, "(is not supported)|(takes a pointer to struct)", err.Error())
}

func TestDuration(t *testing.T) {
	t.Parallel()

	expected := newSliceWrapper(time.Second*11, time.Second*12, time.Second*13)
	buf := must(protoenc.Marshal(expected))(t)

	t.Log(hex.Dump(buf))

	var actual sliceWrapper[time.Duration]

	require.NoError(t, protoenc.Unmarshal(buf, &actual))
	assert.Equal(t, expected.Arr, actual.Arr)
}

func TestTime(t *testing.T) {
	t.Parallel()

	expected := newSliceWrapper(time.Unix(11, 0).UTC(), time.Unix(12, 0).UTC(), time.Unix(13, 0).UTC())
	buf := must(protoenc.Marshal(expected))(t)

	t.Log(hex.Dump(buf))

	var actual sliceWrapper[time.Time]

	require.NoError(t, protoenc.Unmarshal(buf, &actual))
	assert.Equal(t, expected.Arr, actual.Arr)
}

func TestSlicesOfEmpty(t *testing.T) {
	type Empty struct{}

	type NotEmpty struct {
		Field int `protobuf:"1"`
	}

	t.Run("not empty to empty", func(t *testing.T) {
		wrapper := newSliceWrapper(NotEmpty{Field: 1}, NotEmpty{Field: 1}, NotEmpty{Field: 1})
		buf := must(protoenc.Marshal(wrapper))(t)

		result := sliceWrapper[Empty]{}

		require.NoError(t, protoenc.Unmarshal(buf, &result))
		require.Len(t, result.Arr, 3)
	})

	t.Run("empty to not empty", func(t *testing.T) {
		wrapper := newSliceWrapper(Empty{}, Empty{}, Empty{})
		buf := must(protoenc.Marshal(wrapper))(t)

		result := sliceWrapper[NotEmpty]{}

		require.NoError(t, protoenc.Unmarshal(buf, &result))
		require.Len(t, result.Arr, 3)
	})
}
