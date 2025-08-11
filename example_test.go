// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package protoenc_test

import (
	"encoding/hex"
	"fmt"

	"github.com/siderolabs/protoenc"
)

type Test1 struct {
	A uint32 `protobuf:"1"`
}

// https://developers.google.com/protocol-buffers/docs/encoding
func Example_test1() {
	t := Test1{150}
	buf := panicOnErr(protoenc.Marshal(&t))
	fmt.Print(hex.Dump(buf))

	// Output:
	// 00000000  08 96 01                                          |...|
}

type Test2 struct {
	B string `protobuf:"2"`
}

func Example_test2() {
	t := Test2{B: "testing"}
	buf := panicOnErr(protoenc.Marshal(&t))
	fmt.Print(hex.Dump(buf))

	// Output:
	// 00000000  12 07 74 65 73 74 69 6e  67                       |..testing|
}

type Test3 struct {
	C Test1 `protobuf:"3"`
}

func Example_test3() {
	t := Test3{C: Test1{150}}
	buf := panicOnErr(protoenc.Marshal(&t))
	fmt.Print(hex.Dump(buf))

	// Output:
	// 00000000  1a 03 08 96 01                                    |.....|
}

//nolint:govet
type WithRepeatedString struct {
	First int      `protobuf:"1"`
	Slc   []string `protobuf:"2"`
	Other string   `protobuf:"3"`
}

//nolint:govet
type WithString struct {
	First int    `protobuf:"1"`
	Str   string `protobuf:"2"`
	Last  string `protobuf:"3"`
}

func Example_repeatedToSingleAndBack() { //nolint:govet
	withString := WithString{
		First: 9,
		Str:   "a",
		Last:  "e",
	}
	buf := panicOnErr(protoenc.Marshal(&withString))

	var withRepeated WithRepeatedString

	err := protoenc.Unmarshal(buf, &withRepeated)
	if err != nil {
		panic(err)
	}

	fmt.Printf("withRepeated = %+v\n", withRepeated)

	withRepeated.Slc = append(withRepeated.Slc, "d")

	buf = panicOnErr(protoenc.Marshal(&withRepeated))

	err = protoenc.Unmarshal(buf, &withString)
	if err != nil {
		panic(err)
	}

	fmt.Printf("withString = %+v\n", withString)

	// Output:
	// withRepeated = {First:9 Slc:[a] Other:e}
	// withString = {First:9 Str:d Last:e}
}
