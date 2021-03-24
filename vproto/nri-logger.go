/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"sigs.k8s.io/yaml"

	"github.com/pkg/errors"
	"github.com/containerd/ttrpc"

	api "github.com/containerd/nri/api/plugin/vproto"
)

// config data for our logger plugin.
type config struct {
	LogFile string   `json:"logFile"`
	Events  []string `json:"events"`
}

// plugin is a logger for NRI events.
type plugin struct {
	listener net.Listener
	server   *ttrpc.Server
	pipeFd   int
	Logger
}

func (p *plugin) Configure(ctx context.Context, req *api.ConfigureRequest) (*api.ConfigureResponse, error) {
	cfg := config{}

	if req.Config != "" {
		p.Error("parsing configuration %q...", req.Config)
		err := yaml.Unmarshal([]byte(req.Config), &cfg)
		if err != nil {
			p.Error("failed to parse configuration: %v", err)
			return nil, errors.Wrap(err, "invalid configuration")
		}
	}

	rpl := &api.ConfigureResponse{}

	events := map[string]api.Event{
		"runpodsandbox":    api.Event_RUN_POD_SANDBOX,
		"stoppodsandbox":   api.Event_STOP_POD_SANDBOX,
		"removepodsandbox": api.Event_REMOVE_POD_SANDBOX,
		"createcontainer":  api.Event_CREATE_CONTAINER,
		"startcontainer":   api.Event_START_CONTAINER,
		"updatecontainer":  api.Event_UPDATE_CONTAINER,
		"stopcontainer":    api.Event_STOP_CONTAINER,
		"removecontainer":  api.Event_REMOVE_CONTAINER,
		"all":              api.Event_ALL,
	}
	for _, name := range cfg.Events {
		e, ok := events[strings.ToLower(name)]
		if !ok {
			return nil, errors.Errorf("invalid event %q", name)
		}
		rpl.Subscribe = append(rpl.Subscribe, e)
	}
	if len(rpl.Subscribe) < 1 {
		rpl.Subscribe = []api.Event{ api.Event_ALL }
	}

	if cfg.LogFile != "" {
		w, err := os.OpenFile(cfg.LogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to open log file %q", cfg.LogFile)
		}
		p.SetWriter(w)
	}

	return rpl, nil
}

func (p *plugin) Synchronize(ctx context.Context, req *api.SynchronizeRequest) (*api.SynchronizeResponse, error) {
	p.dump(req)
	return &api.SynchronizeResponse{}, nil
}

func (p *plugin) Shutdown(ctx context.Context, req *api.ShutdownRequest) (*api.ShutdownResponse, error) {
	return &api.ShutdownResponse{}, nil
}

func (p *plugin) RunPodSandbox(ctx context.Context, req *api.RunPodSandboxRequest) (*api.RunPodSandboxResponse, error) {
	p.dump(req)
	return &api.RunPodSandboxResponse{}, nil
}

func (p *plugin) StopPodSandbox(ctx context.Context, req *api.StopPodSandboxRequest) (*api.StopPodSandboxResponse, error) {
	p.dump(req)
	return &api.StopPodSandboxResponse{}, nil
}

func (p *plugin) RemovePodSandbox(ctx context.Context, req *api.RemovePodSandboxRequest) (*api.RemovePodSandboxResponse, error) {
	p.dump(req)
	return &api.RemovePodSandboxResponse{}, nil
}

func (p *plugin) CreateContainer(ctx context.Context, req *api.CreateContainerRequest) (*api.CreateContainerResponse, error) {
	p.dump(req)
	return &api.CreateContainerResponse{
		Create: &api.ContainerCreateUpdate{
			Labels: map[string]string{
				fmt.Sprintf("logger%d", os.Getpid()) : "was-here",
			},
		},
	}, nil

}

func (p *plugin) StartContainer(ctx context.Context, req *api.StartContainerRequest) (*api.StartContainerResponse, error) {
	p.dump(req)
	return &api.StartContainerResponse{}, nil
}

func (p *plugin) UpdateContainer(ctx context.Context, req *api.UpdateContainerRequest) (*api.UpdateContainerResponse, error) {
	p.dump(req)
	return &api.UpdateContainerResponse{}, nil
}

func (p *plugin) StopContainer(ctx context.Context, req *api.StopContainerRequest) (*api.StopContainerResponse, error) {
	p.dump(req)
	return &api.StopContainerResponse{}, nil
}

func (p *plugin) RemoveContainer(ctx context.Context, req *api.RemoveContainerRequest) (*api.RemoveContainerResponse, error) {
	p.dump(req)
	return &api.RemoveContainerResponse{}, nil
}

// create a plugin using a pre-connected socketpair.
func create(sockFd, pipeFd int, l Logging) (*plugin, error) {
	server, err := ttrpc.NewServer()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create ttrpc server")
	}

	listener, err := newSocketListener(sockFd)
	if err != nil {
		return nil, err
	}

	p := &plugin{
		listener: listener,
		server:   server,
		pipeFd:   pipeFd,
		Logger:   l.Get("plugin"),
	}

	return p, nil
}

// connect to the given NRI socket and create a plugin.
func connect(path string, l Logging) (*plugin, error) {
	conn, err := net.Dial("unix", path)
	if err != nil {
		return nil, err
	}

	server, err := ttrpc.NewServer()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create ttrpc server")
	}

	return &plugin{
		listener: newConnListener(conn),
		server:   server,
		pipeFd:   -1,
		Logger:   l.Get("plugin"),
	}, nil
}

// run the plugin.
func (p *plugin) run(ctx context.Context) error {
	api.RegisterPluginService(p.server, p)
	return p.server.Serve(ctx, p.listener)
}

// dump a message.
func (p *plugin) dump(obj interface{}) {
	msg, err := yaml.Marshal(obj)
	if err != nil {
		return
	}
	prefix := ""+strings.TrimPrefix(fmt.Sprintf("%T", obj), "*vproto.")
	prefix = strings.TrimSuffix(prefix, "Request") + ": "
	p.InfoBlock(prefix, "%s", msg)
}

// call the given function if/when our server-monitoring pipe is closed.
func (p *plugin) callOnClose(fn func ()) {
	if p.pipeFd < 0 {
		return
	}

	go func() {
		pipe := os.NewFile(uintptr(p.pipeFd), "pipe-fd#"+strconv.Itoa(p.pipeFd))
		_, _ = pipe.Read(make([]byte, 1))
		fn()
	}()
}

func main() {
	var p *plugin

	if len(os.Args) < 2 {
		logFile, err := os.OpenFile("/tmp/nri-logger.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			os.Exit(1)
		}

		sockFd := 3
		pipeFd := 0

		p, err = create(sockFd, pipeFd, LogWriter(logFile))
		if err != nil {
			fmt.Printf("failed to set up plugin: %v\n", err)
			os.Exit(1)
		}

		p.callOnClose(func () {
			p.listener.Close()
			p.server.Close()
			os.Exit(0)
		})
	} else {
		var err error

		p, err = connect(os.Args[1], LogWriter(os.Stdout))
		if err != nil {
			fmt.Printf("failed to connect to NRI server: %v\n", err)
			os.Exit(1)
		}
	}

	if err := p.run(context.Background()); err != nil {
		fmt.Errorf("failed to run plugin: %v\n", err)
		os.Exit(1)
	}
}
