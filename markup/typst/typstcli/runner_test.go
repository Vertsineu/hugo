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

package typstcli

import (
	"errors"
	"io"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/gohugoio/hugo/common/hexec"
	"github.com/gohugoio/hugo/markup/typst/typst_config"
)

func TestRunnerCompile(t *testing.T) {
	c := qt.New(t)
	exec := &captureExec{}
	r := New(exec, "typst")

	err := r.Compile(CompileOptions{
		CommonOptions: CommonOptions{
			Root:                "/project",
			Inputs:              map[string]string{"b": "2", "a": "1"},
			FontPaths:           []string{"/fonts1", "/fonts2"},
			IgnoreSystemFonts:   true,
			IgnoreEmbeddedFonts: true,
			PackagePath:         "/pkg",
			PackageCachePath:    "/pkg-cache",
			Jobs:                3,
		},
		Pages:  "1-2",
		Format: "html",
	})

	c.Assert(err, qt.IsNil)
	c.Assert(exec.name, qt.Equals, "typst")
	c.Assert(extractStrings(exec.args), qt.DeepEquals, []string{
		"compile",
		"--root", "/project",
		"--input", "a=1",
		"--input", "b=2",
		"--font-path", "/fonts1:/fonts2",
		"--ignore-system-fonts",
		"--ignore-embedded-fonts",
		"--package-path", "/pkg",
		"--package-cache-path", "/pkg-cache",
		"-j", "3",
		"--pages", "1-2",
		"-f", "html",
		"--features", "html",
		"--diagnostic-format", "human",
		"-", "-",
	})
}

func TestRunnerCompileAddsFeaturesHTML(t *testing.T) {
	c := qt.New(t)
	exec := &captureExec{}
	r := New(exec, "typst")

	err := r.Compile(CompileOptions{Format: "pdf"})
	c.Assert(err, qt.IsNil)
	c.Assert(extractStrings(exec.args), qt.DeepEquals, []string{
		"compile",
		"-f", "pdf",
		"--features", "html",
		"--diagnostic-format", "human",
		"-", "-",
	})
}

func TestRunnerQuery(t *testing.T) {
	c := qt.New(t)
	exec := &captureExec{}
	r := New(exec, "typst")

	err := r.Query(QueryOptions{
		CommonOptions: CommonOptions{
			Root:   "/project",
			Inputs: map[string]string{"k": "v"},
		},
		Input:    "/project/a.typ",
		Selector: "metadata",
		Field:    "value",
	})

	c.Assert(err, qt.IsNil)
	c.Assert(extractStrings(exec.args), qt.DeepEquals, []string{
		"query",
		"/project/a.typ",
		"metadata",
		"--field", "value",
		"--root", "/project",
		"--input", "k=v",
		"--features", "html",
	})
}

func TestRunnerRunCustomSubcommand(t *testing.T) {
	c := qt.New(t)
	exec := &captureExec{}
	r := New(exec, "typst")

	err := r.Run(CommandOptions{
		Subcommand: "watch",
		Args:       []string{"doc.typ"},
		TailArgs:   []string{"--open"},
	})

	c.Assert(err, qt.IsNil)
	c.Assert(extractStrings(exec.args), qt.DeepEquals, []string{"watch", "doc.typ", "--open"})
}

func TestRunnerMissingSubcommand(t *testing.T) {
	c := qt.New(t)
	r := New(&captureExec{}, "typst")

	err := r.Run(CommandOptions{})
	c.Assert(err, qt.ErrorMatches, "typst subcommand is required")
}

func TestCommonOptionsFromConfig(t *testing.T) {
	c := qt.New(t)

	cfg := typst_config.Config{
		Root:                "/cfg-root",
		Inputs:              map[string]string{"a": "1"},
		FontPaths:           []string{"/f"},
		IgnoreSystemFonts:   true,
		IgnoreEmbeddedFonts: true,
		PackagePath:         "/pkg",
		PackageCachePath:    "/cache",
		Jobs:                2,
	}

	common := CommonOptionsFromConfig(cfg, "")
	c.Assert(common.Root, qt.Equals, "/cfg-root")
	c.Assert(common.Inputs, qt.DeepEquals, map[string]string{"a": "1"})
	c.Assert(common.FontPaths, qt.DeepEquals, []string{"/f"})

	cfg.Inputs["a"] = "2"
	cfg.FontPaths[0] = "/changed"
	c.Assert(common.Inputs["a"], qt.Equals, "1")
	c.Assert(common.FontPaths[0], qt.Equals, "/f")
}

type captureExec struct {
	name string
	args []any
}

func (e *captureExec) New(name string, arg ...any) (hexec.Runner, error) {
	e.name = name
	e.args = arg
	return captureRunner{}, nil
}

type captureRunner struct{}

func (captureRunner) Run() error {
	return nil
}

func (captureRunner) StdinPipe() (io.WriteCloser, error) {
	return nil, errors.New("not implemented")
}

func extractStrings(args []any) []string {
	var ss []string
	for _, v := range args {
		s, ok := v.(string)
		if ok {
			ss = append(ss, s)
		}
	}
	return ss
}
