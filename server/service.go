// Copyright 2017 fatedier, fatedier@gmail.com
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

package server

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/fatedier/golib/crypto"
	"github.com/fatedier/golib/net/mux"
	fmux "github.com/hashicorp/yamux"
	quic "github.com/quic-go/quic-go"
	"github.com/samber/lo"

	"github.com/fatedier/frp/pkg/auth"
	v1 "github.com/fatedier/frp/pkg/config/v1"
	modelmetrics "github.com/fatedier/frp/pkg/metrics"
	"github.com/fatedier/frp/pkg/msg"
	"github.com/fatedier/frp/pkg/nathole"
	plugin "github.com/fatedier/frp/pkg/plugin/server"
	"github.com/fatedier/frp/pkg/ssh"
	"github.com/fatedier/frp/pkg/transport"
	httppkg "github.com/fatedier/frp/pkg/util/http"
	"github.com/fatedier/frp/pkg/util/log"
	netpkg "github.com/fatedier/frp/pkg/util/net"
	"github.com/fatedier/frp/pkg/util/tcpmux"
	"github.com/fatedier/frp/pkg/util/util"
	"github.com/fatedier/frp/pkg/util/version"
	"github.com/fatedier/frp/pkg/util/vhost"
	"github.com/fatedier/frp/pkg/util/xlog"
	"github.com/fatedier/frp/server/controller"
	"github.com/fatedier/frp/server/group"
	"github.com/fatedier/frp/server/metrics"
	"github.com/fatedier/frp/server/ports"
	"github.com/fatedier/frp/server/proxy"
	"github.com/fatedier/frp/server/visitor"
)

const (
	connReadTimeout       time.Duration = 10 * time.Second
	vhostReadWriteTimeout time.Duration = 30 * time.Second
)

func init() {
	crypto.DefaultSalt = "frp"
	// Disable quic-go's receive buffer warning.
	os.Setenv("QUIC_GO_DISABLE_RECEIVE_BUFFER_WARNING", "true")
	// Disable quic-go's ECN support by default. It may cause issues on certain operating systems.
	if os.Getenv("QUIC_GO_DISABLE_ECN") == "" {
		os.Setenv("QUIC_GO_DISABLE_ECN", "true")
	}
}

// Server service
type Service struct {
	// Dispatch connections to different handlers listen on same port
	muxer *mux.Mux

	// Accept connections from client
	listener net.Listener

	// Accept connections using kcp
	kcpListener net.Listener

	// Accept connections using quic
	quicListener *quic.Listener

	// Accept connections using websocket
	websocketListener net.Listener

	// Accept frp tls connections
	tlsListener net.Listener

	// Accept pipe connections from ssh tunnel gateway
	sshTunnelListener *netpkg.InternalListener

	// Manage all controllers
	ctlManager *ControlManager

	// Manage all proxies
	pxyManager *proxy.Manager

	// Manage all plugins
	pluginManager *plugin.Manager

	// HTTP vhost router
	httpVhostRouter *vhost.Routers

	// All resource managers and controllers
	rc *controller.ResourceController

	// web server for dashboard UI and apis
	webServer *httppkg.Server

	sshTunnelGateway *ssh.Gateway

	// Verifies authentication based on selected method
	authVerifier auth.Verifier

	tlsConfig *tls.Config

	cfg *v1.ServerConfig

	// 黑名单，存储被踢出客户端的runID和封禁截止时间
	blacklist     map[string]time.Time
	blacklistLock sync.RWMutex

	// service context
	ctx context.Context
	// call cancel to stop service
	cancel context.CancelFunc
}

