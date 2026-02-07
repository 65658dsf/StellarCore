// Copyright 2025 The frp Authors
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

//go:build !frps

package plugin

import (
	"context"
	"io"
	"net"

	v1 "github.com/65658dsf/StellarCore/pkg/config/v1"
)

func init() {
	Register(v1.PluginVirtualNet, NewVirtualNetPlugin)
}

type VirtualNetPlugin struct {
	opts *v1.VirtualNetPluginOptions
}

func NewVirtualNetPlugin(options v1.ClientPluginOptions) (Plugin, error) {
	opts := options.(*v1.VirtualNetPluginOptions)

	p := &VirtualNetPlugin{
		opts: opts,
	}
	return p, nil
}

func (p *VirtualNetPlugin) Name() string {
	return v1.PluginVirtualNet
}

func (p *VirtualNetPlugin) Handle(ctx context.Context, conn io.ReadWriteCloser, realConn net.Conn, extra *ExtraInfo) {
	_ = conn.Close()
}

func (p *VirtualNetPlugin) Close() error {
	return nil
}
