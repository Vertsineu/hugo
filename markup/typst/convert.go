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

// Package typst converts Typst content to HTML using go-typst.
package typst

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"

	gotypst "github.com/Dadido3/go-typst"
	"github.com/gohugoio/hugo/common/hexec"
	"github.com/gohugoio/hugo/common/loggers"
	"github.com/gohugoio/hugo/htesting"
	"github.com/gohugoio/hugo/identity"
	"github.com/gohugoio/hugo/markup/converter"
	"github.com/gohugoio/hugo/markup/typst/typst_config"
)

// Provider is the package entry point.
var Provider converter.ProviderProvider = provider{}

type provider struct{}

func (p provider) New(cfg converter.ProviderConfig) (converter.Provider, error) {
	return converter.NewProvider("typst", func(ctx converter.DocumentContext) (converter.Converter, error) {
		return &typstConverter{
			ctx: ctx,
			cfg: cfg,
		}, nil
	}), nil
}

type typstConverter struct {
	ctx converter.DocumentContext
	cfg converter.ProviderConfig
}

func (c *typstConverter) Convert(ctx converter.RenderContext) (converter.ResultRender, error) {
	b, err := c.getTypstContent(ctx.Src, c.ctx)
	if err != nil {
		return nil, err
	}
	return converter.Bytes(b), nil
}

func (c *typstConverter) Supports(feature identity.Identity) bool {
	return false
}

func (c *typstConverter) getTypstContent(src []byte, ctx converter.DocumentContext) ([]byte, error) {
	logger := c.cfg.Logger

	cfg := c.cfg.MarkupConfig().Typst
	if cfg.Binary == "" {
		cfg.Binary = typst_config.Default.Binary
	}

	if _, err := c.cfg.Exec.New(cfg.Binary, "--version"); err != nil {
		if hexec.IsNotFound(err) {
			logger.Println("typst not found in $PATH: Please install.\n",
				"                 Leaving typst content unrendered.")
			return src, nil
		}
		return nil, err
	}

	options := &gotypst.OptionsCompile{
		Format: gotypst.OutputFormatHTML,
	}
	options.Root = resolveRootDirectory(cfg.Root, ctx)
	if len(cfg.Inputs) > 0 {
		options.Input = make(map[string]string, len(cfg.Inputs))
		for k, v := range cfg.Inputs {
			options.Input[k] = v
		}
	}
	if len(cfg.FontPaths) > 0 {
		options.FontPaths = append([]string(nil), cfg.FontPaths...)
	}
	options.IgnoreSystemFonts = cfg.IgnoreSystemFonts
	options.IgnoreEmbeddedFonts = cfg.IgnoreEmbeddedFonts
	options.PackagePath = cfg.PackagePath
	options.PackageCachePath = cfg.PackageCachePath
	options.Jobs = cfg.Jobs
	options.Pages = cfg.Pages

	var out bytes.Buffer
	caller := gotypst.CLI{ExecutablePath: cfg.Binary}
	if err := caller.Compile(bytes.NewReader(src), &out, options); err != nil {
		logDetails(logger, ctx.DocumentName, err)
		logger.Errorf("%s rendering %s: %v", cfg.Binary, ctx.DocumentName, err)
		return src, nil
	}

	if out.Len() == 0 {
		logger.Errorf("%s rendered no output for %s", cfg.Binary, ctx.DocumentName)
		return src, nil
	}

	clean := stripHTMLDocument(out.Bytes())
	return normalizeExternalHelperLineFeeds(clean), nil
}

func resolveRootDirectory(configuredRoot string, ctx converter.DocumentContext) string {
	if configuredRoot != "" {
		return configuredRoot
	}
	if ctx.Filename != "" {
		return filepath.Dir(ctx.Filename)
	}
	if ctx.DocumentName != "" {
		return filepath.Dir(ctx.DocumentName)
	}
	return ""
}

func logDetails(logger loggers.Logger, doc string, err error) {
	var terr *gotypst.Error
	if !errors.As(err, &terr) {
		return
	}
	for _, d := range terr.Details {
		msg := strings.TrimSpace(d.Message)
		if msg == "" {
			continue
		}
		if d.Path != "" && d.Line > 0 {
			logger.Errorf("%s: %s:%d:%d: %s", doc, d.Path, d.Line, d.Column, msg)
			continue
		}
		logger.Errorf("%s: %s", doc, msg)
	}
}

func normalizeExternalHelperLineFeeds(content []byte) []byte {
	return bytes.Replace(content, []byte("\r"), []byte(""), -1)
}

func stripHTMLDocument(content []byte) []byte {
	if len(content) == 0 {
		return content
	}
	lower := bytes.ToLower(content)
	start := bytes.Index(lower, []byte("<body"))
	if start < 0 {
		return content
	}
	startEnd := bytes.IndexByte(lower[start:], '>')
	if startEnd < 0 {
		return content
	}
	start = start + startEnd + 1
	end := bytes.Index(lower[start:], []byte("</body>"))
	if end < 0 {
		return content
	}
	end = start + end
	if start >= end || start > len(content) {
		return content
	}
	if end > len(content) {
		end = len(content)
	}
	return content[start:end]
}

// Supports returns whether Typst is installed on this computer.
func Supports() bool {
	hasBin := hexec.InPath(typst_config.Default.Binary)
	if htesting.SupportsAll() {
		if !hasBin {
			panic("typst not installed")
		}
		return true
	}
	return hasBin
}
