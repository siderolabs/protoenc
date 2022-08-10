// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package protoenc

import (
	"reflect"
	"time"
)

type (
	// FixedU32 fields will be encoded as fixed 64-bit unsigned integers.
	FixedU32 uint32

	// FixedU64 fields will be encoded as fixed 64-bit unsigned integers.
	FixedU64 uint64

	// FixedS32 fields will be encoded as fixed 32-bit signed integers.
	FixedS32 int32

	// FixedS64 fields will be encoded as fixed 64-bit signed integers.
	FixedS64 int64
)

var (
	typeFixedU32     = typeOf[FixedU32]()
	typeFixedU64     = typeOf[FixedU64]()
	typeFixedS32     = typeOf[FixedS32]()
	typeFixedS64     = typeOf[FixedS64]()
	typeTime         = typeOf[time.Time]()
	typeDuration     = typeOf[time.Duration]()
	typeMapInterface = typeOf[map[string]interface{}]()
	typeFixedU32s    = reflect.SliceOf(typeFixedU32)
	typeFixedU64s    = reflect.SliceOf(typeFixedU64)
	typeFixedS32s    = reflect.SliceOf(typeFixedS32)
	typeFixedS64s    = reflect.SliceOf(typeFixedS64)
	typeDurations    = reflect.SliceOf(typeDuration)
	typeByte         = typeOf[byte]()
)

func typeOf[T any]() reflect.Type {
	var zero T

	return reflect.TypeOf(zero)
}
