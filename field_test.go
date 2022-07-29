// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package protoenc_test

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/protoenc"
)

func TestEncodeNested(t *testing.T) {
	t.Parallel()

	s := &StructWithEmbed{
		A: 11,
		EmbedStruct: &EmbedStruct{
			A: 111,
			B: 112,
			C: 113,
		},
	}

	val := reflect.ValueOf(s)

	actual := must(protoenc.StructFields(val.Type()))(t)
	for _, f := range actual {
		f.Field = reflect.StructField{}
	}

	expected := []*protoenc.FieldData{
		{1, []int{0}, reflect.StructField{}},
		{9, []int{1, 0}, reflect.StructField{}},
		{10, []int{1, 1}, reflect.StructField{}},
		{11, []int{1, 2}, reflect.StructField{}},
	}

	assert.Equal(t, len(expected), len(actual))

	for i := range expected {
		assert.Equal(t, expected[i].Num, actual[i].Num)
		assert.Equal(t, expected[i].FieldIndex, actual[i].FieldIndex)
	}

	expectedValues := []int64{11, 111, 112, 113}
	for i := range expectedValues {
		assert.Equal(t, expectedValues[i], val.Elem().FieldByIndex(actual[i].FieldIndex).Int())
	}
}

//nolint:govet
type StructWithEmbed struct {
	A int32 `protobuf:"1"`
	*EmbedStruct
}

type EmbedStruct struct {
	A int32 `protobuf:"9"`
	B int32 `protobuf:"10"`
	C int32 `protobuf:"11"`
}

func TestDuplicateIDNotAllowed(t *testing.T) {
	t.Parallel()

	v := reflect.TypeOf(&StructWithDuplicates{})
	_, err := protoenc.StructFields(v)
	require.Error(t, err)
	assert.Contains(t, err.Error(), " duplicated in ")
}

type StructWithDuplicates struct {
	Field1 int32 `protobuf:"1"`
	Field2 int32 `protobuf:"2"`
	Field3 int32 `protobuf:"1"`
}
