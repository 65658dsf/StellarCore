// Copyright 2023 The frp Authors
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

package proxy

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fatedier/golib/errors"

	"github.com/65658dsf/StellarCore/client/event"
	"github.com/65658dsf/StellarCore/client/health"
	v1 "github.com/65658dsf/StellarCore/pkg/config/v1"
	"github.com/65658dsf/StellarCore/pkg/msg"
	"github.com/65658dsf/StellarCore/pkg/transport"
	"github.com/65658dsf/StellarCore/pkg/util/xlog"
)

const (
	ProxyPhaseNew         = "新建"
	ProxyPhaseWaitStart   = "等待启动"
	ProxyPhaseStartErr    = "启动错误"
	ProxyPhaseRunning     = "运行中"
	ProxyPhaseCheckFailed = "检查失败"
	ProxyPhaseClosed      = "已关闭"
)

var (
	statusCheckInterval = 3 * time.Second
	waitResponseTimeout = 20 * time.Second
	startErrTimeout     = 30 * time.Second
)

type WorkingStatus struct {
	Name  string             `json:"name"`
	Type  string             `json:"type"`
	Phase string             `json:"status"`
	Err   string             `json:"err"`
	Cfg   v1.ProxyConfigurer `json:"cfg"`

	// Got from server.
	RemoteAddr string `json:"remote_addr"`
}

type Wrapper struct {
	WorkingStatus

	// underlying proxy
	pxy Proxy

	// if ProxyConf has healcheck config
	// monitor will watch if it is alive
	monitor *health.Monitor

	// event handler
	handler event.Handler

	msgTransporter transport.MessageTransporter
	clientCfg      *v1.ClientCommonConfig

	health           uint32
	lastSendStartMsg time.Time
	lastStartErr     time.Time
	closeCh          chan struct{}
	healthNotifyCh   chan struct{}
	mu               sync.RWMutex

	xl  *xlog.Logger
	ctx context.Context
}

func NewWrapper(
	ctx context.Context,
	cfg v1.ProxyConfigurer,
	clientCfg *v1.ClientCommonConfig,
	eventHandler event.Handler,
	msgTransporter transport.MessageTransporter,
) *Wrapper {
	baseInfo := cfg.GetBaseConfig()
	xl := xlog.FromContextSafe(ctx).Spawn().AppendPrefix(baseInfo.Name)
	pw := &Wrapper{
		WorkingStatus: WorkingStatus{
			Name:  baseInfo.Name,
			Type:  baseInfo.Type,
			Phase: ProxyPhaseNew,
			Cfg:   cfg,
		},
		closeCh:        make(chan struct{}),
		healthNotifyCh: make(chan struct{}),
		handler:        eventHandler,
		msgTransporter: msgTransporter,
		clientCfg:      clientCfg,
		xl:             xl,
		ctx:            xlog.NewContext(ctx, xl),
	}

	if baseInfo.HealthCheck.Type != "" && baseInfo.LocalPort > 0 {
		pw.health = 1 // means failed
		addr := net.JoinHostPort(baseInfo.LocalIP, strconv.Itoa(baseInfo.LocalPort))
		pw.monitor = health.NewMonitor(pw.ctx, baseInfo.HealthCheck, addr,
			pw.statusNormalCallback, pw.statusFailedCallback)
		xl.Tracef("启用健康检查监控")
	}

	pw.pxy = NewProxy(pw.ctx, pw.Cfg, clientCfg, pw.msgTransporter)
	return pw
}

func (pw *Wrapper) SetInWorkConnCallback(cb func(*v1.ProxyBaseConfig, net.Conn, *msg.StartWorkConn) bool) {
	pw.pxy.SetInWorkConnCallback(cb)
}

