// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package protoenc

import (
	"encoding"
	"errors"
	"fmt"
	"reflect"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Unmarshal a protobuf value into a Go value.
// The caller must pass a pointer to the struct to decode into.
func Unmarshal(buf []byte, structPtr interface{}) error {
	return unmarshal(buf, structPtr)
}

func unmarshal(buf []byte, structPtr interface{}) (returnErr error) {
	defer func() {
		if r := recover(); r != nil {
			switch e := r.(type) {
			case string:
				returnErr = errors.New(e)
			case error:
				returnErr = e
			default:
				returnErr = errors.New("failed to decode the field")
			}
		}
	}()

	if structPtr == nil {
		return nil
	}

	if hasCustomEncoders(reflect.TypeOf(structPtr)) {
		return errors.New("custom decoders are not supported for top-level structs, use BinaryUnmarshaler instead")
	}

	if bu, ok := structPtr.(encoding.BinaryUnmarshaler); ok {
		return bu.UnmarshalBinary(buf)
	}

	val := reflect.ValueOf(structPtr)
	if val.Kind() != reflect.Pointer || val.Type().Elem().Kind() != reflect.Struct {
		return errors.New("unmarshal takes a pointer to struct")
	}

	return unmarshalStruct(val.Elem(), buf)
}

func unmarshalStruct(structVal reflect.Value, buf []byte) error {
	zeroStructFields(structVal)

	structFields, err := StructFields(structVal.Type())
	if err != nil {
		return err
	}

	rdr := makeScanner(buf)

	for rdr.Scan() {
		var field reflect.Value

		fieldIndex := findField(structFields, rdr.FieldNum())
		if fieldIndex != -1 {
			field = initStructField(structVal, structFields[fieldIndex])
		}

		if err = putValue(field, rdr); err != nil {
			if fieldIndex != -1 {
				return fmt.Errorf("error while unmarshalling field '%s' of struct '%s.%s': %w",
					structFields[fieldIndex].Field.Name,
					structVal.Type().PkgPath(),
					structVal.Type().Name(),
					err)
			}

			return err
		}
	}

	if err := rdr.Err(); err != nil {
		return err
	}

	return nil
}

func putValue(dst reflect.Value, rdr *scanner) error {
	if val, ok := rdr.Primitive(); ok {
		err := unmarshalPrimitive(dst, val)
		if err != nil {
			return fmt.Errorf("error while unmarshalling primitive '%v': %w", val, err)
		}

		return nil
	} else if val, ok := rdr.Complex(); ok {
		err := unmarshalBytes(dst, val)
		if err != nil {
			return fmt.Errorf("error while unmarshalling complex '%v': %w", val, err)
		}

		return nil
	}

	panic("unexpected value")
}

func zeroStructFields(val reflect.Value) {
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)

		if field.CanSet() {
			field.Set(reflect.Zero(field.Type()))
		}
	}
}

func initStructField(structField reflect.Value, fieldData FieldData) reflect.Value {
	index := fieldData.FieldIndex
	if len(index) == 0 {
		panic(fmt.Errorf("field '%s' has no index", fieldData.Field.Name))
	}

	var result reflect.Value

	for i := range index {
		path := index[:i+1]

		result = structField.FieldByIndex(path)
		if result.Kind() == reflect.Pointer && result.IsNil() {
			result.Set(reflect.New(result.Type().Elem()))
		}
	}

	if !result.IsValid() {
		panic(fmt.Errorf("field was not initialized"))
	}

	return result
}

func findField(fields []FieldData, fieldnum protowire.Number) int {
	idx := 0

	for idx < len(fields) && fields[idx].Num != fieldnum {
		idx++
	}

	if idx == len(fields) {
		return -1
	}

	return idx
}

func tryDecodeFunc(vb []byte, dst reflect.Value) (bool, error) {
	dec, ok := decoders.Get(dst.Type())
	if !ok {
		return false, nil
	}

	if err := dec(vb, dst); err != nil {
		return false, err
	}

	return true, nil
}