func NewService(cfg *v1.ServerConfig) (*Service, error) {
	tlsConfig, err := transport.NewServerTLSConfig(
		cfg.Transport.TLS.CertFile,
		cfg.Transport.TLS.KeyFile,
		cfg.Transport.TLS.TrustedCaFile)
	if err != nil {
		return nil, err
	}

	var webServer *httppkg.Server
	if cfg.WebServer.Port > 0 {
		ws, err := httppkg.NewServer(cfg.WebServer)
		if err != nil {
			return nil, err
		}
		webServer = ws

		modelmetrics.EnableMem()
		if cfg.EnablePrometheus {
			modelmetrics.EnablePrometheus()
		}
	}

	svr := &Service{
		ctlManager:    NewControlManager(),
		pxyManager:    proxy.NewManager(),
		pluginManager: plugin.NewManager(),
		rc: &controller.ResourceController{
			VisitorManager: visitor.NewManager(),
			TCPPortManager: ports.NewManager("tcp", cfg.ProxyBindAddr, cfg.AllowPorts),
			UDPPortManager: ports.NewManager("udp", cfg.ProxyBindAddr, cfg.AllowPorts),
		},
		sshTunnelListener: netpkg.NewInternalListener(),
		httpVhostRouter:   vhost.NewRouters(),
		authVerifier:      auth.NewAuthVerifier(cfg.Auth),
		webServer:         webServer,
		tlsConfig:         tlsConfig,
		cfg:               cfg,
		ctx:               context.Background(),
		blacklist:         make(map[string]time.Time),
	}
	if webServer != nil {
		webServer.RouteRegister(svr.registerRouteHandlers)
	}

	// Create tcpmux httpconnect multiplexer.
	if cfg.TCPMuxHTTPConnectPort > 0 {
		var l net.Listener
		address := net.JoinHostPort(cfg.ProxyBindAddr, strconv.Itoa(cfg.TCPMuxHTTPConnectPort))
		l, err = net.Listen("tcp", address)
		if err != nil {
			return nil, fmt.Errorf("创建服务器监听器错误, %v", err)
		}

		svr.rc.TCPMuxHTTPConnectMuxer, err = tcpmux.NewHTTPConnectTCPMuxer(l, cfg.TCPMuxPassthrough, vhostReadWriteTimeout)
		if err != nil {
			return nil, fmt.Errorf("创建 vhost tcpMuxer 错误, %v", err)
		}
		log.Infof("tcpmux httpconnect multiplexer 监听在 %s, 透传: %v", address, cfg.TCPMuxPassthrough)
	}

	// Init all plugins
	for _, p := range cfg.HTTPPlugins {
		svr.pluginManager.Register(plugin.NewHTTPPluginOptions(p))
		log.Infof("插件 [%s] 已注册", p.Name)
	}
	svr.rc.PluginManager = svr.pluginManager

	// Init group controller
	svr.rc.TCPGroupCtl = group.NewTCPGroupCtl(svr.rc.TCPPortManager)

	// Init HTTP group controller
	svr.rc.HTTPGroupCtl = group.NewHTTPGroupController(svr.httpVhostRouter)

	// Init TCP mux group controller
	svr.rc.TCPMuxGroupCtl = group.NewTCPMuxGroupCtl(svr.rc.TCPMuxHTTPConnectMuxer)

	// Init 404 not found page
	vhost.NotFoundPagePath = cfg.Custom404Page

	var (
		httpMuxOn  bool
		httpsMuxOn bool
	)
	if cfg.BindAddr == cfg.ProxyBindAddr {
		if cfg.BindPort == cfg.VhostHTTPPort {
			httpMuxOn = true
		}
		if cfg.BindPort == cfg.VhostHTTPSPort {
			httpsMuxOn = true
		}
	}

	// Listen for accepting connections from client.
	address := net.JoinHostPort(cfg.BindAddr, strconv.Itoa(cfg.BindPort))
	ln, err := net.Listen("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("创建服务器监听器错误, %v", err)
	}

	svr.muxer = mux.NewMux(ln)
	svr.muxer.SetKeepAlive(time.Duration(cfg.Transport.TCPKeepAlive) * time.Second)
	go func() {
		_ = svr.muxer.Serve()
	}()
	ln = svr.muxer.DefaultListener()

	svr.listener = ln
	log.Infof("frps %s 启动成功", address)

	// Listen for accepting connections from client using kcp protocol.
	if cfg.KCPBindPort > 0 {
		address := net.JoinHostPort(cfg.BindAddr, strconv.Itoa(cfg.KCPBindPort))
		svr.kcpListener, err = netpkg.ListenKcp(address)
		if err != nil {
			return nil, fmt.Errorf("监听 kcp 地址 %s 错误: %v", address, err)
		}
		log.Infof("frps kcp 监听在 %s", address)
	}

	if cfg.QUICBindPort > 0 {
		address := net.JoinHostPort(cfg.BindAddr, strconv.Itoa(cfg.QUICBindPort))
		quicTLSCfg := tlsConfig.Clone()
		quicTLSCfg.NextProtos = []string{"frp"}
		svr.quicListener, err = quic.ListenAddr(address, quicTLSCfg, &quic.Config{
			MaxIdleTimeout:     time.Duration(cfg.Transport.QUIC.MaxIdleTimeout) * time.Second,
			MaxIncomingStreams: int64(cfg.Transport.QUIC.MaxIncomingStreams),
			KeepAlivePeriod:    time.Duration(cfg.Transport.QUIC.KeepalivePeriod) * time.Second,
		})
		if err != nil {
			return nil, fmt.Errorf("监听 quic 地址 %s 错误: %v", address, err)
		}
		log.Infof("frps quic 监听在 %s", address)
	}

	if cfg.SSHTunnelGateway.BindPort > 0 {
		sshGateway, err := ssh.NewGateway(cfg.SSHTunnelGateway, cfg.ProxyBindAddr, svr.sshTunnelListener)
		if err != nil {
			return nil, fmt.Errorf("创建 ssh 网关错误: %v", err)
		}
		svr.sshTunnelGateway = sshGateway
		log.Infof("frps sshTunnelGateway 监听在端口 %d", cfg.SSHTunnelGateway.BindPort)
	}

	// Listen for accepting connections from client using websocket protocol.
	websocketPrefix := []byte("GET " + netpkg.FrpWebsocketPath)
	websocketLn := svr.muxer.Listen(0, uint32(len(websocketPrefix)), func(data []byte) bool {
		return bytes.Equal(data, websocketPrefix)
	})
	svr.websocketListener = netpkg.NewWebsocketListener(websocketLn)

	// Create http vhost muxer.
	if cfg.VhostHTTPPort > 0 {
		rp := vhost.NewHTTPReverseProxy(vhost.HTTPReverseProxyOptions{
			ResponseHeaderTimeoutS: cfg.VhostHTTPTimeout,
		}, svr.httpVhostRouter)
		svr.rc.HTTPReverseProxy = rp

		address := net.JoinHostPort(cfg.ProxyBindAddr, strconv.Itoa(cfg.VhostHTTPPort))
		server := &http.Server{
			Addr:              address,
			Handler:           rp,
			ReadHeaderTimeout: 60 * time.Second,
		}
		var l net.Listener
		if httpMuxOn {
			l = svr.muxer.ListenHTTP(1)
		} else {
			l, err = net.Listen("tcp", address)
			if err != nil {
				return nil, fmt.Errorf("create vhost http listener error, %v", err)
			}
		}
		go func() {
			_ = server.Serve(l)
		}()
		log.Infof("http 服务监听在 %s", address)
	}

	// Create https vhost muxer.
	if cfg.VhostHTTPSPort > 0 {
		var l net.Listener
		if httpsMuxOn {
			l = svr.muxer.ListenHTTPS(1)
		} else {
			address := net.JoinHostPort(cfg.ProxyBindAddr, strconv.Itoa(cfg.VhostHTTPSPort))
			l, err = net.Listen("tcp", address)
			if err != nil {
				return nil, fmt.Errorf("create server listener error, %v", err)
			}
			log.Infof("https 服务监听在 %s", address)
		}

		svr.rc.VhostHTTPSMuxer, err = vhost.NewHTTPSMuxer(l, vhostReadWriteTimeout)
		if err != nil {
			return nil, fmt.Errorf("create vhost httpsMuxer error, %v", err)
		}
	}

	// frp tls listener
	svr.tlsListener = svr.muxer.Listen(2, 1, func(data []byte) bool {
		// tls first byte can be 0x16 only when vhost https port is not same with bind port
		return int(data[0]) == netpkg.FRPTLSHeadByte || int(data[0]) == 0x16
	})

	// Create nat hole controller.
	nc, err := nathole.NewController(time.Duration(cfg.NatHoleAnalysisDataReserveHours) * time.Hour)
	if err != nil {
		return nil, fmt.Errorf("create nat hole controller error, %v", err)
	}
	svr.rc.NatHoleController = nc
	return svr, nil
}

