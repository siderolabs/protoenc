// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package protoenc_test

import (
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/protoenc"
)

func TestArrayEncodingDecoding(t *testing.T) {
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

	type localMap struct { //nolint:unused
		A map[int]int `protobuf:"1"`
	}

	tests := map[string]struct {
		fn func(t *testing.T)
	}{
		"bool":            {testArray[bool]},
		"string":          {testArray[string]},
		"int":             {testArray[int]},
		"int32":           {testArray[int32]},
		"int64":           {testArray[int64]},
		"uint":            {testArray[uint]},
		"uint32":          {testArray[uint32]},
		"uint64":          {testArray[uint64]},
		"float32":         {testArray[float32]},
		"float64":         {testArray[float64]},
		"array[int64]":    {testArray[array[int64]]},
		"FixedS32":        {testArray[protoenc.FixedS32]},
		"FixedS64":        {testArray[protoenc.FixedS64]},
		"FixedU32":        {testArray[protoenc.FixedU32]},
		"FixedU64":        {testArray[protoenc.FixedU64]},
		"time.Time":       {testArray[time.Time]},
		"Struct":          {testArray[localType]},
		"array[localMap]": {testArray[array[localMap]]},
		"time.Duration":   {testArray[time.Duration]},
	}

	for name, test := range tests {
		t.Run(name, test.fn)
	}
}

func testArray[T any](t *testing.T) {
	t.Parallel()

	// This is our best-effort attempt to generate a random array of values.
	for i := 0; i < 100; i++ {
		original := array[T]{}
		faker := gofakeit.New(Seed + int64(i))

		require.NoError(t, faker.Struct(&original))

		// This is needed because faker cannot fill time.Time in slices.
		if timeSlice, ok := any(&original.Arr).(*[]time.Time); ok {
			for i := range *timeSlice {
				(*timeSlice)[i] = faker.Date()
			}
		}

		buf := must(protoenc.Marshal(&original))(t)
		target := array[T]{}

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

func TestArrayEncodingForm(t *testing.T) {
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

	type localType struct {
		A int `protobuf:"1"`
	}

	tests := []struct { //nolint:govet
		name string
		fn   func(t *testing.T)
	}{
		{
			"ints should be encoded in 'packed' form",
			testArrayEncodingForm([]int{1, 2, 3}, encodedInts),
		},
		{
			"uints should be encoded in 'packed' form",
			testArrayEncodingForm([]uint{1, 2, 3}, encodedInts),
		},
		{
			"float32s should be encoded in 'packed' form",
			testArrayEncodingForm([]float32{1, 2, 3}, encodedFloat32s),
		},
		{
			"FixedU32s should be encoded in 'packed' form",
			testArrayEncodingForm([]protoenc.FixedU32{1, 2, 3}, encodedFixedU32s),
		},
		{
			"strings should be encoded in normal form",
			testArrayEncodingForm([]string{"ab", "bc", "cde"}, encodedStrings),
		},
		{
			"wrapped values should be encoded in normal form",
			testArrayEncodingForm([]intWrapper{{1}, {2}, {3}}, encodedWrappedInts),
		},
		{
			"wrapped values with no marshallers should be encoded in normal form",
			testArrayEncodingForm([]localType{{1}, {2}, {3}}, encodedLocalType),
		},
		{
			"time values should be encoded in normal form",
			testArrayEncodingForm([]time.Time{time.Unix(0, 1), time.Unix(0, 2), time.Unix(0, 3)}, encodedTime),
		},
		{
			"duration values should be encoded in normal form",
			testArrayEncodingForm([]time.Duration{16 * time.Second, 17 * time.Second, 18 * time.Second}, encodedDuration),
		},
		{
			"nil pointers will be skipped, and it should be encoded in normal form",
			testArrayEncodingForm([]*string{nil, ptr("ab"), ptr("bc"), nil, ptr("cde")}, encodedStrings),
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, test.fn)
	}
}

func testArrayEncodingForm[T any](slc []T, expected []byte) func(t *testing.T) {
	return func(t *testing.T) {
		t.Parallel()

		original := array[T]{Arr: slc}
		buf := must(protoenc.Marshal(&original))(t)

		assert.Equal(t, expected, buf)
	}
}

// hexToBytes converts a hex string to a byte slice, removing any whitespace.
func hexToBytes(t *testing.T, s string) []byte {
	t.Helper()

	s = strings.ReplaceAll(s, "|", "")
	s = strings.ReplaceAll(s, "[", "")
	s = strings.ReplaceAll(s, "]", "")
	s = strings.ReplaceAll(s, " ", "")

	b, err := hex.DecodeString(s)
	require.NoError(t, err)

	return b
}

// newArray returns a new array with the given elements.
func newArray[T any](elements ...T) *array[T] {
	return &array[T]{elements}
}

type array[T any] struct {
	Arr []T `protobuf:"1"`
}

func TestDisallowedTypes(t *testing.T) {
	t.Parallel()

	type localMapOfSlices struct { //nolint:unused
		A map[int][]int `protobuf:"1"`
	}

	type myBytes byte //nolint:unused

	tests := map[string]struct {
		fn func(t *testing.T)
	}{
		"array[map[string]string]": {
			fn: testDisallowedTypes[array[map[string]string]],
		},
		"array[localMapOfSlices]": {
			fn: testDisallowedTypes[array[localMapOfSlices]],
		},
		"array[myBytes]": {
			fn: testDisallowedTypes[array[myBytes]],
		},
		"map[string]string": {
			fn: testDisallowedTypes[map[string]string],
		},
		"array[Value[int]]": {
			fn: func(t *testing.T) {
				t.Parallel()

				arr := newArray[Value[int]](&ValueWrapper[int]{1}, &ValueWrapper[int]{1})
				buf, err := protoenc.Marshal(arr)
				require.NoError(t, err)

				var target array[Value[int]]
				err = protoenc.Unmarshal(buf, &target)
				require.Error(t, err)
				assert.Contains(t, err.Error(), "nil interface fields are not supported")
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
	assert.Regexp(t, "(unsupported type)|(map only support)|(takes a struct)", err.Error())
}

func TestDuration(t *testing.T) {
	t.Parallel()

	expected := newArray(time.Second*11, time.Second*12, time.Second*13)
	buf := must(protoenc.Marshal(expected))(t)

	t.Log(hex.Dump(buf))

	var actual array[time.Duration]

	require.NoError(t, protoenc.Unmarshal(buf, &actual))
	assert.Equal(t, expected.Arr, actual.Arr)
}

func TestTime(t *testing.T) {
	t.Parallel()

	expected := newArray(time.Unix(11, 0).UTC(), time.Unix(12, 0).UTC(), time.Unix(13, 0).UTC())
	buf := must(protoenc.Marshal(expected))(t)

	t.Log(hex.Dump(buf))

	var actual array[time.Time]

	require.NoError(t, protoenc.Unmarshal(buf, &actual))
	assert.Equal(t, expected.Arr, actual.Arr)
}

func TestSliceToArray(t *testing.T) {
	t.Parallel()

	expected := newArray(1, 2, 3, 4, 5, 6, 7, 8, 9, 100500)
	buf := must(protoenc.Marshal(expected))(t)

	t.Log(hex.Dump(buf))

	type structWithArray struct {
		Arr [10]int `protobuf:"1"`
	}

	var actual structWithArray

	require.NoError(t, protoenc.Unmarshal(buf, &actual))
	assert.Equal(t, expected.Arr, actual.Arr[:])
}
