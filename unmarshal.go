// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package protoenc

import (
	"encoding"
	"errors"
	"fmt"
	"math"
	"reflect"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Decoder is the main struct used to decode a protobuf blob.
type unmarshaller struct{}

// Unmarshal a protobuf value into a Go value.
// The caller must pass a pointer to the struct to decode into.
func Unmarshal(buf []byte, ptr interface{}) error {
	return unmarshal(buf, ptr)
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

	if bu, ok := structPtr.(encoding.BinaryUnmarshaler); ok {
		return bu.UnmarshalBinary(buf)
	}

	val := reflect.ValueOf(structPtr)
	if val.Kind() != reflect.Ptr {
		return errors.New("decode has been given a non pointer type")
	}

	de := unmarshaller{}

	return de.unmarshalStruct(buf, val.Elem())
}

func (u *unmarshaller) unmarshalStruct(buf []byte, structVal reflect.Value) error {
	if structVal.Kind() != reflect.Struct {
		return errors.New("not a struct")
	}

	zeroStructFields(structVal)

	ok, err := tryDecodeFunc(buf, structVal)
	if err != nil {
		return err
	}

	if ok {
		return nil
	}

	structFields, err := StructFields(structVal.Type())
	if err != nil {
		return err
	}

	for len(buf) > 0 {
		fieldnum, wiretype, n := protowire.ConsumeTag(buf)
		if n <= 0 {
			return errors.New("bad protobuf field key")
		}

		buf = buf[n:]

		var field reflect.Value

		fieldIndex := findField(structFields, fieldnum)
		if fieldIndex != -1 {
			field = initStructField(structVal, structFields[fieldIndex])
		}

		// Decode the field's value
		rem, err := u.decodeValue(wiretype, buf, field)
		if err != nil {
			if fieldIndex != -1 {
				return fmt.Errorf("error while unmarshalling field %+v: %w", structFields[fieldIndex].Field, err)
			}

			return err
		}

		buf = rem
	}

	return nil
}

func zeroStructFields(val reflect.Value) {
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		if field.Kind() == reflect.Interface {
			// Interface are not reset because the unmarshaller won't
			// be able to instantiate it again.
			continue
		}

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
		if result.Kind() == reflect.Ptr && result.IsNil() {
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

// Pull a value from the buffer and put it into a reflective Value.
func (u *unmarshaller) decodeValue(wiretype protowire.Type, buf []byte, dst reflect.Value) ([]byte, error) {
	var (
		// Break out the value from the buffer based on the wire type
		decodedValue uint64
		n            int
		decodedBytes []byte
	)

	switch wiretype { //nolint:exhaustive
	case protowire.VarintType:
		decodedValue, n = protowire.ConsumeVarint(buf)
		if n <= 0 {
			return nil, errors.New("bad protobuf varint value")
		}

		buf = buf[n:]

	case protowire.Fixed32Type:
		var res uint32
		res, n = protowire.ConsumeFixed32(buf)

		if n <= 0 {
			return nil, errors.New("bad protobuf 32-bit value")
		}

		decodedValue = uint64(res)
		buf = buf[n:]

	case protowire.Fixed64Type:
		var res uint64
		res, n = protowire.ConsumeFixed64(buf)

		if n <= 0 {
			return nil, errors.New("bad protobuf 64-bit value")
		}

		decodedValue = res
		buf = buf[n:]

	case protowire.BytesType:
		decodedBytes, n = protowire.ConsumeBytes(buf)
		if n <= 0 {
			return nil, errors.New("bad protobuf length-delimited value")
		}

		decodedBytes = decodedBytes[:len(decodedBytes):len(decodedBytes)]
		buf = buf[n:]

	default:
		return nil, errors.New("unknown protobuf wire-type")
	}

	if err := u.putInto(dst, wiretype, decodedValue, decodedBytes); err != nil {
		return nil, err
	}

	return buf, nil
}

//nolint:gocognit,gocyclo,cyclop
func (u *unmarshaller) putInto(dst reflect.Value, wiretype protowire.Type, decodedValue uint64, decodedBytes []byte) error {
	// Value is not settable (invalid reflect.Value, private)
	if !dst.CanSet() {
		return nil
	}

	// Check predefined pb types
	switch dst.Type() {
	case typeTime:
		if wiretype != protowire.BytesType {
			return fmt.Errorf("bad wiretype for time.Time: %v", wiretype)
		}

		var result timestamppb.Timestamp

		err := proto.Unmarshal(decodedBytes, &result)
		if err != nil {
			return err
		}

		dst.Set(reflect.ValueOf(result.AsTime()))

		return nil
	case typeDuration:
		if wiretype != protowire.BytesType {
			return fmt.Errorf("bad wiretype for time.Duration: %v", wiretype)
		}

		var result durationpb.Duration

		err := proto.Unmarshal(decodedBytes, &result)
		if err != nil {
			return err
		}

		dst.Set(reflect.ValueOf(result.AsDuration()))

		return nil
	}

	switch dst.Kind() { //nolint:exhaustive
	case reflect.Bool:
		if wiretype != protowire.VarintType {
			return fmt.Errorf("bad wiretype for bool: %v", wiretype)
		}

		if decodedValue > 1 {
			return errors.New("invalid bool value")
		}

		dst.SetBool(protowire.DecodeBool(decodedValue))

	case reflect.Int, reflect.Int32, reflect.Int64:
		// Signed integers may be encoded either zigzag-varint or fixed
		// Note that protobufs don't support 8- or 16-bit ints.
		if dst.Kind() == reflect.Int && dst.Type().Size() < 8 {
			return errors.New("detected a 32bit machine, please use either int64 or int32")
		}

		sv, err := decodeSignedInt(wiretype, decodedValue)
		if err != nil {
			fmt.Println("Error Reflect.Int for decodedValue=", decodedValue, "wiretype=", wiretype, "for Value=", dst.Type().Name())

			return err
		}

		dst.SetInt(sv)

	case reflect.Uint, reflect.Uint32, reflect.Uint64:
		// Varint-encoded 32-bit and 64-bit unsigned integers.
		if dst.Kind() == reflect.Uint && dst.Type().Size() < 8 {
			return errors.New("detected a 32bit machine, please use either uint64 or uint32")
		}

		switch wiretype { //nolint:exhaustive
		case protowire.VarintType:
			dst.SetUint(decodedValue)
		case protowire.Fixed32Type:
			dst.SetUint(uint64(uint32(decodedValue)))
		case protowire.Fixed64Type:
			dst.SetUint(decodedValue)
		default:
			return errors.New("bad wiretype for uint")
		}

	case reflect.Float32:
		if wiretype != protowire.Fixed32Type {
			return errors.New("bad wiretype for float32")
		}

		dst.SetFloat(float64(math.Float32frombits(uint32(decodedValue))))

	case reflect.Float64:
		if wiretype != protowire.Fixed64Type {
			return errors.New("bad wiretype for float64")
		}

		dst.SetFloat(math.Float64frombits(decodedValue))

	case reflect.String:
		if wiretype != protowire.BytesType {
			return errors.New("bad wiretype for string")
		}

		dst.SetString(string(decodedBytes))

	case reflect.Ptr:
		if dst.IsNil() {
			err := instantiate(dst)
			if err != nil {
				return err
			}
		}

		return u.putInto(dst.Elem(), wiretype, decodedValue, decodedBytes)

	case reflect.Struct:
		if enc, ok := dst.Addr().Interface().(encoding.BinaryUnmarshaler); ok {
			return enc.UnmarshalBinary(decodedBytes)
		}

		if wiretype != protowire.BytesType {
			return errors.New("bad wiretype for embedded message")
		}

		return u.unmarshalStruct(decodedBytes, dst)

	case reflect.Slice, reflect.Array:
		// Repeated field or byte-slice
		if wiretype != protowire.BytesType {
			return errors.New("bad wiretype for repeated field")
		}

		return u.slice(dst, decodedBytes)
	case reflect.Map:
		if wiretype != protowire.BytesType {
			return errors.New("bad wiretype for repeated field")
		}

		if dst.IsNil() {
			dst.Set(reflect.MakeMap(dst.Type()))
		}

		return u.mapEntry(dst, decodedBytes)
	case reflect.Interface:
		// TODO: find a way to handle nil interfaces
		if dst.IsNil() {
			return errors.New("nil interface fields are not supported")
		}

		// If the object support self-decoding, use that.
		if enc, ok := dst.Interface().(encoding.BinaryUnmarshaler); ok {
			if wiretype != protowire.BytesType {
				return errors.New("bad wiretype for bytes")
			}

			return enc.UnmarshalBinary(decodedBytes)
		}

		// Decode into the object the interface points to.
		return Unmarshal(decodedBytes, dst.Interface())

	default:
		panic("unsupported value kind " + dst.Kind().String())
	}

	return nil
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

func decodeSignedInt(wiretype protowire.Type, v uint64) (int64, error) {
	switch wiretype { //nolint:exhaustive
	case protowire.VarintType:
		return int64(v), nil
	case protowire.Fixed32Type:
		return int64(int32(v)), nil
	case protowire.Fixed64Type:
		return int64(v), nil
	default:
		return -1, errors.New("bad wiretype for sint")
	}
}

// Instantiate an arbitrary type, handling dynamic interface types.
// Returns a Ptr value.
func instantiate(dst reflect.Value) error {
	dstType := dst.Type()

	if dstType.Kind() == reflect.Interface {
		return fmt.Errorf("cannot instantiate interface type %s", dstType.Name())
	}

	dst.Set(reflect.New(dstType.Elem()))

	return nil
}

func (u *unmarshaller) slice(dst reflect.Value, decodedBytes []byte) error {
	// Find the element type, and create a temporary instance of it.
	elemType := dst.Type().Elem()

	ok, err := tryDecodeUnpackedByteSlice(dst, elemType, decodedBytes)
	if err != nil {
		return err
	}

	if ok {
		return nil
	}

	wiretype, err := getWiretypeFor(elemType)
	if err != nil {
		return err
	}

	if wiretype < 0 { // Other unpacked repeated types
		// Just unpack and append one value from decodedBytes.
		elem := reflect.New(elemType).Elem()
		if err := u.putInto(elem, protowire.BytesType, 0, decodedBytes); err != nil {
			return err
		}

		dst.Set(reflect.Append(dst, elem))

		return nil
	}

	sw := sequenceWrapper{
		seq: dst,
	}

	defer sw.FixLen()

	// Decode packed values from the buffer and append them to the dst.
	for len(decodedBytes) > 0 {
		nextElem := sw.NextElem()

		rem, err := u.decodeValue(wiretype, decodedBytes, nextElem)
		if err != nil {
			return err
		}

		decodedBytes = rem
	}

	return nil
}

func getWiretypeFor(elemType reflect.Type) (protowire.Type, error) {
	switch elemType.Kind() { //nolint:exhaustive
	case reflect.Bool, reflect.Int32, reflect.Int64, reflect.Int,
		reflect.Uint32, reflect.Uint64, reflect.Uint:
		if (elemType.Kind() == reflect.Int || elemType.Kind() == reflect.Uint) && elemType.Size() < 8 {
			return 0, errors.New("detected a 32bit machine, please either use (u)int64 or (u)int32")
		}

		switch elemType {
		case typeFixedS32:
			return protowire.Fixed32Type, nil
		case typeFixedS64:
			return protowire.Fixed64Type, nil
		case typeFixedU32:
			return protowire.Fixed32Type, nil
		case typeFixedU64:
			return protowire.Fixed64Type, nil
		case typeDuration:
			return -1, nil
		default:
			return protowire.VarintType, nil
		}

	case reflect.Float32:
		return protowire.Fixed32Type, nil

	case reflect.Float64:
		return protowire.Fixed64Type, nil
	default:
		return -1, nil
	}
}

func tryDecodeUnpackedByteSlice(dst reflect.Value, elemType reflect.Type, decodedBytes []byte) (bool, error) {
	if elemType.Kind() != reflect.Uint8 {
		return false, nil
	}

	if dst.Kind() == reflect.Array {
		if dst.Len() != len(decodedBytes) {
			return false, errors.New("array length and buffer length differ")
		}

		for i := 0; i < dst.Len(); i++ {
			// no SetByte method in reflect so has to pass down by uint64
			dst.Index(i).SetUint(uint64(decodedBytes[i]))
		}
	} else {
		dst.SetBytes(decodedBytes)
	}

	return true, nil
}

func (u *unmarshaller) mapEntry(dstEntry reflect.Value, decodedBytes []byte) error {
	entryKey := reflect.New(dstEntry.Type().Key()).Elem()
	entryVal := reflect.New(dstEntry.Type().Elem()).Elem()

	_, wiretype, n := protowire.ConsumeTag(decodedBytes)
	if n <= 0 {
		return errors.New("bad protobuf field key")
	}

	buf := decodedBytes[n:]

	var err error
	buf, err = u.decodeValue(wiretype, buf, entryKey)

	if err != nil {
		return err
	}

	for len(buf) > 0 { // for repeated values (slices etc)
		_, wiretype, n := protowire.ConsumeTag(buf)
		if n <= 0 {
			return errors.New("bad protobuf field key")
		}

		buf = buf[n:]
		buf, err = u.decodeValue(wiretype, buf, entryVal)

		if err != nil {
			return err
		}
	}

	if !entryKey.IsValid() || !entryVal.IsValid() {
		// We did not decode the key or the value in the map entry.
		// Either way, it's an invalid map entry.
		return errors.New("proto: bad map data: missing key/val")
	}

	dstEntry.SetMapIndex(entryKey, entryVal)

	return nil
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
