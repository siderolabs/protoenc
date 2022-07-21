// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package protoenc_test

import (
	"encoding/hex"
	"fmt"
	"reflect"

	"github.com/siderolabs/protoenc"
)

// Go-based protobuf definition for the example Person message format
//nolint:govet
type Person struct {
	Name  string        `protobuf:"1"`
	ID    int32         `protobuf:"2"`
	Email *string       `protobuf:"3"`
	Phone []PhoneNumber `protobuf:"4"`
}

type PhoneType uint32

const (
	MOBILE PhoneType = iota
	HOME
	WORK
)

//nolint:govet
type PhoneNumber struct {
	Number string     `protobuf:"1"`
	Type   *PhoneType `protobuf:"2"`
}

// This example defines, encodes, and decodes a Person message format
// equivalent to the example used in the Protocol Buffers overview.
//nolint:nosnakecase
func Example_protobuf() {
	// Create a Person record
	email := "alice@somewhere"
	ptype := WORK
	person := Person{
		"Alice",
		123,
		&email,
		[]PhoneNumber{
			{"111-222-3333", nil},
			{"444-555-6666", &ptype},
		},
	}

	// Encode it
	buf, err := protoenc.Marshal(&person)
	if err != nil {
		panic("Encode failed: " + err.Error())
	}

	fmt.Print(hex.Dump(buf))

	// Decode it
	person2 := Person{}
	if err := protoenc.Unmarshal(buf, &person2); err != nil {
		panic("Decode failed")
	}

	if !reflect.DeepEqual(person, person2) {
		panic("Decode failed")
	}

	// Output:
	// 00000000  0a 05 41 6c 69 63 65 10  7b 1a 0f 61 6c 69 63 65  |..Alice.{..alice|
	// 00000010  40 73 6f 6d 65 77 68 65  72 65 22 0e 0a 0c 31 31  |@somewhere"...11|
	// 00000020  31 2d 32 32 32 2d 33 33  33 33 22 10 0a 0c 34 34  |1-222-3333"...44|
	// 00000030  34 2d 35 35 35 2d 36 36  36 36 10 02              |4-555-6666..|
}
