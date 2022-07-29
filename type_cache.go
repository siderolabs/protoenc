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
			if deref(typField.Type).Kind() != reflect.Struct {
				return nil, fmt.Errorf("%s.%s.%s is not a struct type", typ.PkgPath(), typ.Name(), typField.Name)
			}

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

	if len(result) == 0 {
		return nil, fmt.Errorf("%s.%s has no exported fields", typ.PkgPath(), typ.Name())
	}

	return result, nil
}

func deref(typ reflect.Type) reflect.Type {
	for typ.Kind() == reflect.Ptr {
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

var cache = typesCache{}

type typesCache struct {
	m sync.Map
}

func (tc *typesCache) Get(t reflect.Type) ([]FieldData, bool) {
	value, ok := tc.m.Load(t)
	if !ok {
		return nil, false
	}

	return value.([]FieldData), true //nolint:forcetypeassert
}

func (tc *typesCache) Add(t reflect.Type, structFields []FieldData) {
	tc.m.Store(t, structFields)
}
