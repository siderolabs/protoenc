// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package protoenc_test

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func must[T any](v T, err error) func(t *testing.T) T {
	return func(t *testing.T) T {
		t.Helper()
		require.NoError(t, err)

		return v
	}
}

func panicOnErr[T any](t T, err error) T {
	if err != nil {
		panic(err)
	}

	return t
}

func ptr[T any](t T) *T {
	return &t
}
