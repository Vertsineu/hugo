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
	"maps"
	"os"
	"slices"
	"strconv"

	"github.com/gohugoio/hugo/common/collections"
	"github.com/gohugoio/hugo/common/hexec"
	"github.com/gohugoio/hugo/markup/typst/typst_config"
)

type commandFactory interface {
	New(name string, arg ...any) (hexec.Runner, error)
}

type Runner struct {
	exec   commandFactory
	binary string
}

func New(exec commandFactory, binary string) Runner {
	if binary == "" {
		binary = typst_config.Default.Binary
	}
	return Runner{exec: exec, binary: binary}
}

type CommonOptions struct {
	Root                string
	Inputs              map[string]string
	FontPaths           []string
	IgnoreSystemFonts   bool
	IgnoreEmbeddedFonts bool
	PackagePath         string
	PackageCachePath    string
	Jobs                int
}

type CommandOptions struct {
	Subcommand string

	// Args are added right after Subcommand.
	Args []string

	Common CommonOptions

	// TailArgs are added after Common options.
	TailArgs []string

	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

type CompileOptions struct {
	CommonOptions

	Format string
	Pages  string

	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

type QueryOptions struct {
	CommonOptions

	Input    string
	Selector string
	Field    string

	Stdout io.Writer
	Stderr io.Writer
}

func CommonOptionsFromConfig(cfg typst_config.Config, root string) CommonOptions {
	if root == "" {
		root = cfg.Root
	}

	inputs := make(map[string]string, len(cfg.Inputs))
	for k, v := range cfg.Inputs {
		inputs[k] = v
	}

	fontPaths := append([]string(nil), cfg.FontPaths...)

	return CommonOptions{
		Root:                root,
		Inputs:              inputs,
		FontPaths:           fontPaths,
		IgnoreSystemFonts:   cfg.IgnoreSystemFonts,
		IgnoreEmbeddedFonts: cfg.IgnoreEmbeddedFonts,
		PackagePath:         cfg.PackagePath,
		PackageCachePath:    cfg.PackageCachePath,
		Jobs:                cfg.Jobs,
	}
}

func (r Runner) Run(opts CommandOptions) error {
	if opts.Subcommand == "" {
		return errors.New("typst subcommand is required")
	}

	args := make([]string, 0, 1+len(opts.Args)+len(opts.TailArgs)+16)
	args = append(args, opts.Subcommand)
	args = append(args, opts.Args...)
	args = appendCommonArgs(args, opts.Common)
	args = append(args, opts.TailArgs...)

	argv := collections.StringSliceToInterfaceSlice(args)
	if opts.Stdin != nil {
		argv = append(argv, hexec.WithStdin(opts.Stdin))
	}
	if opts.Stdout != nil {
		argv = append(argv, hexec.WithStdout(opts.Stdout))
	}
	if opts.Stderr != nil {
		argv = append(argv, hexec.WithStderr(opts.Stderr))
	}

	cmd, err := r.exec.New(r.binary, argv...)
	if err != nil {
		return err
	}

	return cmd.Run()
}

func (r Runner) Compile(opts CompileOptions) error {
	tailArgs := make([]string, 0, 10)
	if opts.Pages != "" {
		tailArgs = append(tailArgs, "--pages", opts.Pages)
	}
	if opts.Format != "" {
		tailArgs = append(tailArgs, "-f", opts.Format)
		if opts.Format == "html" {
			// HTML export is behind this feature flag for some Typst versions.
			tailArgs = append(tailArgs, "--features", "html")
		}
	}

	tailArgs = append(tailArgs,
		"--diagnostic-format", "human",
		"-", "-",
	)

	return r.Run(CommandOptions{
		Subcommand: "compile",
		Common:     opts.CommonOptions,
		TailArgs:   tailArgs,
		Stdin:      opts.Stdin,
		Stdout:     opts.Stdout,
		Stderr:     opts.Stderr,
	})
}

func (r Runner) Query(opts QueryOptions) error {
	if opts.Input == "" {
		return errors.New("typst query input is required")
	}
	if opts.Selector == "" {
		return errors.New("typst query selector is required")
	}

	args := []string{opts.Input, opts.Selector}
	if opts.Field != "" {
		args = append(args, "--field", opts.Field)
	}

	return r.Run(CommandOptions{
		Subcommand: "query",
		Args:       args,
		Common:     opts.CommonOptions,
		Stdout:     opts.Stdout,
		Stderr:     opts.Stderr,
	})
}

func appendCommonArgs(args []string, opts CommonOptions) []string {
	if opts.Root != "" {
		args = append(args, "--root", opts.Root)
	}

	if len(opts.Inputs) > 0 {
		for _, k := range slices.Sorted(maps.Keys(opts.Inputs)) {
			args = append(args, "--input", k+"="+opts.Inputs[k])
		}
	}

	if len(opts.FontPaths) > 0 {
		var paths string
		for i, path := range opts.FontPaths {
			if i > 0 {
				paths += string(os.PathListSeparator)
			}
			paths += path
		}
		args = append(args, "--font-path", paths)
	}

	if opts.IgnoreSystemFonts {
		args = append(args, "--ignore-system-fonts")
	}

	if opts.IgnoreEmbeddedFonts {
		args = append(args, "--ignore-embedded-fonts")
	}

	if opts.PackagePath != "" {
		args = append(args, "--package-path", opts.PackagePath)
	}

	if opts.PackageCachePath != "" {
		args = append(args, "--package-cache-path", opts.PackageCachePath)
	}

	if opts.Jobs > 0 {
		args = append(args, "-j", strconv.Itoa(opts.Jobs))
	}

	return args
}
