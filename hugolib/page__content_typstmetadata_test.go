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
	"github.com/gohugoio/hugo/markup/typst/typstcli"
)

func TestDecodeTypstQueryMetadata(t *testing.T) {
	c := qt.New(t)

	c.Run("FirstElement", func(c *qt.C) {
		in := []byte(`[{"title":"Hello Hugo","date":"2026-03-28","draft":true},{"title":"Other"}]`)
		m, err := decodeTypstQueryMetadata(in)
		c.Assert(err, qt.IsNil)
		c.Assert(m["title"], qt.Equals, "Hello Hugo")
		c.Assert(m["draft"], qt.Equals, true)
		c.Assert(m["date"], qt.Equals, "2026-03-28")
	})

	c.Run("Empty", func(c *qt.C) {
		m, err := decodeTypstQueryMetadata([]byte(`[]`))
		c.Assert(err, qt.IsNil)
		c.Assert(m, qt.IsNil)
	})
}

func TestWithTypstMetadataQueryInput(t *testing.T) {
	c := qt.New(t)

	c.Run("AddMissing", func(c *qt.C) {
		world := typstcli.WorldArgs{
			Inputs: []typstcli.SysInput{{Key: "lang", Value: "en"}},
		}

		world = withTypstMetadataQueryInput(world)
		c.Assert(world.Inputs, qt.DeepEquals, []typstcli.SysInput{
			{Key: "lang", Value: "en"},
			{Key: "query", Value: "prelude"},
		})
	})

	c.Run("KeepExisting", func(c *qt.C) {
		world := typstcli.WorldArgs{
			Inputs: []typstcli.SysInput{
				{Key: "query", Value: "custom"},
				{Key: "lang", Value: "en"},
			},
		}

		world = withTypstMetadataQueryInput(world)
		c.Assert(world.Inputs, qt.DeepEquals, []typstcli.SysInput{
			{Key: "query", Value: "custom"},
			{Key: "lang", Value: "en"},
		})
	})
}
