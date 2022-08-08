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
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Marshal a Go struct into protocol buffer format.
// The caller must pass a pointer to the struct to encode.
func Marshal(ptr interface{}) (result []byte, err error) {
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

	if ptr == nil {
		return nil, nil
	}

	if bu, ok := ptr.(encoding.BinaryMarshaler); ok {
		return bu.MarshalBinary()
	}

	m := marshaller{
		buf: make([]byte, 0, 32),
	}

	val := reflect.ValueOf(ptr)
	if val.Kind() != reflect.Pointer {
		return nil, errors.New("encode takes a pointer to struct")
	}

	m.encodeStruct(val.Elem())

	return m.Bytes(), nil
}

type marshaller struct {
	buf []byte
}

func (m *marshaller) Bytes() []byte {
	if len(m.buf) == 0 {
		return nil
	}

	return m.buf
}

func (m *marshaller) encodeStruct(val reflect.Value) {
	if val.Type().Kind() != reflect.Struct {
		panic("encodeStruct takes a struct")
	}

	res, ok := tryEncodeFunc(val)
	if ok {
		m.buf = append(m.buf, res...)

		return
	}

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
			} else {
				panic(r)
			}
		}
	}()

	noneEncoded := true

	for _, fieldData = range fieldsData {
		field := fieldByIndex(val, fieldData)

		if field.IsValid() {
			m.encodeValue(fieldData.Num, field)

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

	for i := 0; i < len(data.FieldIndex); i++ {
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
		putTag(m, num, protowire.Fixed32Type)
		putInt32(m, int32(val.Int()))

	case reflect.Uint8, reflect.Uint16:
		putTag(m, num, protowire.Fixed32Type)
		putInt32(m, int32(val.Uint()))

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

		bmarshaler, ok := asBinaryMarshaler(val)
		if ok {
			var err error

			b, err = bmarshaler.MarshalBinary()
			if err != nil {
				panic(err)
			}
		} else {
			inner := marshaller{}
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

		m.encodeValue(num, val.Elem())

	case reflect.Interface:
		// Abstract interface field.
		if val.IsNil() {
			return
		}

		// If the object support self-encoding, use that.
		if enc, ok := val.Interface().(encoding.BinaryMarshaler); ok {
			putTag(m, num, protowire.BytesType)

			bytes, err := enc.MarshalBinary()
			if err != nil {
				panic(err)
			}

			putBytes(m, bytes)

			return
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
	result := marshaller{}

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
		for i := 0; i < sliceLen; i++ {
			putInt64(&result, slice[i])
		}
	case typeFixedS32s:
		slice := val.Interface().([]FixedS32) //nolint:errcheck,forcetypeassert
		for i := 0; i < sliceLen; i++ {
			putInt32(&result, slice[i])
		}
	case typeFixedU64s:
		slice := val.Interface().([]FixedU64) //nolint:errcheck,forcetypeassert
		for i := 0; i < sliceLen; i++ {
			putInt64(&result, slice[i])
		}
	case typeFixedU32s:
		slice := val.Interface().([]FixedU32) //nolint:errcheck,forcetypeassert
		for i := 0; i < sliceLen; i++ {
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

//nolint:gocyclo,cyclop
func (m *marshaller) sliceReflect(key protowire.Number, val reflect.Value) {
	if !isSliceOrArray(val) {
		panic("passed value is not slice or array")
	}

	sliceLen := val.Len()
	elem := val.Type().Elem()
	result := marshaller{}

	switch elem.Kind() { //nolint:exhaustive
	case reflect.Int8, reflect.Int16:
		for i := 0; i < sliceLen; i++ {
			putInt32(&result, int32(val.Index(i).Int()))
		}

	case reflect.Uint8, reflect.Uint16:
		for i := 0; i < sliceLen; i++ {
			putInt32(&result, uint32(val.Index(i).Uint()))
		}

	case reflect.Bool:
		for i := 0; i < sliceLen; i++ {
			putBool(&result, val.Index(i).Bool())
		}

	case reflect.Int, reflect.Int32, reflect.Int64:
		for i := 0; i < sliceLen; i++ {
			putUVarint(&result, val.Index(i).Int())
		}

	case reflect.Uint, reflect.Uint32, reflect.Uint64:
		for i := 0; i < sliceLen; i++ {
			putUVarint(&result, val.Index(i).Uint())
		}

	case reflect.Float32:
		for i := 0; i < sliceLen; i++ {
			putInt32(&result, math.Float32bits(float32(val.Index(i).Float())))
		}

	case reflect.Float64:
		for i := 0; i < sliceLen; i++ {
			putInt64(&result, math.Float64bits(val.Index(i).Float()))
		}

	case reflect.Pointer:
		if !isSlicePtrElemSupported(elem) {
			panic(fmt.Errorf("unsupported type: '%s'", val.String()))
		}

		for i := 0; i < sliceLen; i++ {
			m.encodeValue(key, val.Index(i))
		}

		return

	case reflect.Map:
		panic(fmt.Errorf("unsupported type %s", val.Type().String()))

	default: // Write each element as a separate key,value pair
		if elem.Kind() == reflect.Slice || elem.Kind() == reflect.Array {
			subSlice := elem.Elem()
			if subSlice.Kind() != reflect.Uint8 {
				panic("unsupported type: error no support for 2-dimensional array except for [][]byte")
			}
		}

		for i := 0; i < sliceLen; i++ {
			m.encodeValue(key, val.Index(i))
		}

		return
	}

	putTag(m, key, protowire.BytesType)
	putBytes(m, result.buf)
}

func isSlicePtrElemSupported(elem reflect.Type) bool {
	elem = deref(elem)

	switch elem.Kind() { //nolint:exhaustive
	case reflect.Int8, reflect.Int16, reflect.Uint8, reflect.Uint16, reflect.Bool,
		reflect.Int, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return false

	case reflect.Slice, reflect.Array:
		if elem.Elem().Kind() == reflect.Uint8 {
			return true
		}

		return false

	default:
		return true
	}
}

func (m *marshaller) encodeMap(key protowire.Number, mpval reflect.Value) {
	first := true

	for _, mkey := range mpval.MapKeys() {
		mval := mpval.MapIndex(mkey)

		if first {
			// map key can only be a primitive type or a string
			switch mkey.Kind() { //nolint:exhaustive
			case reflect.Struct, reflect.Array, reflect.Interface, reflect.Pointer:
				panic(errors.New("unsupported type: map key cannot be struct, array, interface or pointer"))
			}

			unwrapVal := deref(mval.Type())

			switch unwrapVal.Kind() { //nolint:exhaustive
			case reflect.Slice, reflect.Array:
				if mval.Type().Elem() == typeByte {
					break
				}

				fallthrough
			case reflect.Interface:
				panic(errors.New("unsupported type: map value cannot be non byte slice, array or interface"))
			}

			first = false
		}

		if kind := mval.Kind(); kind == reflect.Pointer && mval.IsNil() {
			panic("error: map has nil element")
		}

		inner := marshaller{}
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

func isSliceOrArray(val reflect.Value) bool {
	kind := val.Kind()

	return kind == reflect.Slice || kind == reflect.Array
}