func (svr *Service) Run(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	svr.ctx = ctx
	svr.cancel = cancel

	// 启动黑名单清理协程
	go svr.cleanBlacklist()

	// 注释掉连接池相关代码，因为无法编译
	// Create connection pool if specified.
	/*
		if svr.cfg.Transport.MaxPoolCount > 0 {
			connPool := connpool.NewPool()
			connPool.MaxCount = int(svr.cfg.Transport.MaxPoolCount)
			svr.rc.ConnPool = connPool
		}
	*/

	// run dashboard web server.
	if svr.webServer != nil {
		go func() {
			log.Infof("管理面板地址: %s", svr.webServer.Address())
			if err := svr.webServer.Run(); err != nil {
				log.Warnf("管理面板退出错误: %v", err)
			}
		}()
	}

	go svr.HandleListener(svr.sshTunnelListener, true)

	if svr.kcpListener != nil {
		go svr.HandleListener(svr.kcpListener, false)
	}
	if svr.quicListener != nil {
		go svr.HandleQUICListener(svr.quicListener)
	}
	go svr.HandleListener(svr.websocketListener, false)
	go svr.HandleListener(svr.tlsListener, false)

	if svr.rc.NatHoleController != nil {
		go svr.rc.NatHoleController.CleanWorker(svr.ctx)
	}

	if svr.sshTunnelGateway != nil {
		go svr.sshTunnelGateway.Run()
	}

	svr.HandleListener(svr.listener, false)

	<-svr.ctx.Done()
	// service context may not be canceled by svr.Close(), we should call it here to release resources
	if svr.listener != nil {
		svr.Close()
	}
}

