// Copyright 2026 The Hugo Authors. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hugolib

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/gohugoio/hugo/hugolib/sitesmatrix"
)

func TestContentNodeShifterShiftSkipsPageMetaSource(t *testing.T) {
	c := qt.New(t)

	var (
		s  contentNodeShifter
		v  sitesmatrix.Vector
		ms = &pageMetaSource{}
	)

	for _, n := range []contentNode{
		ms,
		contentNodes{ms},
		contentNodesMap{v: ms},
	} {
		got, ok := s.Shift(n, v, false)
		c.Assert(ok, qt.Equals, false)
		c.Assert(got, qt.IsNil)

		got, ok = s.Shift(n, v, true)
		c.Assert(ok, qt.Equals, false)
		c.Assert(got, qt.IsNil)
	}
}
