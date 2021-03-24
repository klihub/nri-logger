// Copyright 2021 Intel Corporation. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/containerd/nri/skel"
	types "github.com/containerd/nri/types/v1"

	"sigs.k8s.io/yaml"
)

const (
	pluginType    = "logger"
	pluginVersion = "0.1"
	defaultFile   = "/tmp/nri.log"
)

type config struct {
	LogFile string `json:"logFile,omitempty"`
}

type plugin struct {
	cfg  config
	buf  strings.Builder
	file *os.File
}

func (c config) path() string {
	if c.LogFile != "" {
		return c.LogFile
	}
	return defaultFile
}

func (p *plugin) Type() string {
	return pluginType
}

func (p *plugin) Version() string {
	return pluginVersion
}

func (p *plugin) configure(request *types.Request) error {

	rawConfig := string(request.Conf)
	if rawConfig == "" {
		return nil
	}
	raw, err := strconv.Unquote(rawConfig)
	if err == nil {
		rawConfig = raw
	}

	cfg := config{}
	if err := json.Unmarshal([]byte(rawConfig), &cfg); err != nil {
		return errors.Wrapf(err, "failed to unmarshal plugin configuration")
	}

	p.cfg = cfg
	p.open()

	return nil
}

func (p *plugin) open() error {
	flags, mode := os.O_APPEND|os.O_CREATE|os.O_WRONLY, fs.FileMode(0644)
	file, err := os.OpenFile(p.cfg.path(), flags, mode)
	if err != nil {
		return p.error("failed to open log file %q: %v", p.cfg.path(), err)
	}

	p.close()
	p.file = file
	return nil
}

func (p *plugin) close() {
	if p.file == nil {
		return
	}
	p.flush()
	p.file.Close()
	p.file = nil
}

func (p *plugin) flush() {
	if p.file == nil {
		return
	}
	p.file.Write([]byte(p.buf.String()))
	p.file.Sync()
	p.buf.Reset()
}

func (p *plugin) logLine(format string, args ...interface{}) {
	p.buf.WriteString(fmt.Sprintf(format, args...))
	p.buf.WriteByte('\n')
}

func (p *plugin) logBlock(prefix, format string, args ...interface{}) {
	for _, line := range strings.Split(fmt.Sprintf(format, args...), "\n") {
		p.buf.WriteString(prefix)
		p.buf.WriteString(line)
		p.buf.WriteByte('\n')
	}
}

func (p *plugin) error(format string, args ...interface{}) error {
	return fmt.Errorf(pluginType+": "+format, args...)
}

func (p *plugin) Invoke(ctx context.Context, request *types.Request) (*types.Result, error) {
	var kind string

	result := request.NewResult(pluginType)

	if err := p.configure(request); err != nil {
		result.Error = err.Error()
		return result, nil
	}

	if request.IsSandbox() {
		kind = "pod"
	} else {
		kind = "container"
	}

	raw, err := yaml.Marshal(request)
	if err != nil {
		p.logLine("failed to marshal request: %v\n", err)
	}

	prefix := fmt.Sprintf("    %s %s request: ", kind, request.State)
	p.logLine("=> %s %s request for %s:%s:\n", kind, request.State,
		request.SandboxID, request.ID)
	p.logBlock(prefix, "%s", string(raw))

	return result, nil
}

func main() {
	plugin := &plugin{}
	defer plugin.flush()

	err := skel.Run(context.Background(), plugin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "NRI failed to run plugin %s: %v\n", pluginType, err)
		os.Exit(1)
	}
}