func (svr *Service) Close() error {
	if svr.kcpListener != nil {
		svr.kcpListener.Close()
	}
	if svr.quicListener != nil {
		svr.quicListener.Close()
	}
	if svr.websocketListener != nil {
		svr.websocketListener.Close()
	}
	if svr.tlsListener != nil {
		svr.tlsListener.Close()
	}
	if svr.sshTunnelListener != nil {
		svr.sshTunnelListener.Close()
	}
	if svr.listener != nil {
		svr.listener.Close()
	}
	if svr.webServer != nil {
		svr.webServer.Close()
	}
	if svr.sshTunnelGateway != nil {
		svr.sshTunnelGateway.Close()
	}
	svr.rc.Close()
	svr.muxer.Close()
	svr.ctlManager.Close()
	if svr.cancel != nil {
		svr.cancel()
	}
	return nil
}

func (svr *Service) handleConnection(ctx context.Context, conn net.Conn, internal bool) {
	xl := xlog.FromContextSafe(ctx)

	var (
		rawMsg msg.Message
		err    error
	)

	_ = conn.SetReadDeadline(time.Now().Add(connReadTimeout))
	if rawMsg, err = msg.ReadMsg(conn); err != nil {
		log.Tracef("读取消息失败: %v", err)
		conn.Close()
		return
	}
	_ = conn.SetReadDeadline(time.Time{})

	switch m := rawMsg.(type) {
	case *msg.Login:
		// server plugin hook
		content := &plugin.LoginContent{
			Login:         *m,
			ClientAddress: conn.RemoteAddr().String(),
		}
		retContent, err := svr.pluginManager.Login(content)
		if err == nil {
			m = &retContent.Login
			err = svr.RegisterControl(conn, m, internal)
		}

		// If login failed, send error message there.
		// Otherwise send success message in control's work goroutine.
		// Reject 可以有也可以没有
		if err != nil {
			xl.Warnf("注册控制器失败: %v", err)
			_ = msg.WriteMsg(conn, &msg.LoginResp{
				Version: version.Full(),
				Error:   util.GenerateResponseErrorString("注册控制器失败", err, lo.FromPtr(svr.cfg.DetailedErrorsToClient)),
			})
			conn.Close()
		}
	case *msg.NewWorkConn:
		if err := svr.RegisterWorkConn(conn, m); err != nil {
			conn.Close()
		}
	case *msg.NewVisitorConn:
		if err = svr.RegisterVisitorConn(conn, m); err != nil {
			xl.Warnf("注册访问者连接失败: %v", err)
			_ = msg.WriteMsg(conn, &msg.NewVisitorConnResp{
				ProxyName: m.ProxyName,
				Error:     util.GenerateResponseErrorString("注册访问者连接失败", err, lo.FromPtr(svr.cfg.DetailedErrorsToClient)),
			})
			conn.Close()
		} else {
			_ = msg.WriteMsg(conn, &msg.NewVisitorConnResp{
				ProxyName: m.ProxyName,
				Error:     "",
			})
		}
	default:
		log.Warnf("新连接错误消息类型: %s", conn.RemoteAddr().String())
		conn.Close()
	}
}

