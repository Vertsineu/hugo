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

// Package typst_config holds Typst-related configuration.
package typst_config

type WatchConfig struct {
	Enabled bool
	Timeout string
}

// Config configures the Typst converter.
type Config struct {
	Binary string

	// Root sets Typst's project root. If empty, Hugo uses the current .typ file's directory.
	Root string

	// Input values exposed to Typst via sys.inputs.
	Inputs map[string]string

	// FontPaths are additional directories searched for fonts.
	FontPaths []string

	IgnoreSystemFonts   bool
	IgnoreEmbeddedFonts bool

	PackagePath      string
	PackageCachePath string

	Jobs  int
	Pages string

	Watch WatchConfig
}

var Default = Config{
	Binary: "typst",
	Watch: WatchConfig{
		Timeout: "3s",
	},
}
