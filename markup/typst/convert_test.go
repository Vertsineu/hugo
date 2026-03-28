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

package typst_test

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/gohugoio/hugo/common/hexec"
	"github.com/gohugoio/hugo/common/loggers"
	"github.com/gohugoio/hugo/config/security"
	"github.com/gohugoio/hugo/config/testconfig"
	"github.com/gohugoio/hugo/markup/converter"
	"github.com/gohugoio/hugo/markup/typst"
)

func TestConvert(t *testing.T) {
	if !typst.Supports() {
		t.Skip("typst not installed")
	}

	c := qt.New(t)
	sc := security.DefaultConfig
	sc.Exec.Allow = security.MustNewWhitelist("typst")

	p, err := typst.Provider.New(
		converter.ProviderConfig{
			Conf:   testconfig.GetTestConfig(nil, nil),
			Exec:   hexec.New(sc, "", loggers.NewDefault()),
			Logger: loggers.NewDefault(),
		},
	)
	c.Assert(err, qt.IsNil)

	conv, err := p.New(converter.DocumentContext{Filename: "test.typ", DocumentName: "test.typ"})
	c.Assert(err, qt.IsNil)

	b, err := conv.Convert(converter.RenderContext{Src: []byte("= Test\nHello, Typst!")})
	c.Assert(err, qt.IsNil)
	c.Assert(string(b.Bytes()), qt.Contains, "Test")
	c.Assert(len(b.Bytes()) > 0, qt.Equals, true)
}