// HandleListener accepts connections from client and call handleConnection to handle them.
// If internal is true, it means that this listener is used for internal communication like ssh tunnel gateway.
// TODO(fatedier): Pass some parameters of listener/connection through context to avoid passing too many parameters.
func (svr *Service) HandleListener(l net.Listener, internal bool) {
	// Listen for incoming connections from client.
	for {
		c, err := l.Accept()
		if err != nil {
			log.Warnf("客户端连接监听器关闭")
			return
		}
		// inject xlog object into net.Conn context
		xl := xlog.New()
		ctx := context.Background()

		c = netpkg.NewContextConn(xlog.NewContext(ctx, xl), c)

		if !internal {
			log.Tracef("开始检查TLS连接...")
			originConn := c
			forceTLS := svr.cfg.Transport.TLS.Force
			var isTLS, custom bool
			c, isTLS, custom, err = netpkg.CheckAndEnableTLSServerConnWithTimeout(c, svr.tlsConfig, forceTLS, connReadTimeout)
			if err != nil {
				log.Warnf("检查TLS连接失败: %v", err)
				originConn.Close()
				continue
			}
			log.Tracef("检查TLS连接成功, isTLS: %v custom: %v internal: %v", isTLS, custom, internal)
		}

		// Start a new goroutine to handle connection.
		go func(ctx context.Context, frpConn net.Conn) {
			if lo.FromPtr(svr.cfg.Transport.TCPMux) && !internal {
				fmuxCfg := fmux.DefaultConfig()
				fmuxCfg.KeepAliveInterval = time.Duration(svr.cfg.Transport.TCPMuxKeepaliveInterval) * time.Second
				fmuxCfg.LogOutput = io.Discard
				fmuxCfg.MaxStreamWindowSize = 6 * 1024 * 1024
				session, err := fmux.Server(frpConn, fmuxCfg)
				if err != nil {
					log.Warnf("创建多路复用连接失败: %v", err)
					frpConn.Close()
					return
				}

				for {
					stream, err := session.AcceptStream()
					if err != nil {
						log.Debugf("接受新多路复用流失败: %v", err)
						session.Close()
						return
					}
					go svr.handleConnection(ctx, stream, internal)
				}
			} else {
				svr.handleConnection(ctx, frpConn, internal)
			}
		}(ctx, c)
	}
}

