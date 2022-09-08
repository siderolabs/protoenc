// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package protoenc

import (
	"errors"
	"fmt"
	"math"
	"reflect"

	"google.golang.org/protobuf/encoding/protowire"
)

func makeScanner(buf []byte) *scanner {
	return &scanner{
		buf: buf,
	}
}

//nolint:govet
type scanner struct {
	buf []byte

	lastFieldNum protowire.Number
	lastType     protowire.Type

	lastValueType valueType
	lastComplex   complexValue
	lastPrimitive primitiveValue

	lastErr error
}

type valueType int8

const (
	valueTypeInvalid valueType = iota
	valueTypeComplex
	valueTypePrimitive
)

func (s *scanner) Scan() bool {
	if len(s.buf) == 0 || s.lastErr != nil {
		return false
	}

	s.lastErr = s.scan()

	return s.lastErr == nil
}

func (s *scanner) Err() error {
	return s.lastErr
}

func (s *scanner) FieldNum() protowire.Number {
	return s.lastFieldNum
}

func (s *scanner) Primitive() (primitiveValue, bool) {
	if s.lastValueType != valueTypePrimitive {
		return primitiveValue{}, false
	}

	return s.lastPrimitive, true
}

func (s *scanner) Complex() (complexValue, bool) {
	if s.lastValueType != valueTypeComplex {
		return complexValue{}, false
	}

	return s.lastComplex, true
}

func (s *scanner) scan() error {
	s.lastValueType = valueTypeInvalid
	s.lastPrimitive = primitiveValue{}
	s.lastComplex = complexValue{}
	s.lastFieldNum = 0
	s.lastType = 0

	fieldnum, wiretype, n := protowire.ConsumeTag(s.buf)
	if n <= 0 {
		return errors.New("bad protobuf field key")
	}

	s.lastFieldNum = fieldnum
	s.lastType = wiretype
	s.buf = s.buf[n:]

	ds := makeDataScanner(s.lastType, s.buf)

	if !ds.Scan() {
		if err := ds.Err(); err != nil {
			return err
		}

		return errors.New("protobuf data scanner failed")
	}

	s.buf = ds.buf

	if val, ok := ds.ComplexValue(); ok {
		s.lastComplex = val
		s.lastValueType = valueTypeComplex
	} else if val, ok := ds.PrimitiveValue(); ok {
		s.lastPrimitive = val
		s.lastValueType = valueTypePrimitive
	} else {
		return errors.New("bad value type")
	}

	return nil
}

type primitiveValue struct {
	value    uint64
	wireType protowire.Type
}

func (v *primitiveValue) WireType() protowire.Type {
	return v.wireType
}

func (v *primitiveValue) Bool() (bool, error) {
	if v.wireType != protowire.VarintType {
		return false, fmt.Errorf("bad wiretype for bool: %v", v.wireType)
	}

	return protowire.DecodeBool(v.value), nil
}

func (v *primitiveValue) Int() (int64, error) {
	switch v.wireType { //nolint:exhaustive
	case protowire.VarintType:
		return int64(v.value), nil
	case protowire.Fixed32Type:
		return int64(int32(v.value)), nil
	case protowire.Fixed64Type:
		return int64(v.value), nil
	default:
		return -1, fmt.Errorf("bad wiretype for int: %v", v.wireType)
	}
}

func (v *primitiveValue) Uint() (uint64, error) {
	switch v.wireType { //nolint:exhaustive
	case protowire.VarintType:
		return v.value, nil
	case protowire.Fixed32Type:
		return uint64(uint32(v.value)), nil
	case protowire.Fixed64Type:
		return v.value, nil
	default:
		return 0, fmt.Errorf("bad wiretype for uint: %v", v.wireType)
	}
}

func (v *primitiveValue) Float32() (float32, error) {
	if v.wireType != protowire.Fixed32Type {
		return 0, fmt.Errorf("bad wiretype for float32: %v", v.wireType)
	}

	return math.Float32frombits(uint32(v.value)), nil
}

func (v *primitiveValue) Float64() (float64, error) {
	if v.wireType != protowire.Fixed64Type {
		return 0, fmt.Errorf("bad wiretype for float64: %v", v.wireType)
	}

	return math.Float64frombits(v.value), nil
}

type complexValue struct {
	value    []byte
	wireType protowire.Type
}