func (pw *Wrapper) SetRunningStatus(remoteAddr string, respErr string) error {
	pw.mu.Lock()
	defer pw.mu.Unlock()
	if pw.Phase != ProxyPhaseWaitStart {
		return fmt.Errorf("状态不是等待启动，忽略启动消息")
	}

	pw.RemoteAddr = remoteAddr
	if respErr != "" {
		pw.Phase = ProxyPhaseStartErr
		pw.Err = respErr
		pw.lastStartErr = time.Now()
		return fmt.Errorf("%s", pw.Err)
	}

	if err := pw.pxy.Run(); err != nil {
		pw.close()
		pw.Phase = ProxyPhaseStartErr
		pw.Err = err.Error()
		pw.lastStartErr = time.Now()
		return err
	}

	pw.Phase = ProxyPhaseRunning
	pw.Err = ""
	return nil
}

func (pw *Wrapper) Start() {
	go pw.checkWorker()
	if pw.monitor != nil {
		go pw.monitor.Start()
	}
}

func (pw *Wrapper) Stop() {
	pw.mu.Lock()
	defer pw.mu.Unlock()
	close(pw.closeCh)
	close(pw.healthNotifyCh)
	pw.pxy.Close()
	if pw.monitor != nil {
		pw.monitor.Stop()
	}
	pw.Phase = ProxyPhaseClosed
	pw.close()
}

func (pw *Wrapper) close() {
	_ = pw.handler(&event.CloseProxyPayload{
		CloseProxyMsg: &msg.CloseProxy{
			ProxyName: pw.Name,
		},
	})
}

func (pw *Wrapper) checkWorker() {
	xl := pw.xl
	if pw.monitor != nil {
		// let monitor do check request first
		time.Sleep(500 * time.Millisecond)
	}
	for {
		// check proxy status
		now := time.Now()
		if atomic.LoadUint32(&pw.health) == 0 {
			pw.mu.Lock()
			if pw.Phase == ProxyPhaseNew ||
				pw.Phase == ProxyPhaseCheckFailed ||
				(pw.Phase == ProxyPhaseWaitStart && now.After(pw.lastSendStartMsg.Add(waitResponseTimeout))) ||
				(pw.Phase == ProxyPhaseStartErr && now.After(pw.lastStartErr.Add(startErrTimeout))) {

				xl.Tracef("状态由 [%s] 变更为 [%s]", pw.Phase, ProxyPhaseWaitStart)
				pw.Phase = ProxyPhaseWaitStart

				var newProxyMsg msg.NewProxy
				pw.Cfg.MarshalToMsg(&newProxyMsg)
				pw.lastSendStartMsg = now
				_ = pw.handler(&event.StartProxyPayload{
					NewProxyMsg: &newProxyMsg,
				})
			}
			pw.mu.Unlock()
		} else {
			pw.mu.Lock()
			if pw.Phase == ProxyPhaseRunning || pw.Phase == ProxyPhaseWaitStart {
				pw.close()
				xl.Tracef("状态由 [%s] 变更为 [%s]", pw.Phase, ProxyPhaseCheckFailed)
				pw.Phase = ProxyPhaseCheckFailed
			}
			pw.mu.Unlock()
		}

		select {
		case <-pw.closeCh:
			return
		case <-time.After(statusCheckInterval):
		case <-pw.healthNotifyCh:
		}
	}
}

func (pw *Wrapper) statusNormalCallback() {
	xl := pw.xl
	atomic.StoreUint32(&pw.health, 0)
	_ = errors.PanicToError(func() {
		select {
		case pw.healthNotifyCh <- struct{}{}:
		default:
		}
	})
	xl.Infof("健康检查成功")
}

func (pw *Wrapper) statusFailedCallback() {
	xl := pw.xl
	atomic.StoreUint32(&pw.health, 1)
	_ = errors.PanicToError(func() {
		select {
		case pw.healthNotifyCh <- struct{}{}:
		default:
		}
	})
	xl.Infof("健康检查失败")
}

func (pw *Wrapper) InWorkConn(workConn net.Conn, m *msg.StartWorkConn) {
	xl := pw.xl
	pw.mu.RLock()
	pxy := pw.pxy
	pw.mu.RUnlock()
	if pxy != nil && pw.Phase == ProxyPhaseRunning {
		xl.Debugf("启动新的工作连接, 本地地址: %s 远程地址: %s", workConn.LocalAddr().String(), workConn.RemoteAddr().String())
		go pxy.InWorkConn(workConn, m)
	} else {
		workConn.Close()
	}
}