func mapEntry(dstEntry reflect.Value, buf []byte) error {
	entryKey := reflect.New(dstEntry.Type().Key()).Elem()
	entryVal := reflect.New(dstEntry.Type().Elem()).Elem()

	s := makeScanner(buf)

	// scan key
	if !s.Scan() {
		if s.Err() != nil {
			return s.Err()
		}

		return errors.New("map key is missing")
	}

	if err := putValue(entryKey, s); err != nil {
		return fmt.Errorf("failed to unmarshal map key type:'%s': %w", entryKey.Type().String(), err)
	}

	// scan value
	if s.Scan() {
		if err := putValue(entryVal, s); err != nil {
			return fmt.Errorf("failed to unmarshal map value type:'%s': %w", entryKey.Type().String(), err)
		}
	}

	if s.Err() != nil {
		return fmt.Errorf("map scanning failed: %w", s.Err())
	}

	// scan more and fail if there is more
	if s.Scan() {
		return errors.New("map entry cannot have several values")
	}

	if !entryKey.IsValid() || !entryVal.IsValid() {
		return errors.New("proto: bad map data: missing key/val")
	}

	dstEntry.SetMapIndex(entryKey, entryVal)

	return nil
}

func unmarshalPrimitive(dst reflect.Value, value primitiveValue) error {
	// Value is not settable (invalid reflect.Value, private)
	if !dst.CanSet() {
		return nil
	}

	switch dst.Kind() { //nolint:exhaustive
	case reflect.Pointer:
		if dst.IsNil() {
			instantiate(dst)
		}

		return unmarshalPrimitive(dst.Elem(), value)

	case reflect.Bool:
		val, err := value.Bool()
		if err != nil {
			return err
		}

		dst.SetBool(val)

		return nil

	case reflect.Int, reflect.Int32, reflect.Int64,
		reflect.Int8, reflect.Int16: // Those two are a special case
		if dst.Kind() == reflect.Int && dst.Type().Size() < 8 {
			return errors.New("detected a 32bit machine, please use either int64 or int32")
		}

		val, err := value.Int()
		if err != nil {
			return err
		}

		dst.SetInt(val)

		return nil

	case reflect.Uint, reflect.Uint32, reflect.Uint64,
		reflect.Uint8, // This is a special case for uint8 kind, []uint8 values will be decoded as protobuf 'bytes'
		reflect.Uint16:
		if dst.Kind() == reflect.Uint && dst.Type().Size() < 8 {
			return errors.New("detected a 32bit machine, please use either uint64 or uint32")
		}

		val, err := value.Uint()
		if err != nil {
			return err
		}

		dst.SetUint(val)

		return nil

	case reflect.Float32:
		val, err := value.Float32()
		if err != nil {
			return err
		}

		dst.SetFloat(float64(val))

		return nil

	case reflect.Float64:
		val, err := value.Float64()
		if err != nil {
			return err
		}

		dst.SetFloat(val)

		return nil

	default:
		return fmt.Errorf("unsupported primitive kind " + dst.Kind().String())
	}
}

// Instantiate an arbitrary type, handling dynamic interface types.
// Returns a Ptr value.
func instantiate(dst reflect.Value) {
	dstType := dst.Type().Elem()

	dst.Set(reflect.New(dstType))
}

//nolint:cyclop,gocyclo
func unmarshalBytes(dst reflect.Value, value complexValue) (err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("failed to unmarshal bytes: %w", err)
		}
	}()

	// Value is not settable (invalid reflect.Value, private)
	if !dst.CanSet() {
		return nil
	}

	bytes, err := value.Bytes()
	if err != nil {
		return fmt.Errorf("bad wiretype for complex types: %w", err)
	}

	// Check predefined pb types
	switch dst.Type() {
	case typeTime:
		var result timestamppb.Timestamp

		err = proto.Unmarshal(bytes, &result)
		if err != nil {
			return err
		}

		dst.Set(reflect.ValueOf(result.AsTime()))

		return nil
	case typeDuration:
		var result durationpb.Duration

		err = proto.Unmarshal(bytes, &result)
		if err != nil {
			return err
		}

		dst.Set(reflect.ValueOf(result.AsDuration()))

		return nil
	case typeMapInterface:
		var result structpb.Struct

		err = proto.Unmarshal(bytes, &result)
		if err != nil {
			return err
		}

		dst.Set(reflect.ValueOf(result.AsMap()))

		return nil
	}

	switch dst.Kind() { //nolint:exhaustive
	case reflect.Pointer:
		if dst.IsNil() {
			instantiate(dst)
		}

		// If the pointer is to a struct
		if indirect(dst.Type()).Kind() == reflect.Struct {
			ok, err := tryDecodeFunc(bytes, dst)
			if err != nil {
				return err
			}

			if ok {
				return nil
			}
		}

		return unmarshalBytes(dst.Elem(), value)

	case reflect.String:
		dst.SetString(string(bytes))

		return nil

	case reflect.Struct:
		if ok, err := tryDecodeFunc(bytes, dst); ok {
			return nil
		} else if err != nil {
			return err
		}

		if enc, ok := dst.Addr().Interface().(encoding.BinaryUnmarshaler); ok {
			return enc.UnmarshalBinary(bytes)
		}

		return unmarshalStruct(dst, bytes)

	case reflect.Slice, reflect.Array:
		return slice(dst, value)

	case reflect.Map:
		if dst.IsNil() {
			dst.Set(reflect.MakeMap(dst.Type()))
		}

		return mapEntry(dst, bytes)

	default:
		return fmt.Errorf("unsupported value kind " + dst.Kind().String())
	}
}