func (svr *Service) HandleQUICListener(l *quic.Listener) {
	// Listen for incoming connections from client.
	for {
		c, err := l.Accept(context.Background())
		if err != nil {
			log.Warnf("QUIC监听器关闭")
			return
		}
		// Start a new goroutine to handle connection.
		go func(ctx context.Context, frpConn quic.Connection) {
			for {
				stream, err := frpConn.AcceptStream(context.Background())
				if err != nil {
					log.Debugf("接受新QUIC多路复用流失败: %v", err)
					_ = frpConn.CloseWithError(0, "")
					return
				}
				go svr.handleConnection(ctx, netpkg.QuicStreamToNetConn(stream, frpConn), false)
			}
		}(context.Background(), c)
	}
}

func (svr *Service) RegisterControl(ctlConn net.Conn, loginMsg *msg.Login, internal bool) error {
	// If client's RunID is empty, it's a new client, we just create a new controller.
	// Otherwise, we check if there is one controller has the same run id. If so, we release previous controller and start new one.
	var err error
	if loginMsg.RunID == "" {
		loginMsg.RunID, err = util.RandID()
		if err != nil {
			return err
		}
	}

	ctx := netpkg.NewContextFromConn(ctlConn)
	xl := xlog.FromContextSafe(ctx)
	xl.AppendPrefix(loginMsg.RunID)
	ctx = xlog.NewContext(ctx, xl)
	xl.Infof("客户端登录信息: IP [%s] 版本 [%s] 主机名 [%s] 操作系统 [%s] 架构 [%s]",
		ctlConn.RemoteAddr().String(), loginMsg.Version, loginMsg.Hostname, loginMsg.Os, loginMsg.Arch)

	// 检查客户端是否在黑名单中
	if svr.IsInBlacklist(loginMsg.RunID) {
		xl.Warnf("客户端 [%s] 在黑名单中，不允许连接", loginMsg.RunID)
		// 发送黑名单响应
		resp := &msg.LoginResp{
			Version: version.Full(),
			Error:   "您的客户端已被管理员暂时禁止连接",
		}
		err := msg.WriteMsg(ctlConn, resp)
		if err != nil {
			xl.Warnf("写入错误消息到客户端失败: %v", err)
		}
		return fmt.Errorf("客户端 [%s] 在黑名单中", loginMsg.RunID)
	}

	// Check auth.
	authVerifier := svr.authVerifier
	if internal && loginMsg.ClientSpec.AlwaysAuthPass {
		authVerifier = auth.AlwaysPassVerifier
	}
	if err := authVerifier.VerifyLogin(loginMsg); err != nil {
		return err
	}

	// TODO(fatedier): use SessionContext
	ctl, err := NewControl(ctx, svr.rc, svr.pxyManager, svr.pluginManager, authVerifier, ctlConn, !internal, loginMsg, svr.cfg)
	if err != nil {
		xl.Warnf("创建新控制器失败: %v", err)
		// don't return detailed errors to client
		return fmt.Errorf("创建控制器时发生意外错误")
	}
	if oldCtl := svr.ctlManager.Add(loginMsg.RunID, ctl); oldCtl != nil {
		oldCtl.WaitClosed()
	}

	ctl.Start()

	// for statistics
	metrics.Server.NewClient()

	go func() {
		// block until control closed
		ctl.WaitClosed()
		svr.ctlManager.Del(loginMsg.RunID, ctl)
	}()
	return nil
}

