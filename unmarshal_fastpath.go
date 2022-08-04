// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package protoenc

import (
	"errors"
	"math"
	"reflect"
	"unsafe"

	"google.golang.org/protobuf/encoding/protowire"
)

var predefiniedDecoders = map[reflect.Type]func(buf []byte, dst reflect.Value) (bool, error){
	typeOf[[]int]():      decodeIntSlice[int],
	typeOf[[]int32]():    decodeIntSlice[int32],
	typeOf[[]int64]():    decodeIntSlice[int64],
	typeOf[[]uint]():     decodeIntSlice[uint],
	typeOf[[]uint32]():   decodeIntSlice[uint32],
	typeOf[[]uint64]():   decodeIntSlice[uint64],
	typeOf[[]FixedU32](): decodeFixed32[FixedU32],
	typeOf[[]FixedS32](): decodeFixed32[FixedS32],
	typeOf[[]FixedU64](): decodeFixed64[FixedU64],
	typeOf[[]FixedS64](): decodeFixed64[FixedS64],
	typeOf[[]float32]():  decodeFloat32,
	typeOf[[]float64]():  decodeFloat64,
}

func tryDecodePredefinedSlice(wiretype protowire.Type, buf []byte, dst reflect.Value) (bool, error) {
	switch wiretype { //nolint:exhaustive
	case protowire.VarintType, protowire.Fixed32Type, protowire.Fixed64Type:
		fn, ok := predefiniedDecoders[dst.Type()]
		if !ok {
			return false, nil
		}

		return fn(buf, dst)
	default:
		return false, nil
	}
}

type integer interface {
	int32 | int64 | uint32 | uint64 | int | uint
}

func decodeIntSlice[I integer](buf []byte, dst reflect.Value) (bool, error) {
	var result []I

	for len(buf) > 0 {
		v, n := protowire.ConsumeVarint(buf)
		if n < 0 {
			return false, errors.New("bad protobuf varint value")
		}

		buf = buf[n:]

		result = append(result, I(v))
	}

	*(*[]I)(unsafe.Pointer(dst.UnsafeAddr())) = result

	return true, nil
}

func decodeFixed32[F FixedU32 | FixedS32](buf []byte, dst reflect.Value) (bool, error) {
	var result []F

	for len(buf) > 0 {
		v, n := protowire.ConsumeFixed32(buf)
		if n < 0 {
			return false, errors.New("bad protobuf 32-bit value")
		}

		buf = buf[n:]

		result = append(result, F(v))
	}

	*(*[]F)(unsafe.Pointer(dst.UnsafeAddr())) = result

	return true, nil
}

func decodeFixed64[F FixedU64 | FixedS64](buf []byte, dst reflect.Value) (bool, error) {
	var result []F

	for len(buf) > 0 {
		v, n := protowire.ConsumeFixed64(buf)
		if n < 0 {
			return false, errors.New("bad protobuf 64-bit value")
		}

		buf = buf[n:]

		result = append(result, F(v))
	}

	*(*[]F)(unsafe.Pointer(dst.UnsafeAddr())) = result

	return true, nil
}

func decodeFloat32(buf []byte, dst reflect.Value) (bool, error) {
	var result []float32

	for len(buf) > 0 {
		v, n := protowire.ConsumeFixed32(buf)
		if n < 0 {
			return false, errors.New("bad protobuf 32-bit value")
		}

		buf = buf[n:]

		result = append(result, math.Float32frombits(v))
	}

	*(*[]float32)(unsafe.Pointer(dst.UnsafeAddr())) = result

	return true, nil
}

func decodeFloat64(buf []byte, dst reflect.Value) (bool, error) {
	var result []float64

	for len(buf) > 0 {
		v, n := protowire.ConsumeFixed64(buf)
		if n < 0 {
			return false, errors.New("bad protobuf 64-bit value")
		}

		buf = buf[n:]

		result = append(result, math.Float64frombits(v))
	}

	*(*[]float64)(unsafe.Pointer(dst.UnsafeAddr())) = result

	return true, nil
}
