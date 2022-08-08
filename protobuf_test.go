// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package protoenc_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/protoenc"
)

//nolint:govet
type emb struct {
	I32 int32  `protobuf:"1"`
	S   string `protobuf:"2"`
}

// test custom type-aliases.
type (
	mybool    bool
	myint32   int32
	myint64   int64
	myuint32  uint32
	myuint64  uint64
	myfloat32 float32
	myfloat64 float64
	mystring  string
)

//nolint:govet
type test struct {
	Bool   bool              `protobuf:"1"`
	I      int               `protobuf:"2"`
	I32    int32             `protobuf:"3"`
	I64    int64             `protobuf:"4"`
	U32    uint32            `protobuf:"5"`
	U64    uint64            `protobuf:"7"`
	SX32   protoenc.FixedS32 `protobuf:"8"`
	SX64   protoenc.FixedS64 `protobuf:"9"`
	UX32   protoenc.FixedU32 `protobuf:"10"`
	UX64   protoenc.FixedU64 `protobuf:"11"`
	F32    float32           `protobuf:"12"`
	F64    float64           `protobuf:"13"`
	Bytes  []byte            `protobuf:"14"`
	String string            `protobuf:"16"`
	Struct emb               `protobuf:"17"`

	OBool   *mybool    `protobuf:"50"`
	OI32    *myint32   `protobuf:"51"`
	OI64    *myint64   `protobuf:"52"`
	OU32    *myuint32  `protobuf:"53"`
	OU64    *myuint64  `protobuf:"54"`
	OF32    *myfloat32 `protobuf:"55"`
	OF64    *myfloat64 `protobuf:"56"`
	OString *mystring  `protobuf:"58"`
	OStruct *test      `protobuf:"59"`

	SBool   []mybool            `protobuf:"100"`
	SI32    []myint32           `protobuf:"101"`
	SI64    []myint64           `protobuf:"102"`
	SU32    []myuint32          `protobuf:"103"`
	SU64    []myuint64          `protobuf:"104"`
	SSX32   []protoenc.FixedS32 `protobuf:"105"`
	SSX64   []protoenc.FixedS64 `protobuf:"106"`
	SUX32   []protoenc.FixedU32 `protobuf:"107"`
	SUX64   []protoenc.FixedU64 `protobuf:"108"`
	SF32    []myfloat32         `protobuf:"109"`
	SF64    []myfloat64         `protobuf:"110"`
	SString []mystring          `protobuf:"112"`
	SStruct []emb               `protobuf:"113"`
}

func TestProtobuf(t *testing.T) {
	b0 := mybool(true)
	i1 := myint32(-1)
	i2 := myint64(-2)
	i3 := myuint32(3)
	i4 := myuint64(4)
	f5 := myfloat32(5.5)
	f6 := myfloat64(6.6)
	s8 := mystring("ABC")
	e9 := test{Bytes: nil}

	t1 := test{
		true,
		0,
		-1,
		-2,
		3,
		4,
		-11,
		-22,
		33,
		44,
		5.0,
		6.0,
		[]byte("789"),
		"abc",
		emb{123, "def"},
		&b0,
		&i1,
		&i2,
		&i3,
		&i4,
		&f5,
		&f6,
		&s8,
		&e9,
		[]mybool{true, false, true},
		[]myint32{1, -2, 3},
		[]myint64{2, -3, 4},
		[]myuint32{3, 4, 5},
		[]myuint64{4, 5, 6},
		[]protoenc.FixedS32{11, -22, 33},
		[]protoenc.FixedS64{22, -33, 44},
		[]protoenc.FixedU32{33, 44, 55},
		[]protoenc.FixedU64{44, 55, 66},
		[]myfloat32{5.5, 6.6, 7.7},
		[]myfloat64{6.6, 7.7, 8.8},
		[]mystring{"the", "quick", "brown", "fox"},
		[]emb{{-1, "a"}, {-2, "b"}, {-3, "c"}},
	}
	buf, err := protoenc.Marshal(&t1)
	assert.NoError(t, err)

	t2 := test{}
	err = protoenc.Unmarshal(buf, &t2)
	assert.NoError(t, err)

	if result := cmp.Diff(t1, t2); result != "" {
		t.Fatal(result)
	}
}

//nolint:govet
type simpleFilledInput struct {
	I   string  `protobuf:"1"`
	Ptr *mybool `protobuf:"2"`
}

