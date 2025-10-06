// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package protoenc provides a way to marshal and unmarshal Go structs tp protocol buffers.
package protoenc

import (
	"encoding"
	"errors"
	"fmt"
	"math"
	"reflect"
	"time"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// MarshalOptions configures the behavior of Marshal.
type MarshalOptions struct {
	MarshalZeroFields bool // if true, marshal fields with zero values
}

// MarshalOption is a function that configures MarshalOptions.
type MarshalOption func(*MarshalOptions)

// WithMarshalZeroFields configures Marshal to marshal fields with zero values.
func WithMarshalZeroFields() MarshalOption {
	return func(o *MarshalOptions) {
		o.MarshalZeroFields = true
	}
}

// Marshal a Go struct into protocol buffer format.
// The caller must pass a pointer to the struct to encode.
func Marshal(structPtr interface{}, opts ...MarshalOption) (result []byte, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			e, ok := recovered.(error)
			if !ok {
				err = fmt.Errorf("%v", recovered)
			} else {
				err = e
			}

			result = nil
		}
	}()

	if structPtr == nil {
		return nil, nil
	}

	if hasCustomEncoders(reflect.TypeOf(structPtr)) {
		return nil, errors.New("custom encoders are not supported for top-level structs, use BinaryMarshaler instead")
	}

	if bu, ok := structPtr.(encoding.BinaryMarshaler); ok {
		return bu.MarshalBinary()
	}

	var o MarshalOptions

	for _, opt := range opts {
		opt(&o)
	}

	m := marshaller{
		buf:  make([]byte, 0, 32),
		opts: o,
	}

	val := reflect.ValueOf(structPtr)
	if val.Kind() != reflect.Pointer || val.Type().Elem().Kind() != reflect.Struct {
		return nil, errors.New("marshal takes a pointer to struct")
	}

	m.encodeStruct(val.Elem())

	return m.Bytes(), nil
}

type marshaller struct {
	buf  []byte
	opts MarshalOptions
}

func (m *marshaller) Bytes() []byte {
	if len(m.buf) == 0 {
		return nil
	}

	return m.buf
}

func (m *marshaller) encodeStruct(val reflect.Value) {
	structFields, err := StructFields(val.Type())
	if err != nil {
		panic(err)
	}

	if len(structFields) == 0 {
		return
	}

	m.encodeFields(val, structFields)
}

func (m *marshaller) encodeFields(val reflect.Value, fieldsData []FieldData) {
	var fieldData FieldData

	defer func() {
		if r := recover(); r != nil {
			if !fieldData.IsZero() {
				panic(fmt.Errorf("%s (field %s)", r, fieldData.Field.Name))
			}

			panic(r)
		}
	}()

	noneEncoded := true

	for _, fieldData = range fieldsData {
		field := fieldByIndex(val, fieldData)

		if field.IsValid() {
			if m.opts.MarshalZeroFields || !field.IsZero() {
				m.encodeValue(fieldData.Num, field)
			}

			noneEncoded = false
		}
	}

	if noneEncoded {
		panic(fmt.Errorf("struct '%s' has no marshallable fields", val.Type().Name()))
	}
}

// fieldByIndex returns the field of the struct by its index if the field is exported.
// Otherwise, it returns empty reflect.Value.
func fieldByIndex(structVal reflect.Value, data FieldData) reflect.Value {
	if !structVal.IsValid() || !data.Field.IsExported() || len(data.FieldIndex) == 0 {
		return reflect.Value{}
	}

	var result reflect.Value

	for i := range len(data.FieldIndex) {
		index := data.FieldIndex[:i+1]

		result = structVal.FieldByIndex(index)
		if len(data.FieldIndex) > 1 && result.Kind() == reflect.Pointer && result.IsNil() {
			// Embedded field is nil, return empty reflect.Value. Avo
			return reflect.Value{}
		}
	}

	return result
}

