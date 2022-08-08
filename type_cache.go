// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package protoenc

import (
	"fmt"
	"reflect"
	"strconv"
	"sync"

	"google.golang.org/protobuf/encoding/protowire"
)

// StructFields returns a list of StructFields for the given struct type.
func StructFields(typ reflect.Type) ([]FieldData, error) {
	result, ok := cache.Get(typ)
	if ok {
		return result, nil
	}

	fields, err := structFields(typ)
	if err != nil {
		return nil, err
	}

	if num := findFirstDuplicate(fields); num != 0 {
		return nil, fmt.Errorf("protobuf num %d duplicated in %s.%s", num, typ.PkgPath(), typ.Name())
	}

	cache.Add(typ, fields)

	return fields, nil
}

// FieldData represents a field of a struct with proto number and field index.
//
//nolint:govet
type FieldData struct {
	Num        protowire.Number
	FieldIndex []int
	Field      reflect.StructField
}

// IsZero returns true if FieldData is zero.
func (fd *FieldData) IsZero() bool {
	return fd.Num == 0
}

func structFields(typ reflect.Type) ([]FieldData, error) {
	typ = deref(typ)
	if typ.Kind() != reflect.Struct {
		return nil, fmt.Errorf("%s is not a struct", typ)
	}

	var result []FieldData

	for i := 0; i < typ.NumField(); i++ {
		typField := typ.Field(i)

		// Report error sooner than later
		if typField.Anonymous && deref(typField.Type).Kind() != reflect.Struct {
			return nil, fmt.Errorf("%s.%s.%s is not a struct type", typ.PkgPath(), typ.Name(), typField.Name)
		}

		// Skipping private types
		if !typField.IsExported() {
			continue
		}

		if typField.Anonymous {
			fields, err := structFields(typField.Type)
			if err != nil {
				return nil, err
			}

			for _, innerField := range fields {
				innerField.FieldIndex = append([]int{i}, innerField.FieldIndex...)
				result = append(result, innerField)
			}

			continue
		}

		num := ParseTag(typField)
		switch num {
		case 0:
			// Skipping explicitly ignored fields
			continue
		case -1:
			return nil, fmt.Errorf("%s.%s.%s has empty protobuf tag", typ.PkgPath(), typ.Name(), typField.Name)
		case -2:
			return nil, fmt.Errorf("%s.%s.%s has invalid protobuf tag", typ.PkgPath(), typ.Name(), typField.Name)
		}

		result = append(result, FieldData{
			Num:        protowire.Number(num),
			FieldIndex: []int{i},
			Field:      typField,
		})
	}

	return result, nil
}

func deref(typ reflect.Type) reflect.Type {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}

	return typ
}

func findFirstDuplicate(fields []FieldData) protowire.Number {
	exists := map[protowire.Number]struct{}{}

	for _, field := range fields {
		if _, ok := exists[field.Num]; ok {
			return field.Num
		}

		exists[field.Num] = struct{}{}
	}

	return 0
}

// ParseTag parses the protobuf tag of the given struct field.
func ParseTag(field reflect.StructField) int {
	tag := field.Tag.Get("protobuf")
	if tag == "" {
		return -1
	}

	if tag == "-" {
		return 0
	}

	num, err := strconv.Atoi(tag)
	if err != nil {
		return -2
	}

	return num
}

type syncMap[K, V any] struct {
	m sync.Map
}

func (sm *syncMap[K, V]) Get(k K) (V, bool) {
	value, ok := sm.m.Load(k)
	if !ok {
		var zero V

		return zero, false
	}

	return value.(V), true //nolint:forcetypeassert
}

func (sm *syncMap[K, V]) Add(k K, v V) {
	sm.m.Store(k, v)
}

var (
	cache    = syncMap[reflect.Type, []FieldData]{}
	encoders = syncMap[reflect.Type, encoder]{}
	decoders = syncMap[reflect.Type, decoder]{}
)

type (
	encoder func(any) ([]byte, error)
	decoder func(slc []byte, dst reflect.Value) error
)

// RegisterEncoderDecoder registers the given encoder and decoder for the given type. T should be struct or
// pointer to struct.
func RegisterEncoderDecoder[T any, Enc func(T) ([]byte, error), Dec func([]byte) (T, error)](enc Enc, dec Dec) {
	var zero T

	typ := reflect.TypeOf(zero)
	if indirect(typ).Kind() != reflect.Struct {
		panic("RegisterEncoderDecoder: T must be struct or pointer to struct")
	}

	fnEnc := func(val any) ([]byte, error) {
		v, ok := val.(T)
		if !ok {
			return nil, fmt.Errorf("%T is not %T", val, zero)
		}

		return enc(v)
	}

	fnDec := func(b []byte, dst reflect.Value) error {
		if dst.Type() != typ {
			return fmt.Errorf("%T is not %T", dst, zero)
		}

		v, err := dec(b)
		if err != nil {
			return err
		}

		*(*T)(dst.Addr().UnsafePointer()) = v

		return nil
	}

	if _, ok := encoders.Get(typ); ok {
		panic("RegisterEncoderDecoder: encoder for type " + typ.String() + " already registered")
	}

	if _, ok := decoders.Get(typ); ok {
		panic("RegisterEncoderDecoder: decoder for type " + typ.String() + " already registered")
	}

	encoders.Add(typ, fnEnc)
	decoders.Add(typ, fnDec)
}

func indirect(typ reflect.Type) reflect.Type {
	if typ.Kind() == reflect.Pointer {
		return typ.Elem()
	}

	return typ
}

// CleanEncoderDecoder cleans the map of encoders and decoders. It's not safe to it call concurrently.
func CleanEncoderDecoder() {
	encoders = syncMap[reflect.Type, encoder]{}
	decoders = syncMap[reflect.Type, decoder]{}
}