func TestProtobufFilledInput(t *testing.T) {
	b0 := mybool(true)
	b1 := mybool(false)

	t1 := simpleFilledInput{
		"intermediate value",
		&b0,
	}
	buf, err := protoenc.Marshal(&t1)
	assert.NoError(t, err)

	t2 := simpleFilledInput{
		"intermediate value",
		&b1,
	}
	err = protoenc.Unmarshal(buf, &t2)
	assert.NoError(t, err)
	assert.Equal(t, t1, t2)

	t1 = simpleFilledInput{}
	buf, err = protoenc.Marshal(&t1)
	assert.NoError(t, err)

	err = protoenc.Unmarshal(buf, &t2)
	assert.NoError(t, err)
	assert.Equal(t, t1, t2)
}

type padded struct {
	Field1 int32    `protobuf:"1"`
	_      struct{} `protobuf:"2"`
	Field3 int32    `protobuf:"3"`
	_      int      `protobuf:"4"`
	Field5 int32    `protobuf:"5"`
}

func TestPadded(t *testing.T) {
	t1 := padded{}
	t1.Field1 = 10
	t1.Field3 = 30
	t1.Field5 = 50
	buf, err := protoenc.Marshal(&t1)
	assert.NoError(t, err)

	t2 := padded{}
	if err = protoenc.Unmarshal(buf, &t2); err != nil {
		panic(err.Error())
	}

	if t1 != t2 {
		panic("decode didn't reproduce identical struct")
	}
}

type TimeTypes struct {
	Time     time.Time     `protobuf:"1"`
	Duration time.Duration `protobuf:"2"`
}

const shortForm = "2006-Jan-02"

func TestTimeTypesEncodeDecode(t *testing.T) {
	tt := must(time.Parse(shortForm, "2013-Feb-03"))(t)
	in := &TimeTypes{
		Time:     tt,
		Duration: time.Second * 30,
	}
	buf, err := protoenc.Marshal(in)
	assert.NoError(t, err)

	out := &TimeTypes{}

	err = protoenc.Unmarshal(buf, out)
	assert.NoError(t, err)
	assert.Equal(t, in.Time.UnixNano(), out.Time.UnixNano())
	assert.Equal(t, in.Duration, out.Duration)
}

func TestMapSliceStruct(t *testing.T) {
	t.Parallel()

	type structData struct {
		A int32 `protobuf:"1"`
		B int32 `protobuf:"2"`
	}

	t.Run("test map with slices", func(t *testing.T) {
		t.Parallel()

		type wrongMap struct {
			M map[uint32][]structData `protobuf:"1"`
		}

		cv := []structData{{}, {}}
		msg := &wrongMap{
			M: map[uint32][]structData{1: cv},
		}

		_, err := protoenc.Marshal(msg)
		assert.Error(t, err)
	})

	t.Run("test wrong map ptr to slice", func(t *testing.T) {
		t.Parallel()

		type wrongMap struct {
			M map[uint32]*[]structData `protobuf:"1"`
		}

		ptrCv := &[]structData{{}, {}}
		msg := &wrongMap{
			M: map[uint32]*[]structData{1: ptrCv},
		}

		_, err := protoenc.Marshal(msg)
		assert.Error(t, err)
	})

	t.Run("test right map with ptr to struct", func(t *testing.T) {
		t.Parallel()

		type rightMap struct {
			M map[uint32]*structData `protobuf:"1"`
		}

		msg := &rightMap{
			M: map[uint32]*structData{1: {4, 5}},
		}

		buff, err := protoenc.Marshal(msg)
		assert.NoError(t, err)

		dec := &rightMap{}
		err = protoenc.Unmarshal(buff, dec)
		assert.NoError(t, err)

		assert.True(t, reflect.DeepEqual(dec, msg))
	})

	t.Run("test right map with empty struct", func(t *testing.T) {
		t.Parallel()

		type rightMap struct {
			M map[uint32]struct{} `protobuf:"1"`
		}

		msg := &rightMap{
			M: map[uint32]struct{}{1: {}},
		}

		buff, err := protoenc.Marshal(msg)
		assert.NoError(t, err)

		dec := &rightMap{}
		err = protoenc.Unmarshal(buff, dec)
		assert.NoError(t, err)

		assert.True(t, reflect.DeepEqual(dec, msg))
	})
}