// RegisterWorkConn register a new work connection to control and proxies need it.
func (svr *Service) RegisterWorkConn(workConn net.Conn, newMsg *msg.NewWorkConn) error {
	xl := netpkg.NewLogFromConn(workConn)
	ctl, exist := svr.ctlManager.GetByID(newMsg.RunID)
	if !exist {
		xl.Warnf("未找到运行ID为 [%s] 的客户端控制", newMsg.RunID)
		return fmt.Errorf("未找到运行ID为 [%s] 的客户端控制", newMsg.RunID)
	}
	// server plugin hook
	content := &plugin.NewWorkConnContent{
		User: plugin.UserInfo{
			User:  ctl.loginMsg.User,
			Metas: ctl.loginMsg.Metas,
			RunID: ctl.loginMsg.RunID,
		},
		NewWorkConn: *newMsg,
	}
	retContent, err := svr.pluginManager.NewWorkConn(content)
	if err == nil {
		newMsg = &retContent.NewWorkConn
		// Check auth.
		err = ctl.authVerifier.VerifyNewWorkConn(newMsg)
	}
	if err != nil {
		xl.Warnf("无效的新工作连接，运行ID为 [%s]", newMsg.RunID)
		_ = msg.WriteMsg(workConn, &msg.StartWorkConn{
			Error: util.GenerateResponseErrorString("无效的新工作连接", err, lo.FromPtr(svr.cfg.DetailedErrorsToClient)),
		})
		return fmt.Errorf("无效的新工作连接，运行ID为 [%s]", newMsg.RunID)
	}
	return ctl.RegisterWorkConn(workConn)
}

func (svr *Service) RegisterVisitorConn(visitorConn net.Conn, newMsg *msg.NewVisitorConn) error {
	visitorUser := ""
	// TODO(deprecation): Compatible with old versions, can be without runID, user is empty. In later versions, it will be mandatory to include runID.
	// If runID is required, it is not compatible with versions prior to v0.50.0.
	if newMsg.RunID != "" {
		ctl, exist := svr.ctlManager.GetByID(newMsg.RunID)
		if !exist {
			return fmt.Errorf("未找到运行ID为 [%s] 的客户端控制", newMsg.RunID)
		}
		visitorUser = ctl.loginMsg.User
	}
	return svr.rc.VisitorManager.NewConn(newMsg.ProxyName, visitorConn, newMsg.Timestamp, newMsg.SignKey,
		newMsg.UseEncryption, newMsg.UseCompression, visitorUser)
}

// 添加客户端到黑名单
func (svr *Service) AddToBlacklist(runID string, duration time.Duration) {
	svr.blacklistLock.Lock()
	defer svr.blacklistLock.Unlock()

	expireTime := time.Now().Add(duration)
	svr.blacklist[runID] = expireTime
	log.Infof("添加客户端 [%s] 到黑名单，直到 %s", runID, expireTime.Format("2006-01-02 15:04:05"))
}

// 检查客户端是否在黑名单中
func (svr *Service) IsInBlacklist(runID string) bool {
	svr.blacklistLock.RLock()
	defer svr.blacklistLock.RUnlock()

	expireTime, ok := svr.blacklist[runID]
	if !ok {
		return false
	}

	// 如果已过期，返回false
	if time.Now().After(expireTime) {
		return false
	}

	log.Infof("客户端 [%s] 在黑名单中，直到 %s", runID, expireTime.Format("2006-01-02 15:04:05"))
	return true
}

// 清理过期的黑名单项
func (svr *Service) cleanBlacklist() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-svr.ctx.Done():
			return
		case <-ticker.C:
			svr.blacklistLock.Lock()

			now := time.Now()
			for runID, expireTime := range svr.blacklist {
				if now.After(expireTime) {
					delete(svr.blacklist, runID)
					log.Infof("从黑名单中移除客户端 [%s] (已过期)", runID)
				}
			}

			svr.blacklistLock.Unlock()
		}
	}
}
