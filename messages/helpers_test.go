// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package messages_test

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/siderolabs/protoenc"
)

func shouldBeEqual[T any](t *testing.T, left, right T) {
	t.Helper()

	opts := makeOpts[T]()

	if !cmp.Equal(left, right, opts...) {
		t.Log(cmp.Diff(left, right, opts...))
		t.FailNow()
	}
}

func makeOpts[T any]() []cmp.Option {
	var zero T

	typ := reflect.TypeOf(zero)
	for typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	if typ.Kind() == reflect.Struct {
		return []cmp.Option{cmpopts.IgnoreUnexported(reflect.New(typ).Elem().Interface())}
	}

	return nil
}

type msg[T any] interface {
	*T
	proto.Message
}

func runTestPipe[R any, RP msg[R], T any](t *testing.T, original T) {
	encoded1 := must(protoenc.Marshal(&original))(t)
	decoded := protoUnmarshal[R, RP](t, encoded1)
	encoded2 := must(proto.Marshal(decoded))(t)
	result := ourUnmarshal[T](t, encoded2)

	shouldBeEqual(t, original, result)
}

func protoUnmarshal[T any, V msg[T]](t *testing.T, data []byte) V {
	t.Helper()

	var msg T

	err := proto.UnmarshalOptions{DiscardUnknown: true}.Unmarshal(data, V(&msg))

	require.NoError(t, err)

	return &msg
}

func ourUnmarshal[T any](t *testing.T, data []byte) T {
	t.Helper()

	var msg T

	err := protoenc.Unmarshal(data, &msg)
	require.NoError(t, err)

	return msg
}

func must[T any](v T, err error) func(t *testing.T) T {
	return func(t *testing.T) T {
		t.Helper()
		require.NoError(t, err)

		return v
	}
}