func (pw *Wrapper) GetStatus() *WorkingStatus {
	pw.mu.RLock()
	defer pw.mu.RUnlock()
	ps := &WorkingStatus{
		Name:       pw.Name,
		Type:       pw.Type,
		Phase:      pw.Phase,
		Err:        pw.Err,
		Cfg:        pw.Cfg,
		RemoteAddr: pw.RemoteAddr,
	}
	return ps
}

// parseCertificateDomains 解析证书并提取域名信息
func parseCertificateDomains(crtBase64 string) ([]string, error) {
	// 解码base64证书
	crtData, err := base64.StdEncoding.DecodeString(crtBase64)
	if err != nil {
		return nil, fmt.Errorf("无法解码证书: %v", err)
	}

	// 解析PEM格式
	block, _ := pem.Decode(crtData)
	if block == nil {
		return nil, fmt.Errorf("无法解析证书PEM格式")
	}

	// 解析X.509证书
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("无法解析X.509证书: %v", err)
	}

	var domains []string

	// 添加Subject中的CommonName
	if cert.Subject.CommonName != "" {
		domains = append(domains, cert.Subject.CommonName)
	}

	// 添加SAN (Subject Alternative Names)
	domains = append(domains, cert.DNSNames...)

	// 去重
	uniqueDomains := make([]string, 0)
	seen := make(map[string]bool)
	for _, domain := range domains {
		if !seen[domain] {
			uniqueDomains = append(uniqueDomains, domain)
			seen[domain] = true
		}
	}

	return uniqueDomains, nil
}

func (pw *Wrapper) UpdateCertificate(crtBase64 string, keyBase64 string) error {
    pw.mu.Lock()
    defer pw.mu.Unlock()

    // 检查是否为HTTPS隧道类型
    if httpsConfig, ok := pw.Cfg.(*v1.HTTPSProxyConfig); ok {
        domains, err := parseCertificateDomains(crtBase64)
        if err != nil {
            pw.xl.Warnf("无法解析证书域名信息: %v", err)
        } else if len(domains) > 0 {
            pw.xl.Infof("隧道 [%s] 收到证书，域名: %s", pw.Name, strings.Join(domains, ", "))
        } else {
            pw.xl.Infof("隧道 [%s] 收到证书，无域名信息", pw.Name)
        }

        httpsConfig.CrtBase64 = crtBase64
        httpsConfig.KeyBase64 = keyBase64

        base := httpsConfig.GetBaseConfig()
        if base.Plugin.Type == "" {
            addr := net.JoinHostPort(base.LocalIP, strconv.Itoa(base.LocalPort))
            auto := true
            base.Plugin.Type = v1.PluginHTTPS2HTTP
            base.Plugin.ClientPluginOptions = &v1.HTTPS2HTTPPluginOptions{
                Type:       v1.PluginHTTPS2HTTP,
                LocalAddr:  addr,
                AutoTls:    &auto,
                CrtBase64:  crtBase64,
                KeyBase64:  keyBase64,
            }
        } else if base.Plugin.Type == v1.PluginHTTPS2HTTP {
            if opts, ok := base.Plugin.ClientPluginOptions.(*v1.HTTPS2HTTPPluginOptions); ok {
                opts.CrtBase64 = crtBase64
                opts.KeyBase64 = keyBase64
                auto := true
                opts.AutoTls = &auto
                if opts.LocalAddr == "" {
                    opts.LocalAddr = net.JoinHostPort(base.LocalIP, strconv.Itoa(base.LocalPort))
                }
            }
        }

        if pw.pxy != nil {
            pw.pxy.Close()
            pw.pxy = NewProxy(pw.ctx, pw.Cfg, pw.clientCfg, pw.msgTransporter)
        }

        return nil
    }

    return fmt.Errorf("该隧道,%s类型, 无法更新证书", pw.Type)
}
