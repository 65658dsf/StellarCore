// Copyright 2019 fatedier, fatedier@gmail.com
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
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	stdlog "log"
	"net"
	"net/http"
	"net/http/httputil"
	"time"

	"github.com/fatedier/golib/pool"
	"github.com/samber/lo"

	v1 "github.com/65658dsf/StellarCore/pkg/config/v1"
	"github.com/65658dsf/StellarCore/pkg/transport"
	httppkg "github.com/65658dsf/StellarCore/pkg/util/http"
	"github.com/65658dsf/StellarCore/pkg/util/log"
	netpkg "github.com/65658dsf/StellarCore/pkg/util/net"
)

func init() {
	Register(v1.PluginHTTPS2HTTP, NewHTTPS2HTTPPlugin)
}

// createTLSConfigFromBase64 从Base64编码的证书和密钥创建TLS配置
func createTLSConfigFromBase64(crtBase64, keyBase64 string) (*tls.Config, error) {
	// 解码证书
	crtData, err := base64.StdEncoding.DecodeString(crtBase64)
	if err != nil {
		return nil, fmt.Errorf("解码证书Base64错误: %v", err)
	}

	// 解码密钥
	keyData, err := base64.StdEncoding.DecodeString(keyBase64)
	if err != nil {
		return nil, fmt.Errorf("解码密钥Base64错误: %v", err)
	}

	// 从内存中的证书和密钥数据创建TLS证书
	cert, err := tls.X509KeyPair(crtData, keyData)
	if err != nil {
		return nil, fmt.Errorf("创建X509密钥对错误: %v", err)
	}

	// 创建TLS配置
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	return tlsConfig, nil
}

type HTTPS2HTTPPlugin struct {
	opts *v1.HTTPS2HTTPPluginOptions

	l *Listener
	s *http.Server
}

func NewHTTPS2HTTPPlugin(options v1.ClientPluginOptions) (Plugin, error) {
	opts := options.(*v1.HTTPS2HTTPPluginOptions)
	listener := NewProxyListener()

	p := &HTTPS2HTTPPlugin{
		opts: opts,
		l:    listener,
	}

	rp := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.Out.Header["X-Forwarded-For"] = r.In.Header["X-Forwarded-For"]
			r.SetXForwarded()
			req := r.Out
			req.URL.Scheme = "http"
			req.URL.Host = p.opts.LocalAddr
			if p.opts.HostHeaderRewrite != "" {
				req.Host = p.opts.HostHeaderRewrite
			}
			for k, v := range p.opts.RequestHeaders.Set {
				req.Header.Set(k, v)
			}
		},
		BufferPool: pool.NewBuffer(32 * 1024),
		ErrorLog:   stdlog.New(log.NewWriteLogger(log.WarnLevel, 2), "", 0),
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.TLS != nil {
			tlsServerName, _ := httppkg.CanonicalHost(r.TLS.ServerName)
			host, _ := httppkg.CanonicalHost(r.Host)
			if tlsServerName != "" && tlsServerName != host {
				w.WriteHeader(http.StatusMisdirectedRequest)
				return
			}
		}
		rp.ServeHTTP(w, r)
	})

	var tlsConfig *tls.Config
	var err error

	// 检查是否启用AutoTls
	if lo.FromPtr(opts.AutoTls) {
		// 如果启用AutoTls，优先使用Base64证书，否则使用自动生成的证书
		if opts.CrtBase64 != "" && opts.KeyBase64 != "" {
			tlsConfig, err = createTLSConfigFromBase64(opts.CrtBase64, opts.KeyBase64)
			if err != nil {
				return nil, fmt.Errorf("创建TLS配置从Base64错误: %v", err)
			}
		} else {
			// 使用自动生成的证书
			tlsConfig, err = transport.NewServerTLSConfig("", "", "")
			if err != nil {
				return nil, fmt.Errorf("生成自动TLS配置错误: %v", err)
			}
		}
	} else {
		// 传统模式：使用文件路径
		tlsConfig, err = transport.NewServerTLSConfig(p.opts.CrtPath, p.opts.KeyPath, "")
		if err != nil {
			return nil, fmt.Errorf("生成传统TLS配置错误: %v", err)
		}
	}

	p.s = &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 60 * time.Second,
		TLSConfig:         tlsConfig,
	}
	if !lo.FromPtr(opts.EnableHTTP2) {
		p.s.TLSNextProto = make(map[string]func(*http.Server, *tls.Conn, http.Handler))
	}

	go func() {
		_ = p.s.ServeTLS(listener, "", "")
	}()
	return p, nil
}

func (p *HTTPS2HTTPPlugin) Handle(_ context.Context, conn io.ReadWriteCloser, realConn net.Conn, extra *ExtraInfo) {
	wrapConn := netpkg.WrapReadWriteCloserToConn(conn, realConn)
	if extra.SrcAddr != nil {
		wrapConn.SetRemoteAddr(extra.SrcAddr)
	}
	_ = p.l.PutConn(wrapConn)
}

func (p *HTTPS2HTTPPlugin) Name() string {
	return v1.PluginHTTPS2HTTP
}

func (p *HTTPS2HTTPPlugin) Close() error {
	return p.s.Close()
}
