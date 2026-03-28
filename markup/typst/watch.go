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
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	htypes "github.com/gohugoio/hugo/common/types"
	"github.com/gohugoio/hugo/markup/converter"
	"github.com/gohugoio/hugo/markup/typst/typst_config"
	"github.com/gohugoio/hugo/markup/typst/typstcli"
)

type watchManager struct {
	runner  typstcli.Runner
	logger  interface{ Warnf(format string, v ...any) }
	timeout time.Duration

	pages     string
	outputDir string

	mu      sync.Mutex
	entries map[string]*watchEntry
	closed  bool
}

type watchEntry struct {
	input  string
	output string

	startOnce sync.Once
	cancel    context.CancelFunc
	done      chan struct{}

	errMu sync.RWMutex
	err   error
}

func newWatchManager(cfg converter.ProviderConfig, tcfg typst_config.Config) (*watchManager, error) {
	timeout := typstWatchTimeout(tcfg)
	cacheDir := cfg.Conf.CacheDirMisc()
	outputDir := filepath.Join(cacheDir, "typstwatch")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, err
	}

	m := &watchManager{
		runner:    typstcli.New(cfg.Exec, tcfg.Binary),
		logger:    cfg.Logger,
		timeout:   timeout,
		pages:     tcfg.Pages,
		outputDir: outputDir,
		entries:   make(map[string]*watchEntry),
	}
	return m, nil
}

func typstWatchTimeout(cfg typst_config.Config) time.Duration {
	timeout := 3 * time.Second
	v := strings.TrimSpace(cfg.Watch.Timeout)
	if v == "" {
		return timeout
	}
	d, err := htypes.ToDurationE(v)
	if err != nil || d <= 0 {
		return timeout
	}
	return d
}

func shouldUseWatch(cfg converter.ProviderConfig, tcfg typst_config.Config) bool {
	if !tcfg.Watch.Enabled {
		return false
	}
	if cfg.Conf == nil {
		return false
	}
	if cfg.Exec == nil {
		return false
	}
	return cfg.Conf.Running() && cfg.Conf.Watching()
}

func (m *watchManager) render(input string, world typstcli.WorldArgs, process typstcli.ProcessArgs) ([]byte, error) {
	entry, err := m.ensure(input, world, process)
	if err != nil {
		return nil, err
	}
	if err := m.waitReady(entry); err != nil {
		return nil, err
	}
	return os.ReadFile(entry.output)
}

func (m *watchManager) ensure(input string, world typstcli.WorldArgs, process typstcli.ProcessArgs) (*watchEntry, error) {
	absInput := input
	if p, err := filepath.Abs(input); err == nil {
		absInput = p
	}

	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil, errors.New("typst watch manager is closed")
	}
	entry, found := m.entries[absInput]
	if !found {
		entry = &watchEntry{
			input:  absInput,
			output: filepath.Join(m.outputDir, hashFilename(absInput)+".html"),
			done:   make(chan struct{}),
		}
		m.entries[absInput] = entry
	}
	m.mu.Unlock()

	entry.startOnce.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		entry.cancel = cancel

		go func() {
			defer close(entry.done)
			err := m.runner.Watch(typstcli.WatchArgs{
				Compile: typstcli.CompileArgs{
					Input:   typstcli.Input(entry.input),
					Output:  typstcli.Output(entry.output),
					Format:  typstcli.OutputFormatHTML,
					World:   world,
					Pages:   pagesFromConfig(m.pages),
					Process: process,
					Exec: typstcli.ExecOptions{
						Context: ctx,
						Stderr:  os.Stderr,
					},
				},
				Server: typstcli.ServerArgs{
					NoServe:  true,
					NoReload: true,
				},
			})
			if errors.Is(ctx.Err(), context.Canceled) {
				return
			}
			if err == nil {
				err = errors.New("typst watch exited unexpectedly")
			}
			entry.setErr(err)
			if m.logger != nil {
				m.logger.Warnf("typst watch failed for %q: %v", entry.input, err)
			}
		}()
	})

	return entry, nil
}

func (m *watchManager) waitReady(entry *watchEntry) error {
	info, err := os.Stat(entry.input)
	if err != nil {
		return err
	}
	deadline := time.Now().Add(m.timeout)
	for {
		if err := entry.getErr(); err != nil {
			return err
		}
		outInfo, err := os.Stat(entry.output)
		if err == nil && !outInfo.ModTime().Before(info.ModTime()) {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out after %s waiting for typst watch output", m.timeout)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func (m *watchManager) Close() error {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil
	}
	m.closed = true
	entries := make([]*watchEntry, 0, len(m.entries))
	for _, e := range m.entries {
		entries = append(entries, e)
	}
	m.mu.Unlock()

	for _, e := range entries {
		if e.cancel != nil {
			e.cancel()
		}
	}

	for _, e := range entries {
		if e.done == nil {
			continue
		}
		select {
		case <-e.done:
		case <-time.After(time.Second):
			if m.logger != nil {
				m.logger.Warnf("timeout waiting for typst watch to stop for %q", e.input)
			}
		}
	}
	return nil
}

func (e *watchEntry) setErr(err error) {
	e.errMu.Lock()
	defer e.errMu.Unlock()
	e.err = err
}

func (e *watchEntry) getErr() error {
	e.errMu.RLock()
	defer e.errMu.RUnlock()
	return e.err
}

func hashFilename(s string) string {
	sum := sha1.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}
