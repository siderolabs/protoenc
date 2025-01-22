// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package messages_test

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/protoenc"
)

func FuzzBasicMessage(f *testing.F) {
	testcases := [][]byte{
		hexToBytes(f, "08 01 18 02 29 03 00 00 00 00 00 00 00 32 0b 73 6f 6d 65 20 73 74 72 69  6e 67 3a 0a 73 6f 6d 65 20 62 79 74 65 73"),
		hexToBytes(f, "08 00 18 00 29 00 00 00  00 00 00 00 00 32 00 3a 0a 73 6f 6d 65 20 62 79  74 65 73"),
	}
	for _, tc := range testcases {
		f.Add(tc) // Use f.Add to provide a seed corpus
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		var ourBasicMessage BasicMessage

		err := protoenc.Unmarshal(data, &ourBasicMessage)
		if err != nil {
			errText := err.Error()

			switch {
			case strings.Contains(errText, "index out of range"),
				strings.Contains(errText, "assignment to entry in nil map"),
				strings.Contains(errText, "invalid memory address or nil pointer dereference"):
				t.FailNow()
			}
		}
	})
}

// hexToBytes converts a hex string to a byte slice, removing any whitespace.
func hexToBytes(f *testing.F, s string) []byte {
	f.Helper()

	s = strings.ReplaceAll(s, "|", "")
	s = strings.ReplaceAll(s, "[", "")
	s = strings.ReplaceAll(s, "]", "")
	s = strings.ReplaceAll(s, " ", "")

	b, err := hex.DecodeString(s)
	require.NoError(f, err)

	return b
}
