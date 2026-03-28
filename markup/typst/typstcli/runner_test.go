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
	"context"
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

	err := r.Compile(CompileArgs{
		Input:  InputStdin,
		Output: OutputStdout,
		Format: OutputFormatHTML,
		World: WorldArgs{
			Root:   "/project",
			Inputs: []SysInput{{Key: "b", Value: "2"}, {Key: "a", Value: "1"}},
			Font: FontArgs{
				FontPaths:           []string{"/fonts1", "/fonts2"},
				IgnoreSystemFonts:   true,
				IgnoreEmbeddedFonts: true,
			},
			Package: PackageArgs{
				PackagePath:      "/pkg",
				PackageCachePath: "/pkg-cache",
			},
		},
		Pages: []string{"1-2"},
		Process: ProcessArgs{
			Jobs:             3,
			Features:         []Feature{FeatureHTML},
			DiagnosticFormat: DiagnosticFormatHuman,
		},
	})

	c.Assert(err, qt.IsNil)
	c.Assert(exec.name, qt.Equals, "typst")
	c.Assert(extractStrings(exec.args), qt.DeepEquals, []string{
		"compile",
		"-f", "html",
		"--root", "/project",
		"--input", "b=2",
		"--input", "a=1",
		"--font-path", "/fonts1:/fonts2",
		"--ignore-system-fonts",
		"--ignore-embedded-fonts",
		"--package-path", "/pkg",
		"--package-cache-path", "/pkg-cache",
		"--pages", "1-2",
		"-j", "3",
		"--features", "html",
		"--diagnostic-format", "human",
		"-", "-",
	})
}

func TestRunnerCompileValidation(t *testing.T) {
	c := qt.New(t)
	r := New(&captureExec{}, "typst")

	err := r.Compile(CompileArgs{})
	c.Assert(err, qt.ErrorMatches, "typst compile input is required")

	err = r.Compile(CompileArgs{Input: InputStdin})
	c.Assert(err, qt.ErrorMatches, "typst compile output is required when input is stdin")
}

func TestRunnerQuery(t *testing.T) {
	c := qt.New(t)
	exec := &captureExec{}
	r := New(exec, "typst")

	err := r.Query(QueryArgs{
		Input:    Input("/project/a.typ"),
		Selector: "metadata",
		Field:    "value",
		World: WorldArgs{
			Root:   "/project",
			Inputs: []SysInput{{Key: "k", Value: "v"}},
		},
		Process: ProcessArgs{
			Features: []Feature{FeatureHTML},
		},
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

func TestRunnerQueryValidation(t *testing.T) {
	c := qt.New(t)
	r := New(&captureExec{}, "typst")

	err := r.Query(QueryArgs{Selector: "metadata"})
	c.Assert(err, qt.ErrorMatches, "typst query input is required")

	err = r.Query(QueryArgs{Input: Input("doc.typ")})
	c.Assert(err, qt.ErrorMatches, "typst query selector is required")
}

func TestRunnerRunCustomSubcommand(t *testing.T) {
	c := qt.New(t)
	exec := &captureExec{}
	r := New(exec, "typst")

	err := r.Run(Command{
		Subcommand: "watch",
		Args:       []string{"doc.typ", "--open"},
	})

	c.Assert(err, qt.IsNil)
	c.Assert(extractStrings(exec.args), qt.DeepEquals, []string{"watch", "doc.typ", "--open"})
}

func TestRunnerWatch(t *testing.T) {
	c := qt.New(t)
	exec := &captureExec{}
	r := New(exec, "typst")

	err := r.Watch(WatchArgs{
		Compile: CompileArgs{
			Input:  Input("/project/a.typ"),
			Output: Output("/tmp/a.html"),
			Format: OutputFormatHTML,
			Pages:  []string{"1-3"},
			World: WorldArgs{
				Root: "/project",
			},
			Process: ProcessArgs{
				Features: []Feature{FeatureHTML},
			},
			Exec: ExecOptions{Context: context.Background()},
		},
		Server: ServerArgs{NoServe: true, NoReload: true},
	})

	c.Assert(err, qt.IsNil)
	c.Assert(extractStrings(exec.args), qt.DeepEquals, []string{
		"watch",
		"-f", "html",
		"--root", "/project",
		"--pages", "1-3",
		"--features", "html",
		"--no-serve",
		"--no-reload",
		"/project/a.typ",
		"/tmp/a.html",
	})
}

func TestRunnerWatchValidation(t *testing.T) {
	c := qt.New(t)
	r := New(&captureExec{}, "typst")

	err := r.Watch(WatchArgs{})
	c.Assert(err, qt.ErrorMatches, "typst watch input is required")

	err = r.Watch(WatchArgs{Compile: CompileArgs{Input: InputStdin}})
	c.Assert(err, qt.ErrorMatches, "typst watch output is required when input is stdin")
}

func TestRunnerMissingSubcommand(t *testing.T) {
	c := qt.New(t)
	r := New(&captureExec{}, "typst")

	err := r.Run(Command{})
	c.Assert(err, qt.ErrorMatches, "typst subcommand is required")
}

func TestWorldArgsFromConfig(t *testing.T) {
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

	world := WorldArgsFromConfig(cfg, "")
	process := ProcessArgsFromConfig(cfg)

	c.Assert(world.Root, qt.Equals, "/cfg-root")
	c.Assert(world.Inputs, qt.DeepEquals, []SysInput{{Key: "a", Value: "1"}})
	c.Assert(world.Font.FontPaths, qt.DeepEquals, []string{"/f"})
	c.Assert(world.Package.PackagePath, qt.Equals, "/pkg")
	c.Assert(world.Package.PackageCachePath, qt.Equals, "/cache")
	c.Assert(process.Jobs, qt.Equals, 2)

	cfg.Inputs["a"] = "2"
	cfg.FontPaths[0] = "/changed"
	c.Assert(world.Inputs[0].Value, qt.Equals, "1")
	c.Assert(world.Font.FontPaths[0], qt.Equals, "/f")
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