//nolint:cyclop,gocyclo
func (m *marshaller) encodeValue(num protowire.Number, val reflect.Value) {
	if m.tryEncodePredefined(num, val) {
		return
	}

	switch val.Kind() { //nolint:exhaustive
	case reflect.Bool:
		putTag(m, num, protowire.VarintType)
		putBool(m, val.Bool())

	case reflect.Int8, reflect.Int16:
		putTag(m, num, protowire.VarintType)
		putUVarint(m, val.Int())

	case reflect.Uint8, reflect.Uint16:
		putTag(m, num, protowire.VarintType)
		putUVarint(m, val.Uint())

	case reflect.Int, reflect.Int32, reflect.Int64:
		putTag(m, num, protowire.VarintType)
		putUVarint(m, val.Int())

	case reflect.Uint, reflect.Uint32, reflect.Uint64:
		putTag(m, num, protowire.VarintType)
		putUVarint(m, val.Uint())

	case reflect.Float32:
		putTag(m, num, protowire.Fixed32Type)
		putInt32(m, math.Float32bits(float32(val.Float())))

	case reflect.Float64:
		putTag(m, num, protowire.Fixed64Type)
		putInt64(m, math.Float64bits(val.Float()))

	case reflect.String:
		putTag(m, num, protowire.BytesType)
		putString(m, val.String())

	case reflect.Struct:
		var b []byte
		if result, ok := tryEncodeFunc(val); ok {
			b = result
		} else if bmarshaler, ok := asBinaryMarshaler(val); ok {
			result, err := bmarshaler.MarshalBinary()
			if err != nil {
				panic(err)
			}

			b = result
		} else {
			inner := marshaller{
				opts: m.opts,
			}
			inner.encodeStruct(val)
			b = inner.Bytes()
		}

		putTag(m, num, protowire.BytesType)
		putBytes(m, b)
	case reflect.Slice, reflect.Array:
		if val.Len() == 0 {
			return
		}

		m.encodeSlice(num, val)

		return

	case reflect.Pointer:
		if val.IsNil() {
			return
		}

		// If the pointer is to a struct
		if indirect(val.Type()).Kind() == reflect.Struct {
			b, ok := tryEncodeFunc(val)
			if ok {
				putTag(m, num, protowire.BytesType)
				putBytes(m, b)

				return
			}
		}

		m.encodeValue(num, val.Elem())

	case reflect.Map:
		m.encodeMap(num, val)

		return

	default:
		panic(fmt.Sprintf("unsupported field Kind %d", val.Kind()))
	}
}

func (m *marshaller) tryEncodePredefined(num protowire.Number, val reflect.Value) bool {
	switch val.Type() {
	case typeMapInterface:
		v := val.Interface().(map[string]interface{}) //nolint:errcheck,forcetypeassert

		val, err := structpb.NewStruct(v)
		if err != nil {
			panic(fmt.Errorf("failed to create structpb.Struct: %w", err))
		}

		encoded, err := proto.Marshal(val)
		if err != nil {
			panic(err)
		}

		putTag(m, num, protowire.BytesType)
		putBytes(m, encoded)

	case typeDuration:
		d := val.Interface().(time.Duration) //nolint:errcheck,forcetypeassert
		duration := durationpb.New(d)

		encoded, err := proto.Marshal(duration)
		if err != nil {
			panic(err)
		}

		putTag(m, num, protowire.BytesType)
		putBytes(m, encoded)

	case typeTime:
		t := val.Interface().(time.Time) //nolint:errcheck,forcetypeassert
		timestamp := timestamppb.New(t)

		encoded, err := proto.Marshal(timestamp)
		if err != nil {
			panic(err)
		}

		putTag(m, num, protowire.BytesType)
		putBytes(m, encoded)

	case typeFixedS32:
		putTag(m, num, protowire.Fixed32Type)
		putInt32(m, int32(val.Int()))

	case typeFixedS64:
		putTag(m, num, protowire.Fixed64Type)
		putInt64(m, val.Int())

	case typeFixedU32:
		putTag(m, num, protowire.Fixed32Type)
		putInt32(m, uint32(val.Uint()))

	case typeFixedU64:
		putTag(m, num, protowire.Fixed64Type)
		putInt64(m, val.Uint())

	default:
		return false
	}

	return true
}

func tryEncodeFunc(val reflect.Value) ([]byte, bool) {
	typ := val.Type()

	enc, ok := encoders.Get(typ)
	if !ok {
		return nil, false
	}

	b, err := enc(val.Interface())
	if err != nil {
		panic(err)
	}

	return b, true
}

func asBinaryMarshaler(val reflect.Value) (encoding.BinaryMarshaler, bool) {
	if enc, ok := val.Interface().(encoding.BinaryMarshaler); ok {
		return enc, true
	}

	if val.CanAddr() {
		if enc, ok := val.Addr().Interface().(encoding.BinaryMarshaler); ok {
			return enc, true
		}
	}

	return nil, false
}

