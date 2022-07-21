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

		// For more debugging output, uncomment the following three lines.
		// if fieldi < len(fields){
		//   fmt.Printf("Decoding FieldName %+v\n", fields[fieldi].Field)
		// }
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
		v  uint64
		n  int
		vb []byte
	)

	switch wiretype { //nolint:exhaustive
	case protowire.VarintType:
		v, n = protowire.ConsumeVarint(buf)
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

		v = uint64(res)
		buf = buf[n:]

	case protowire.Fixed64Type:
		var res uint64
		res, n = protowire.ConsumeFixed64(buf)

		if n <= 0 {
			return nil, errors.New("bad protobuf 64-bit value")
		}

		v = res
		buf = buf[n:]

	case protowire.BytesType:
		vb, n = protowire.ConsumeBytes(buf)
		if n <= 0 {
			return nil, errors.New("bad protobuf length-delimited value")
		}

		vb = vb[:len(vb):len(vb)]
		buf = buf[n:]

	default:
		return nil, errors.New("unknown protobuf wire-type")
	}

	if err := u.putInto(dst, wiretype, v, vb); err != nil {
		return nil, err
	}

	return buf, nil
}

//nolint:gocognit,gocyclo,cyclop
func (u *unmarshaller) putInto(dst reflect.Value, wiretype protowire.Type, v uint64, vb []byte) error {
	// Value is not settable (invalid reflect.Value, private
	if !dst.CanSet() {
		return nil
	}

	// Check predefined pb types
	switch dst.Type() {
	case typeTime:
		if wiretype != protowire.BytesType {
			return fmt.Errorf("bad wiretype for time.Time: %v", wiretype)
		}

		var t timestamppb.Timestamp

		err := proto.Unmarshal(vb, &t)
		if err != nil {
			return err
		}

		dst.Set(reflect.ValueOf(t.AsTime()))

		return nil
	case typeDuration:
		if wiretype != protowire.BytesType {
			return fmt.Errorf("bad wiretype for time.Duration: %v", wiretype)
		}

		var d durationpb.Duration

		err := proto.Unmarshal(vb, &d)
		if err != nil {
			return err
		}

		dst.Set(reflect.ValueOf(d.AsDuration()))

		return nil
	}

	switch dst.Kind() { //nolint:exhaustive
	case reflect.Bool:
		if wiretype != protowire.VarintType {
			return fmt.Errorf("bad wiretype for bool: %v", wiretype)
		}

		if v > 1 {
			return errors.New("invalid bool value")
		}

		dst.SetBool(protowire.DecodeBool(v))

	case reflect.Int, reflect.Int32, reflect.Int64:
		// Signed integers may be encoded either zigzag-varint or fixed
		// Note that protobufs don't support 8- or 16-bit ints.
		if dst.Kind() == reflect.Int && dst.Type().Size() < 8 {
			return errors.New("detected a 32bit machine, please use either int64 or int32")
		}

		sv, err := decodeSignedInt(wiretype, v)
		if err != nil {
			fmt.Println("Error Reflect.Int for v=", v, "wiretype=", wiretype, "for Value=", dst.Type().Name())

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
			dst.SetUint(v)
		case protowire.Fixed32Type:
			dst.SetUint(uint64(uint32(v)))
		case protowire.Fixed64Type:
			dst.SetUint(v)
		default:
			return errors.New("bad wiretype for uint")
		}

	case reflect.Float32:
		if wiretype != protowire.Fixed32Type {
			return errors.New("bad wiretype for float32")
		}

		dst.SetFloat(float64(math.Float32frombits(uint32(v))))

	case reflect.Float64:
		if wiretype != protowire.Fixed64Type {
			return errors.New("bad wiretype for float64")
		}

		dst.SetFloat(math.Float64frombits(v))

	case reflect.String:
		if wiretype != protowire.BytesType {
			return errors.New("bad wiretype for string")
		}

		dst.SetString(string(vb))

	case reflect.Ptr:
		if dst.IsNil() {
			err := instantiate(dst)
			if err != nil {
				return err
			}
		}

		return u.putInto(dst.Elem(), wiretype, v, vb)

	case reflect.Struct:
		if enc, ok := dst.Addr().Interface().(encoding.BinaryUnmarshaler); ok {
			return enc.UnmarshalBinary(vb)
		}

		if wiretype != protowire.BytesType {
			return errors.New("bad wiretype for embedded message")
		}

		return u.unmarshalStruct(vb, dst)

	case reflect.Slice, reflect.Array:
		// Repeated field or byte-slice
		if wiretype != protowire.BytesType {
			return errors.New("bad wiretype for repeated field")
		}

		return u.slice(dst, vb)
	case reflect.Map:
		if wiretype != protowire.BytesType {
			return errors.New("bad wiretype for repeated field")
		}

		if dst.IsNil() {
			dst.Set(reflect.MakeMap(dst.Type()))
		}

		return u.mapEntry(dst, vb)
	case reflect.Interface:
		data := vb

		// TODO: find a way to handle nil interfaces
		if dst.IsNil() {
			return errors.New("nil interface fields are not supported")
		}

		// If the object support self-decoding, use that.
		if enc, ok := dst.Interface().(encoding.BinaryUnmarshaler); ok {
			if wiretype != protowire.BytesType {
				return errors.New("bad wiretype for bytes")
			}

			return enc.UnmarshalBinary(data)
		}

		// Decode into the object the interface points to.
		// XXX perhaps better ONLY to support self-decoding
		// for interface fields?
		return Unmarshal(vb, dst.Interface())

	default:
		panic("unsupported value kind " + dst.Kind().String())
	}

	return nil
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

func (u *unmarshaller) slice(slice reflect.Value, vb []byte) error {
	// Find the element type, and create a temporary instance of it.
	eltype := slice.Type().Elem()
	val := reflect.New(eltype).Elem()

	ok, err := tryDecodeUnpackedByteSlice(slice, eltype, vb)
	if err != nil {
		return err
	}

	if ok {
		return nil
	}

	wiretype, err := getWiretype(eltype)
	if err != nil {
		return err
	}

	if wiretype < 0 { // Other unpacked repeated types
		// Just unpack and append one value from vb.
		if err := u.putInto(val, protowire.BytesType, 0, vb); err != nil {
			return err
		}

		if slice.Kind() != reflect.Slice {
			return errors.New("append to non-slice")
		}

		slice.Set(reflect.Append(slice, val))

		return nil
	}

	// Decode packed values from the buffer and append them to the slice.
	for len(vb) > 0 {
		rem, err := u.decodeValue(wiretype, vb, val)
		if err != nil {
			return err
		}

		slice.Set(reflect.Append(slice, val))

		vb = rem
	}

	return nil
}

func getWiretype(eltype reflect.Type) (protowire.Type, error) {
	switch eltype.Kind() { //nolint:exhaustive
	case reflect.Bool, reflect.Int32, reflect.Int64, reflect.Int,
		reflect.Uint32, reflect.Uint64, reflect.Uint:
		if (eltype.Kind() == reflect.Int || eltype.Kind() == reflect.Uint) && eltype.Size() < 8 {
			return 0, errors.New("detected a 32bit machine, please either use (u)int64 or (u)int32")
		}

		switch eltype {
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

func tryDecodeUnpackedByteSlice(slice reflect.Value, eltype reflect.Type, vb []byte) (bool, error) {
	if eltype.Kind() != reflect.Uint8 {
		return false, nil
	}

	if slice.Kind() == reflect.Array {
		if slice.Len() != len(vb) {
			return false, errors.New("array length and buffer length differ")
		}

		for i := 0; i < slice.Len(); i++ {
			// no SetByte method in reflect so has to pass down by uint64
			slice.Index(i).SetUint(uint64(vb[i]))
		}
	} else {
		slice.SetBytes(vb)
	}

	return true, nil
}

func (u *unmarshaller) mapEntry(slval reflect.Value, vb []byte) error {
	mKey := reflect.New(slval.Type().Key()).Elem()
	mVal := reflect.New(slval.Type().Elem()).Elem()

	_, wiretype, n := protowire.ConsumeTag(vb)
	if n <= 0 {
		return errors.New("bad protobuf field key")
	}

	buf := vb[n:]

	var err error
	buf, err = u.decodeValue(wiretype, buf, mKey)

	if err != nil {
		return err
	}

	for len(buf) > 0 { // for repeated values (slices etc)
		_, wiretype, n := protowire.ConsumeTag(buf)
		if n <= 0 {
			return errors.New("bad protobuf field key")
		}

		buf = buf[n:]
		buf, err = u.decodeValue(wiretype, buf, mVal)

		if err != nil {
			return err
		}
	}

	if !mKey.IsValid() || !mVal.IsValid() {
		// We did not decode the key or the value in the map entry.
		// Either way, it's an invalid map entry.
		return errors.New("proto: bad map data: missing key/val")
	}

	slval.SetMapIndex(mKey, mVal)

	return nil
}
