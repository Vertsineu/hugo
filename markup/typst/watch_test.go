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

package typst

import (
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/gohugoio/hugo/common/hexec"
	"github.com/gohugoio/hugo/markup/typst/typst_config"
	"github.com/gohugoio/hugo/markup/typst/typstcli"
)

func TestTypstWatchTimeout(t *testing.T) {
	c := qt.New(t)

	c.Assert(typstWatchTimeout(typst_config.Config{}), qt.Equals, 3*time.Second)
	c.Assert(typstWatchTimeout(typst_config.Config{Watch: typst_config.WatchConfig{Timeout: "5s"}}), qt.Equals, 5*time.Second)
	c.Assert(typstWatchTimeout(typst_config.Config{Watch: typst_config.WatchConfig{Timeout: "invalid"}}), qt.Equals, 3*time.Second)
	c.Assert(typstWatchTimeout(typst_config.Config{Watch: typst_config.WatchConfig{Timeout: "-2s"}}), qt.Equals, 3*time.Second)
}

func TestWatchManagerRenderUnexpectedExit(t *testing.T) {
	c := qt.New(t)
	tmpDir := t.TempDir()
	input := filepath.Join(tmpDir, "main.typ")
	c.Assert(os.WriteFile(input, []byte("= Hello"), 0o644), qt.IsNil)

	m := &watchManager{
		runner:    typstcli.New(&fakeExec{newRunner: func() hexec.Runner { return &fakeCmd{} }}, "typst"),
		timeout:   200 * time.Millisecond,
		outputDir: tmpDir,
		entries:   make(map[string]*watchEntry),
	}

	_, err := m.render(input, typstcli.WorldArgs{}, typstcli.ProcessArgs{})
	c.Assert(err, qt.ErrorMatches, "typst watch exited unexpectedly")
}

func TestWatchManagerWaitReadyTimeout(t *testing.T) {
	c := qt.New(t)
	tmpDir := t.TempDir()
	input := filepath.Join(tmpDir, "main.typ")
	c.Assert(os.WriteFile(input, []byte("= Hello"), 0o644), qt.IsNil)

	m := &watchManager{timeout: 50 * time.Millisecond}
	err := m.waitReady(&watchEntry{input: input, output: filepath.Join(tmpDir, "missing.html")})
	c.Assert(err, qt.ErrorMatches, `timed out after .* waiting for typst watch output`)
}

type fakeExec struct {
	newRunner func() hexec.Runner
}

func (e *fakeExec) New(name string, arg ...any) (hexec.Runner, error) {
	if e.newRunner != nil {
		return e.newRunner(), nil
	}
	return &fakeCmd{}, nil
}

type fakeCmd struct{}

func (c *fakeCmd) Run() error {
	return nil
}

func (c *fakeCmd) StdinPipe() (io.WriteCloser, error) {
	return nopWriteCloser{}, nil
}

type nopWriteCloser struct{}

func (n nopWriteCloser) Write(p []byte) (int, error) {
	return len(p), nil
}

func (n nopWriteCloser) Close() error {
	return nil
}
