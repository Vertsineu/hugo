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
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"

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

type Input string

const InputStdin Input = "-"

func (i Input) IsZero() bool {
	return i == ""
}

func (i Input) IsStdin() bool {
	return i == InputStdin
}

func (i Input) String() string {
	return string(i)
}

type Output string

const OutputStdout Output = "-"

func (o Output) IsZero() bool {
	return o == ""
}

func (o Output) String() string {
	return string(o)
}

type OutputFormat string

const (
	OutputFormatPDF  OutputFormat = "pdf"
	OutputFormatPNG  OutputFormat = "png"
	OutputFormatSVG  OutputFormat = "svg"
	OutputFormatHTML OutputFormat = "html"
)

type DepsFormat string

const (
	DepsFormatJSON DepsFormat = "json"
	DepsFormatZero DepsFormat = "zero"
	DepsFormatMake DepsFormat = "make"
)

type Target string

const (
	TargetPaged Target = "paged"
	TargetHTML  Target = "html"
)

type DiagnosticFormat string

const (
	DiagnosticFormatHuman DiagnosticFormat = "human"
	DiagnosticFormatShort DiagnosticFormat = "short"
)

type Feature string

const (
	FeatureHTML       Feature = "html"
	FeatureBundle     Feature = "bundle"
	FeatureA11yExtras Feature = "a11y-extras"
)

type SerializationFormat string

const (
	SerializationFormatJSON SerializationFormat = "json"
	SerializationFormatYAML SerializationFormat = "yaml"
)

type SysInput struct {
	Key   string
	Value string
}

type ExecOptions struct {
	Context context.Context
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
}

type Command struct {
	Subcommand string
	Args       []string
	Exec       ExecOptions
}

type WorldArgs struct {
	Root              string
	Inputs            []SysInput
	Font              FontArgs
	Package           PackageArgs
	CreationTimestamp string
}

type FontArgs struct {
	FontPaths           []string
	IgnoreSystemFonts   bool
	IgnoreEmbeddedFonts bool
}

type PackageArgs struct {
	PackagePath      string
	PackageCachePath string
}

type ProcessArgs struct {
	Jobs             int
	Features         []Feature
	DiagnosticFormat DiagnosticFormat
}

type CompileArgs struct {
	Input  Input
	Output Output

	Format      OutputFormat
	World       WorldArgs
	Pages       []string
	PDFStandard []string
	NoPDFTags   bool
	PPI         float32
	Deps        Output
	DepsFormat  DepsFormat
	Process     ProcessArgs

	Open *string

	Timings *string

	Exec ExecOptions
}

type ServerArgs struct {
	NoServe  bool
	NoReload bool
	Port     int
}

type WatchArgs struct {
	Compile CompileArgs
	Server  ServerArgs
}

type QueryArgs struct {
	Input    Input
	Selector string
	Field    string
	One      bool
	Format   SerializationFormat
	Pretty   bool
	Target   Target

	World   WorldArgs
	Process ProcessArgs

	Exec ExecOptions
}

func WorldArgsFromConfig(cfg typst_config.Config, root string) WorldArgs {
	if root == "" {
		root = cfg.Root
	}

	inputs := make([]SysInput, 0, len(cfg.Inputs))
	for _, k := range slices.Sorted(maps.Keys(cfg.Inputs)) {
		inputs = append(inputs, SysInput{Key: k, Value: cfg.Inputs[k]})
	}

	fontPaths := append([]string(nil), cfg.FontPaths...)

	return WorldArgs{
		Root:   root,
		Inputs: inputs,
		Font: FontArgs{
			FontPaths:           fontPaths,
			IgnoreSystemFonts:   cfg.IgnoreSystemFonts,
			IgnoreEmbeddedFonts: cfg.IgnoreEmbeddedFonts,
		},
		Package: PackageArgs{
			PackagePath:      cfg.PackagePath,
			PackageCachePath: cfg.PackageCachePath,
		},
	}
}

func ProcessArgsFromConfig(cfg typst_config.Config) ProcessArgs {
	return ProcessArgs{Jobs: cfg.Jobs}
}

func (r Runner) Run(cmd Command) error {
	if cmd.Subcommand == "" {
		return errors.New("typst subcommand is required")
	}

	args := make([]string, 0, 1+len(cmd.Args))
	args = append(args, cmd.Subcommand)
	args = append(args, cmd.Args...)

	argv := collections.StringSliceToInterfaceSlice(args)
	if cmd.Exec.Context != nil {
		argv = append(argv, hexec.WithContext(cmd.Exec.Context))
	}
	if cmd.Exec.Stdin != nil {
		argv = append(argv, hexec.WithStdin(cmd.Exec.Stdin))
	}
	if cmd.Exec.Stdout != nil {
		argv = append(argv, hexec.WithStdout(cmd.Exec.Stdout))
	}
	if cmd.Exec.Stderr != nil {
		argv = append(argv, hexec.WithStderr(cmd.Exec.Stderr))
	}

	runner, err := r.exec.New(r.binary, argv...)
	if err != nil {
		return err
	}

	return runner.Run()
}

func (r Runner) Compile(args CompileArgs) error {
	if args.Input.IsZero() {
		return errors.New("typst compile input is required")
	}
	if args.Input.IsStdin() && args.Output.IsZero() {
		return errors.New("typst compile output is required when input is stdin")
	}

	cliArgs := appendCompileFlags(nil, args)
	cliArgs = append(cliArgs, args.Input.String())
	if !args.Output.IsZero() {
		cliArgs = append(cliArgs, args.Output.String())
	}

	return r.Run(Command{Subcommand: "compile", Args: cliArgs, Exec: args.Exec})
}