func unmarshalByteSeqeunce(dst reflect.Value, val complexValue) error {
	unmarshalBytes, err := val.Bytes()
	if err != nil {
		return err
	}

	if dst.Kind() == reflect.Array {
		if dst.Len() != len(unmarshalBytes) {
			return errors.New("array length and buffer length differ")
		}

		for i := 0; i < dst.Len(); i++ {
			// no SetByte method in reflect so has to pass down by uint64
			dst.Index(i).SetUint(uint64(unmarshalBytes[i]))
		}
	} else {
		dst.SetBytes(unmarshalBytes)
	}

	return nil
}

func slice(dst reflect.Value, val complexValue) error {
	elemType := dst.Type().Elem()

	// we only decode bytes as []byte or [n]byte field
	if elemType == typeByte {
		err := unmarshalByteSeqeunce(dst, val)
		if err != nil {
			return err
		}

		return nil
	}

	bytes, err := val.Bytes()
	if err != nil {
		return err
	}

	ds, ok, err := getDataScannerFor(elemType, bytes)
	if err != nil {
		return err
	}

	if !ok { // Other unpacked repeated types
		// Just unpack and append one value from buf.
		elem := reflect.New(elemType).Elem()

		if err = unmarshalBytes(elem, val); err != nil {
			return err
		}

		dst.Set(reflect.Append(dst, elem))

		return nil
	}

	ok, err = tryUnmarshalPredefinedSliceTypes(ds.Wiretype(), bytes, dst)

	switch {
	case ok:
		return nil
	case err != nil:
		return err
	}

	sw := sequenceWrapper{
		seq: dst,
	}

	defer sw.FixLen()

	// Decode packed values from the buffer and append them to the dst.
	for ds.Scan() {
		nextElem := sw.NextElem()

		value, ok := ds.PrimitiveValue()
		if !ok {
			return errors.New("incorrect value in packed slice")
		}

		err := unmarshalPrimitive(nextElem, value)
		if err != nil {
			return fmt.Errorf("failed to unmarshal slice type '%s': %w", dst.Type(), err)
		}
	}

	return ds.Err()
}

type sequenceWrapper struct {
	seq reflect.Value
	idx int
}

func (w *sequenceWrapper) NextElem() reflect.Value {
	if w.seq.Kind() == reflect.Array {
		result := w.seq.Index(w.idx)
		w.idx++

		return result
	}

	if sliceCap := w.seq.Cap(); w.idx == sliceCap {
		w.seq.Set(grow(w.seq, 1))
	}

	result := w.seq.Index(w.idx)
	w.idx++

	return result
}

func (w *sequenceWrapper) FixLen() {
	if w.seq.Kind() == reflect.Array {
		return
	}

	w.seq.SetLen(w.idx)
}

// grow grows the slice s so that it can hold extra more values, allocating
// more capacity if needed. It also returns the new cap.
func grow(s reflect.Value, extra int) reflect.Value {
	oldLen := s.Len()
	newLen := oldLen + extra

	if newLen < oldLen {
		panic("reflect.Append: slice overflow")
	}

	targetCap := s.Cap()
	if newLen <= targetCap {
		return s.Slice(0, targetCap)
	}

	if targetCap == 0 {
		targetCap = extra
	} else {
		const threshold = 256
		for targetCap < newLen {
			if oldLen < threshold {
				targetCap += targetCap
			} else {
				targetCap += (targetCap + 3*threshold) / 4
			}
		}
	}

	t := reflect.MakeSlice(s.Type(), targetCap, targetCap)

	reflect.Copy(t, s)

	return t
}
