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

//go:build !frps

package proxy

import (
	"io"
	"net"
	"reflect"
	"strconv"
	"time"

	"github.com/fatedier/golib/errors"
	libio "github.com/fatedier/golib/io"

	v1 "github.com/65658dsf/StellarCore/pkg/config/v1"
	"github.com/65658dsf/StellarCore/pkg/msg"
	"github.com/65658dsf/StellarCore/pkg/proto/udp"
	"github.com/65658dsf/StellarCore/pkg/util/limit"
	netpkg "github.com/65658dsf/StellarCore/pkg/util/net"
)

func init() {
	RegisterProxyFactory(reflect.TypeOf(&v1.UDPProxyConfig{}), NewUDPProxy)
}

type UDPProxy struct {
	*BaseProxy

	cfg *v1.UDPProxyConfig

	localAddr *net.UDPAddr
	readCh    chan *msg.UDPPacket

	// include msg.UDPPacket and msg.Ping
	sendCh   chan msg.Message
	workConn net.Conn
	closed   bool
}

func NewUDPProxy(baseProxy *BaseProxy, cfg v1.ProxyConfigurer) Proxy {
	unwrapped, ok := cfg.(*v1.UDPProxyConfig)
	if !ok {
		return nil
	}
	return &UDPProxy{
		BaseProxy: baseProxy,
		cfg:       unwrapped,
	}
}

func (pxy *UDPProxy) Run() (err error) {
	pxy.localAddr, err = net.ResolveUDPAddr("udp", net.JoinHostPort(pxy.cfg.LocalIP, strconv.Itoa(pxy.cfg.LocalPort)))
	if err != nil {
		return
	}
	return
}

func (pxy *UDPProxy) Close() {
	pxy.mu.Lock()
	defer pxy.mu.Unlock()

	if !pxy.closed {
		pxy.closed = true
		if pxy.workConn != nil {
			pxy.workConn.Close()
		}
		if pxy.readCh != nil {
			close(pxy.readCh)
		}
		if pxy.sendCh != nil {
			close(pxy.sendCh)
		}
	}
}

func (pxy *UDPProxy) InWorkConn(conn net.Conn, _ *msg.StartWorkConn) {
	xl := pxy.xl
	xl.Infof("新的工作连接传入UDP协议, %s", conn.RemoteAddr().String())
	// 关闭与旧工作连接相关的资源
	pxy.Close()

	var rwc io.ReadWriteCloser = conn
	var err error
	if pxy.limiter != nil {
		rwc = libio.WrapReadWriteCloser(limit.NewReader(conn, pxy.limiter), limit.NewWriter(conn, pxy.limiter), func() error {
			return conn.Close()
		})
	}
	if pxy.cfg.Transport.UseEncryption {
		rwc, err = libio.WithEncryption(rwc, []byte(pxy.clientCfg.Auth.Token))
		if err != nil {
			conn.Close()
			xl.Errorf("创建加密流错误: %v", err)
			return
		}
	}
	if pxy.cfg.Transport.UseCompression {
		rwc = libio.WithCompression(rwc)
	}
	conn = netpkg.WrapReadWriteCloserToConn(rwc, conn)

	pxy.mu.Lock()
	pxy.workConn = conn
	pxy.readCh = make(chan *msg.UDPPacket, 1024)
	pxy.sendCh = make(chan msg.Message, 1024)
	pxy.closed = false
	pxy.mu.Unlock()

	workConnReaderFn := func(conn net.Conn, readCh chan *msg.UDPPacket) {
		for {
			var udpMsg msg.UDPPacket
			if errRet := msg.ReadMsgInto(conn, &udpMsg); errRet != nil {
				xl.Warnf("从工作连接读取UDP错误: %v", errRet)
				return
			}
			if errRet := errors.PanicToError(func() {
				xl.Tracef("从工作连接获取UDP包: %s", udpMsg.Content)
				readCh <- &udpMsg
			}); errRet != nil {
				xl.Infof("UDP工作连接读取器协程关闭: %v", errRet)
				return
			}
		}
	}
	workConnSenderFn := func(conn net.Conn, sendCh chan msg.Message) {
		defer func() {
			xl.Infof("UDP工作连接写入器协程关闭")
		}()
		var errRet error
		for rawMsg := range sendCh {
			switch m := rawMsg.(type) {
			case *msg.UDPPacket:
				xl.Tracef("发送UDP包到工作连接: %s", m.Content)
			case *msg.Ping:
				xl.Tracef("发送ping消息到UDP工作连接")
			}
			if errRet = msg.WriteMsg(conn, rawMsg); errRet != nil {
				xl.Errorf("UDP工作连接写入错误: %v", errRet)
				return
			}
		}
	}
	heartbeatFn := func(sendCh chan msg.Message) {
		var errRet error
		for {
			time.Sleep(time.Duration(30) * time.Second)
			if errRet = errors.PanicToError(func() {
				sendCh <- &msg.Ping{}
			}); errRet != nil {
				xl.Tracef("UDP工作连接心跳协程关闭")
				break
			}
		}
	}

	go workConnSenderFn(pxy.workConn, pxy.sendCh)
	go workConnReaderFn(pxy.workConn, pxy.readCh)
	go heartbeatFn(pxy.sendCh)
	udp.Forwarder(pxy.localAddr, pxy.readCh, pxy.sendCh, int(pxy.clientCfg.UDPPacketSize))
}