func (r Runner) Watch(args WatchArgs) error {
	if args.Compile.Input.IsZero() {
		return errors.New("typst watch input is required")
	}
	if args.Compile.Input.IsStdin() && args.Compile.Output.IsZero() {
		return errors.New("typst watch output is required when input is stdin")
	}

	cliArgs := appendCompileFlags(nil, args.Compile)
	if args.Server.NoServe {
		cliArgs = append(cliArgs, "--no-serve")
	}
	if args.Server.NoReload {
		cliArgs = append(cliArgs, "--no-reload")
	}
	if args.Server.Port > 0 {
		cliArgs = append(cliArgs, "--port", strconv.Itoa(args.Server.Port))
	}

	cliArgs = append(cliArgs, args.Compile.Input.String())
	if !args.Compile.Output.IsZero() {
		cliArgs = append(cliArgs, args.Compile.Output.String())
	}

	return r.Run(Command{Subcommand: "watch", Args: cliArgs, Exec: args.Compile.Exec})
}

func (r Runner) Query(args QueryArgs) error {
	if args.Input.IsZero() {
		return errors.New("typst query input is required")
	}
	if args.Selector == "" {
		return errors.New("typst query selector is required")
	}

	cliArgs := []string{args.Input.String(), args.Selector}
	if args.Field != "" {
		cliArgs = append(cliArgs, "--field", args.Field)
	}
	if args.One {
		cliArgs = append(cliArgs, "--one")
	}
	if args.Format != "" {
		cliArgs = append(cliArgs, "--format", string(args.Format))
	}
	if args.Pretty {
		cliArgs = append(cliArgs, "--pretty")
	}
	if args.Target != "" {
		cliArgs = append(cliArgs, "--target", string(args.Target))
	}

	cliArgs = appendWorldArgs(cliArgs, args.World)
	cliArgs = appendProcessArgs(cliArgs, args.Process)

	return r.Run(Command{Subcommand: "query", Args: cliArgs, Exec: args.Exec})
}

func appendCompileFlags(args []string, compile CompileArgs) []string {
	if compile.Format != "" {
		args = append(args, "-f", string(compile.Format))
	}

	args = appendWorldArgs(args, compile.World)

	if len(compile.Pages) > 0 {
		args = append(args, "--pages", strings.Join(compile.Pages, ","))
	}
	if len(compile.PDFStandard) > 0 {
		args = append(args, "--pdf-standard", strings.Join(compile.PDFStandard, ","))
	}
	if compile.NoPDFTags {
		args = append(args, "--no-pdf-tags")
	}
	if compile.PPI > 0 {
		args = append(args, "--ppi", strconv.FormatFloat(float64(compile.PPI), 'f', -1, 32))
	}
	if !compile.Deps.IsZero() {
		args = append(args, "--deps", compile.Deps.String())
	}
	if compile.DepsFormat != "" {
		args = append(args, "--deps-format", string(compile.DepsFormat))
	}

	args = appendProcessArgs(args, compile.Process)

	if compile.Open != nil {
		args = append(args, "--open")
		if *compile.Open != "" {
			args = append(args, *compile.Open)
		}
	}

	if compile.Timings != nil {
		args = append(args, "--timings")
		if *compile.Timings != "" {
			args = append(args, *compile.Timings)
		}
	}

	return args
}

func appendWorldArgs(args []string, world WorldArgs) []string {
	if world.Root != "" {
		args = append(args, "--root", world.Root)
	}

	for _, in := range world.Inputs {
		args = append(args, "--input", in.Key+"="+in.Value)
	}

	args = appendFontArgs(args, world.Font)
	args = appendPackageArgs(args, world.Package)

	if world.CreationTimestamp != "" {
		args = append(args, "--creation-timestamp", world.CreationTimestamp)
	}

	return args
}

func appendFontArgs(args []string, font FontArgs) []string {
	if len(font.FontPaths) > 0 {
		var paths string
		for i, p := range font.FontPaths {
			if i > 0 {
				paths += string(os.PathListSeparator)
			}
			paths += p
		}
		args = append(args, "--font-path", paths)
	}

	if font.IgnoreSystemFonts {
		args = append(args, "--ignore-system-fonts")
	}
	if font.IgnoreEmbeddedFonts {
		args = append(args, "--ignore-embedded-fonts")
	}

	return args
}

func appendPackageArgs(args []string, pkg PackageArgs) []string {
	if pkg.PackagePath != "" {
		args = append(args, "--package-path", pkg.PackagePath)
	}
	if pkg.PackageCachePath != "" {
		args = append(args, "--package-cache-path", pkg.PackageCachePath)
	}
	return args
}

func appendProcessArgs(args []string, process ProcessArgs) []string {
	if process.Jobs > 0 {
		args = append(args, "-j", strconv.Itoa(process.Jobs))
	}
	if len(process.Features) > 0 {
		features := make([]string, len(process.Features))
		for i, v := range process.Features {
			features[i] = string(v)
		}
		args = append(args, "--features", strings.Join(features, ","))
	}
	if process.DiagnosticFormat != "" {
		args = append(args, "--diagnostic-format", string(process.DiagnosticFormat))
	}
	return args
}