func (c *complexValue) Bytes() ([]byte, error) {
	if c.wireType != protowire.BytesType {
		return nil, fmt.Errorf("bad wiretype for bytes: %v", c.wireType)
	}

	return c.value, nil
}

func makeDataScanner(wiretype protowire.Type, buf []byte) dataScanner {
	var lastErr error
	if len(buf) == 0 {
		lastErr = errors.New("buffer for data scanner cannot be empty")
	}

	return dataScanner{
		wiretype: wiretype,
		buf:      buf,
		lastErr:  lastErr,
	}
}

type dataScanner struct {
	lastErr  error
	lastBuf  []byte
	buf      []byte
	lastVal  uint64
	wiretype protowire.Type
}

func (s *dataScanner) Scan() bool {
	if s.wiretype < 0 {
		s.lastErr = fmt.Errorf("negative wiretype: %v", s.wiretype)
	}

	if len(s.buf) == 0 || s.lastErr != nil {
		return false
	}

	s.lastBuf = nil
	s.lastVal = 0

	switch s.wiretype { //nolint:exhaustive
	case protowire.VarintType:
		val, n := protowire.ConsumeVarint(s.buf)
		if n <= 0 {
			s.lastErr = errors.New("bad protobuf varint value")

			return false
		}

		s.buf = s.buf[n:]
		s.lastVal = val

	case protowire.Fixed32Type:
		val, n := protowire.ConsumeFixed32(s.buf)
		if n <= 0 {
			s.lastErr = errors.New("bad protobuf 32-bit value")

			return false
		}

		s.buf = s.buf[n:]
		s.lastVal = uint64(val)

	case protowire.Fixed64Type:
		val, n := protowire.ConsumeFixed64(s.buf)
		if n <= 0 {
			s.lastErr = errors.New("bad protobuf 64-bit value")

			return false
		}

		s.buf = s.buf[n:]
		s.lastVal = val

	case protowire.BytesType:
		val, n := protowire.ConsumeBytes(s.buf)
		if n <= 0 {
			s.lastErr = errors.New("bad protobuf length-delimited value")
		}

		s.buf = s.buf[n:]
		s.lastBuf = val[:len(val):len(val)]

	default:
		s.lastErr = errors.New("unknown protobuf wire-type")
	}

	return true
}

func (s *dataScanner) Err() error {
	return s.lastErr
}

func (s *dataScanner) ComplexValue() (complexValue, bool) {
	return complexValue{
		wireType: s.wiretype,
		value:    s.lastBuf,
	}, s.lastBuf != nil
}

func (s *dataScanner) PrimitiveValue() (primitiveValue, bool) {
	return primitiveValue{
		wireType: s.wiretype,
		value:    s.lastVal,
	}, s.lastBuf == nil
}

func (s *dataScanner) Wiretype() protowire.Type {
	if s.wiretype < 0 {
		panic(fmt.Errorf("invalid wiretype: %v", s.wiretype))
	}

	return s.wiretype
}

func getDataScannerFor(eltype reflect.Type, buf []byte) (dataScanner, bool, error) {
	switch eltype.Kind() { //nolint:exhaustive
	case reflect.Uint8, reflect.Uint16, reflect.Int8, reflect.Int16:
		return makeDataScanner(protowire.VarintType, buf), true, nil

	case reflect.Bool, reflect.Int32, reflect.Int64, reflect.Int,
		reflect.Uint32, reflect.Uint64, reflect.Uint:
		if (eltype.Kind() == reflect.Int || eltype.Kind() == reflect.Uint) && eltype.Size() < 8 {
			return dataScanner{}, false, errors.New("detected a 32bit machine, please either use (u)int64 or (u)int32")
		}

		switch eltype {
		case typeFixedS32:
			return makeDataScanner(protowire.Fixed32Type, buf), true, nil
		case typeFixedS64:
			return makeDataScanner(protowire.Fixed64Type, buf), true, nil
		case typeFixedU32:
			return makeDataScanner(protowire.Fixed32Type, buf), true, nil
		case typeFixedU64:
			return makeDataScanner(protowire.Fixed64Type, buf), true, nil
		case typeDuration:
			return dataScanner{}, false, nil
		default:
			return makeDataScanner(protowire.VarintType, buf), true, nil
		}

	case reflect.Float32:
		return makeDataScanner(protowire.Fixed32Type, buf), true, nil

	case reflect.Float64:
		return makeDataScanner(protowire.Fixed64Type, buf), true, nil
	default:
		return dataScanner{}, false, nil
	}
}