func (m *marshaller) encodeSlice(key protowire.Number, val reflect.Value) {
	sliceLen := val.Len()
	result := marshaller{
		opts: m.opts,
	}

	typ := val.Type()
	if typ.Elem() == typeByte {
		// Special case for byte arrays and slices.
		putTag(m, key, protowire.BytesType)
		putBytes(m, val.Bytes())

		return
	}

	switch typ {
	case typeDurations:
		// Special case for []time.Duration.
		slice := val.Interface().([]time.Duration) //nolint:errcheck,forcetypeassert
		for _, d := range slice {
			duration := durationpb.New(d)

			encoded, err := proto.Marshal(duration)
			if err != nil {
				panic(err)
			}

			putTag(m, key, protowire.BytesType)
			putBytes(m, encoded)
		}

		return

	case typeFixedS64s:
		slice := val.Interface().([]FixedS64) //nolint:errcheck,forcetypeassert
		for i := range sliceLen {
			putInt64(&result, slice[i])
		}
	case typeFixedS32s:
		slice := val.Interface().([]FixedS32) //nolint:errcheck,forcetypeassert
		for i := range sliceLen {
			putInt32(&result, slice[i])
		}
	case typeFixedU64s:
		slice := val.Interface().([]FixedU64) //nolint:errcheck,forcetypeassert
		for i := range sliceLen {
			putInt64(&result, slice[i])
		}
	case typeFixedU32s:
		slice := val.Interface().([]FixedU32) //nolint:errcheck,forcetypeassert
		for i := range sliceLen {
			putInt32(&result, slice[i])
		}
	default:
		// None of predefined types worked, so we need to do it manually.
		m.sliceReflect(key, val)

		return
	}

	putTag(m, key, protowire.BytesType)
	putBytes(m, result.Bytes())
}

func (m *marshaller) sliceReflect(key protowire.Number, val reflect.Value) {
	sliceLen := val.Len()
	elem := val.Type().Elem()
	result := marshaller{
		opts: m.opts,
	}

	switch elem.Kind() { //nolint:exhaustive
	case reflect.Int8, reflect.Int16:
		for i := range sliceLen {
			putUVarint(&result, val.Index(i).Int())
		}

	case reflect.Uint8, reflect.Uint16:
		for i := range sliceLen {
			putUVarint(&result, val.Index(i).Uint())
		}

	case reflect.Bool:
		for i := range sliceLen {
			putBool(&result, val.Index(i).Bool())
		}

	case reflect.Int, reflect.Int32, reflect.Int64:
		for i := range sliceLen {
			putUVarint(&result, val.Index(i).Int())
		}

	case reflect.Uint, reflect.Uint32, reflect.Uint64:
		for i := range sliceLen {
			putUVarint(&result, val.Index(i).Uint())
		}

	case reflect.Float32:
		for i := range sliceLen {
			putInt32(&result, math.Float32bits(float32(val.Index(i).Float())))
		}

	case reflect.Float64:
		for i := range sliceLen {
			putInt64(&result, math.Float64bits(val.Index(i).Float()))
		}

	case reflect.Pointer:
		for i := range sliceLen {
			m.encodeValue(key, val.Index(i))
		}

		return

	case reflect.Map:
		panic(fmt.Errorf("unsupported type %s", val.Type().String()))

	default: // Write each element as a separate key,value pair
		for i := range sliceLen {
			m.encodeValue(key, val.Index(i))
		}

		return
	}

	putTag(m, key, protowire.BytesType)
	putBytes(m, result.buf)
}

func (m *marshaller) encodeMap(key protowire.Number, mpval reflect.Value) {
	for _, mkey := range mpval.MapKeys() {
		mval := mpval.MapIndex(mkey)

		if kind := mval.Kind(); kind == reflect.Pointer && mval.IsNil() {
			panic("error: map has nil element")
		}

		inner := marshaller{
			opts: m.opts,
		}
		inner.encodeValue(1, mkey)
		inner.encodeValue(2, mval)

		putTag(m, key, protowire.BytesType)
		putBytes(m, inner.buf)
	}
}

func putTag(m *marshaller, num protowire.Number, typ protowire.Type) {
	m.buf = protowire.AppendTag(m.buf, num, typ)
}

func putInt32[T ~int32 | ~uint32](m *marshaller, val T) {
	m.buf = protowire.AppendFixed32(m.buf, uint32(val))
}

func putInt64[T ~int64 | ~uint64](m *marshaller, val T) {
	m.buf = protowire.AppendFixed64(m.buf, uint64(val))
}

func putUVarint[T ~int64 | ~uint64](m *marshaller, val T) {
	m.buf = protowire.AppendVarint(m.buf, uint64(val))
}

func putBool(m *marshaller, b bool) {
	putUVarint(m, protowire.EncodeBool(b))
}

func putString(m *marshaller, s string) {
	m.buf = protowire.AppendString(m.buf, s)
}

func putBytes(m *marshaller, b []byte) {
	m.buf = protowire.AppendBytes(m.buf, b)
}
