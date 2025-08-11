// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package protoenc_test

import (
	"fmt"
	"os"
	"testing"
	"time"
)

var Seed = uint64(time.Now().UnixNano())

func TestMain(m *testing.M) {
	fmt.Println("Seed:", Seed)
	os.Exit(m.Run())
}
