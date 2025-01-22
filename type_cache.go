// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package protoenc

import (
	"encoding"
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

	if hasCustomEncoders(typ) {
		return nil, nil
	}

	if ok, err := isBinaryMarshaler(typ); ok {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	fields, err := structFields(typ)
	if err != nil {
		return nil, err
	}

	if num := findFirstDuplicate(fields); num != 0 {
		return nil, fmt.Errorf("protobuf num %d duplicated in %s.%s", num, typ.PkgPath(), typ.Name())
	}

	cache.Add(typ, fields)

	err = validateStruct(deref(typ))
	if err != nil {
		cache.Remove(typ)

		return nil, err
	}

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

	return value.(V), true //nolint:forcetypeassert,errcheck
}

func (sm *syncMap[K, V]) Add(k K, v V) { sm.m.Store(k, v) }

func (sm *syncMap[K, V]) Remove(k K) { sm.m.Delete(k) }

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

func validateStruct(typ reflect.Type) error {
	if typ.Kind() != reflect.Struct {
		return fmt.Errorf("%s is not a struct", typ)
	}

	if hasCustomEncoders(typ) {
		return nil
	}

	if ok, err := isBinaryMarshaler(typ); ok {
		return nil
	} else if err != nil {
		return err
	}

	hasExportedFields := false

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)

		if isEmbeddedField(field) {
			return fmt.Errorf("cannot embed %s.%s.%s: not a struct type", typ.PkgPath(), typ.Name(), field.Name)
		}

		if !field.IsExported() {
			continue
		}

		hasExportedFields = true

		if err := validateField(field); err != nil {
			return fmt.Errorf("%s.%s.%s: %w", typ.PkgPath(), typ.Name(), field.Name, err)
		}
	}

	if !hasExportedFields && typ.NumField() > 0 {
		return fmt.Errorf("%s.%s has only private fields", typ.PkgPath(), typ.Name())
	}

	return nil
}

func isEmbeddedField(field reflect.StructField) bool {
	return field.Anonymous && deref(field.Type).Kind() != reflect.Struct
}

func validateField(field reflect.StructField) error {
	if hasCustomEncoders(field.Type) {
		return nil
	}

	if ok, err := isBinaryMarshaler(field.Type); ok {
		return nil
	} else if err != nil {
		return err
	}

	switch field.Type.Kind() { //nolint:exhaustive
	case reflect.Struct:
		_, err := StructFields(field.Type)

		return err

	case reflect.Array:
		return validateArray(field.Type)

	case reflect.Slice:
		return validateSlice(field.Type)

	case reflect.Pointer:
		return validateFieldPtr(field.Type)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64, reflect.Bool, reflect.String:
		return nil

	case reflect.Map:
		return validateMap(field.Type)

	default:
		return fmt.Errorf("unsupported type '%s'", field.Type)
	}
}

var (
	binaryMarshaler   = reflect.TypeOf((*encoding.BinaryMarshaler)(nil)).Elem()
	binaryUnmarshaler = reflect.TypeOf((*encoding.BinaryUnmarshaler)(nil)).Elem()
)

func isBinaryMarshaler(typ reflect.Type) (bool, error) {
	isBinaryMarshaler := typ.Implements(binaryMarshaler) || reflect.PointerTo(typ).Implements(binaryMarshaler)
	isBinaryUnmarshaller := typ.Implements(binaryUnmarshaler) || reflect.PointerTo(typ).Implements(binaryUnmarshaler)

	if isBinaryMarshaler != isBinaryUnmarshaller {
		return false, fmt.Errorf("'%s' should implement both marshaller and unmarshaller", typ)
	}

	return isBinaryMarshaler && isBinaryUnmarshaller, nil
}

func hasCustomEncoders(typ reflect.Type) bool {
	_, ok := encoders.Get(typ)

	return ok
}

func validateArray(typ reflect.Type) error {
	if typ.Elem().Kind() == reflect.Uint8 {
		return nil
	}

	return fmt.Errorf("%s is not supported", typ)
}

func validateSlice(typ reflect.Type) error {
	elem := typ.Elem()

	switch elem.Kind() { //nolint:exhaustive
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64, reflect.Bool, reflect.String:
		return nil

	case reflect.Struct:
		_, err := StructFields(elem)
		if err != nil {
			return err
		}

		return nil

	case reflect.Array, reflect.Slice:
		if elem.Elem().Kind() == reflect.Uint8 {
			return nil
		}

		return fmt.Errorf("'%s' is not supported", typ)

	case reflect.Pointer:
		if err := validatePtrElem(deref(elem)); err != nil {
			return err
		}

		return nil

	default:
		return fmt.Errorf("'%s' is not supported", typ)
	}
}

func validatePtrElem(elemTyp reflect.Type) error {
	switch elemTyp.Kind() { //nolint:exhaustive
	case reflect.Struct, reflect.String:
		return nil

	case reflect.Slice, reflect.Array:
		if elemTyp.Elem().Kind() == reflect.Uint8 {
			return nil
		}

		fallthrough

	default:
		return fmt.Errorf("'%s' is not supported", elemTyp)
	}
}

func validateFieldPtr(typ reflect.Type) error {
	elem := typ.Elem()

	switch elem.Kind() { //nolint:exhaustive
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64, reflect.Bool, reflect.String:
		return nil

	case reflect.Pointer:
		return validateFieldPtr(indirect(elem))

	case reflect.Struct:
		_, err := StructFields(elem)
		if err != nil {
			return err
		}

		return nil

	case reflect.Array:
		return validateArray(elem)

	case reflect.Slice:
		return validateSlice(elem)

	case reflect.Map:
		return validateMap(elem)

	default:
		return fmt.Errorf("'%s' is not supported", typ)
	}
}

func validateMap(elem reflect.Type) error {
	if elem == typeMapInterface {
		return nil
	}

	if err := validateMapKey(elem.Key()); err != nil {
		return err
	}

	if err := validateMapValue(elem.Elem()); err != nil {
		return err
	}

	return nil
}

func validateMapKey(key reflect.Type) error {
	switch key.Kind() { //nolint:exhaustive
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64, reflect.Bool, reflect.String:
		return nil

	default:
		return fmt.Errorf("'%s' is not supported", key)
	}
}

func validateMapValue(val reflect.Type) error {
	switch val.Kind() { //nolint:exhaustive
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64, reflect.Bool, reflect.String:
		return nil

	case reflect.Struct:
		_, err := StructFields(val)
		if err != nil {
			return err
		}

		return nil
	case reflect.Array, reflect.Slice:
		return validateArray(val)

	case reflect.Pointer:
		return validateMapValue(indirect(val))

	default:
		return fmt.Errorf("'%s' is not supported", val)
	}
}
