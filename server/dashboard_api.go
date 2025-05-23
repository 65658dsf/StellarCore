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
	"cmp"
	"encoding/json"
	"net/http"
	"slices"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/fatedier/frp/pkg/config/types"
	v1 "github.com/fatedier/frp/pkg/config/v1"
	"github.com/fatedier/frp/pkg/metrics/mem"
	httppkg "github.com/fatedier/frp/pkg/util/http"
	"github.com/fatedier/frp/pkg/util/log"
	netpkg "github.com/fatedier/frp/pkg/util/net"
	"github.com/fatedier/frp/pkg/util/version"
)

type GeneralResponse struct {
	Code int
	Msg  string
}

func (svr *Service) registerRouteHandlers(helper *httppkg.RouterRegisterHelper) {
	helper.Router.HandleFunc("/healthz", svr.healthz)
	subRouter := helper.Router.NewRoute().Subrouter()

	subRouter.Use(helper.AuthMiddleware.Middleware)

	// metrics
	if svr.cfg.EnablePrometheus {
		subRouter.Handle("/metrics", promhttp.Handler())
	}

	// apis
	subRouter.HandleFunc("/api/serverinfo", svr.apiServerInfo).Methods("GET")
	subRouter.HandleFunc("/api/proxy/{type}", svr.apiProxyByType).Methods("GET")
	subRouter.HandleFunc("/api/proxy/{type}/{name}", svr.apiProxyByTypeAndName).Methods("GET")
	subRouter.HandleFunc("/api/traffic/{name}", svr.apiProxyTraffic).Methods("GET")
	subRouter.HandleFunc("/api/traffic", svr.apiAllProxiesTraffic).Methods("GET")
	subRouter.HandleFunc("/api/traffic/trend", svr.apiTrafficTrend).Methods("GET")
	subRouter.HandleFunc("/api/proxies", svr.deleteProxies).Methods("DELETE")
	subRouter.HandleFunc("/api/client/kick", svr.kickClient).Methods("POST")

	// view
	subRouter.Handle("/favicon.ico", http.FileServer(helper.AssetsFS)).Methods("GET")
	subRouter.PathPrefix("/static/").Handler(
		netpkg.MakeHTTPGzipHandler(http.StripPrefix("/static/", http.FileServer(helper.AssetsFS))),
	).Methods("GET")

	subRouter.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/static/", http.StatusMovedPermanently)
	})
}

type serverInfoResp struct {
	Version               string `json:"version"`
	BindPort              int    `json:"bindPort"`
	VhostHTTPPort         int    `json:"vhostHTTPPort"`
	VhostHTTPSPort        int    `json:"vhostHTTPSPort"`
	TCPMuxHTTPConnectPort int    `json:"tcpmuxHTTPConnectPort"`
	KCPBindPort           int    `json:"kcpBindPort"`
	QUICBindPort          int    `json:"quicBindPort"`
	SubdomainHost         string `json:"subdomainHost"`
	MaxPoolCount          int64  `json:"maxPoolCount"`
	MaxPortsPerClient     int64  `json:"maxPortsPerClient"`
	HeartBeatTimeout      int64  `json:"heartbeatTimeout"`
	AllowPortsStr         string `json:"allowPortsStr,omitempty"`
	TLSForce              bool   `json:"tlsForce,omitempty"`

	TotalTrafficIn  int64            `json:"totalTrafficIn"`
	TotalTrafficOut int64            `json:"totalTrafficOut"`
	CurConns        int64            `json:"curConns"`
	ClientCounts    int64            `json:"clientCounts"`
	ProxyTypeCounts map[string]int64 `json:"proxyTypeCount"`
}

// /healthz
func (svr *Service) healthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(200)
}

// /api/serverinfo
func (svr *Service) apiServerInfo(w http.ResponseWriter, r *http.Request) {
	res := GeneralResponse{Code: 200}
	defer func() {
		log.Infof("Http response [%s]: code [%d]", r.URL.Path, res.Code)
		w.WriteHeader(res.Code)
		if len(res.Msg) > 0 {
			_, _ = w.Write([]byte(res.Msg))
		}
	}()

	log.Infof("Http request: [%s]", r.URL.Path)
	serverStats := mem.StatsCollector.GetServer()
	svrResp := serverInfoResp{
		Version:               version.Full(),
		BindPort:              svr.cfg.BindPort,
		VhostHTTPPort:         svr.cfg.VhostHTTPPort,
		VhostHTTPSPort:        svr.cfg.VhostHTTPSPort,
		TCPMuxHTTPConnectPort: svr.cfg.TCPMuxHTTPConnectPort,
		KCPBindPort:           svr.cfg.KCPBindPort,
		QUICBindPort:          svr.cfg.QUICBindPort,
		SubdomainHost:         svr.cfg.SubDomainHost,
		MaxPoolCount:          svr.cfg.Transport.MaxPoolCount,
		MaxPortsPerClient:     svr.cfg.MaxPortsPerClient,
		HeartBeatTimeout:      svr.cfg.Transport.HeartbeatTimeout,
		AllowPortsStr:         types.PortsRangeSlice(svr.cfg.AllowPorts).String(),
		TLSForce:              svr.cfg.Transport.TLS.Force,

		TotalTrafficIn:  serverStats.TotalTrafficIn,
		TotalTrafficOut: serverStats.TotalTrafficOut,
		CurConns:        serverStats.CurConns,
		ClientCounts:    serverStats.ClientCounts,
		ProxyTypeCounts: serverStats.ProxyTypeCounts,
	}

	buf, _ := json.Marshal(&svrResp)
	res.Msg = string(buf)
}

type BaseOutConf struct {
	v1.ProxyBaseConfig
}

type TCPOutConf struct {
	BaseOutConf
	RemotePort int `json:"remotePort"`
}

type TCPMuxOutConf struct {
	BaseOutConf
	v1.DomainConfig
	Multiplexer     string `json:"multiplexer"`
	RouteByHTTPUser string `json:"routeByHTTPUser"`
}

type UDPOutConf struct {
	BaseOutConf
	RemotePort int `json:"remotePort"`
}

type HTTPOutConf struct {
	BaseOutConf
	v1.DomainConfig
	Locations         []string `json:"locations"`
	HostHeaderRewrite string   `json:"hostHeaderRewrite"`
}

type HTTPSOutConf struct {
	BaseOutConf
	v1.DomainConfig
}

type STCPOutConf struct {
	BaseOutConf
}

type XTCPOutConf struct {
	BaseOutConf
}

func getConfByType(proxyType string) any {
	switch v1.ProxyType(proxyType) {
	case v1.ProxyTypeTCP:
		return &TCPOutConf{}
	case v1.ProxyTypeTCPMUX:
		return &TCPMuxOutConf{}
	case v1.ProxyTypeUDP:
		return &UDPOutConf{}
	case v1.ProxyTypeHTTP:
		return &HTTPOutConf{}
	case v1.ProxyTypeHTTPS:
		return &HTTPSOutConf{}
	case v1.ProxyTypeSTCP:
		return &STCPOutConf{}
	case v1.ProxyTypeXTCP:
		return &XTCPOutConf{}
	default:
		return nil
	}
}

// Get proxy info.
type ProxyStatsInfo struct {
	Name            string `json:"name"`
	RunID           string `json:"runId"`
	Conf            any    `json:"conf"`
	ClientVersion   string `json:"clientVersion,omitempty"`
	TodayTrafficIn  int64  `json:"todayTrafficIn"`
	TodayTrafficOut int64  `json:"todayTrafficOut"`
	CurConns        int64  `json:"curConns"`
	LastStartTime   string `json:"lastStartTime"`
	LastCloseTime   string `json:"lastCloseTime"`
	Status          string `json:"status"`
}

type GetProxyInfoResp struct {
	Proxies []*ProxyStatsInfo `json:"proxies"`
}

// /api/proxy/:type
func (svr *Service) apiProxyByType(w http.ResponseWriter, r *http.Request) {
	res := GeneralResponse{Code: 200}
	params := mux.Vars(r)
	proxyType := params["type"]

	defer func() {
		log.Infof("Http response [%s]: code [%d]", r.URL.Path, res.Code)
		w.WriteHeader(res.Code)
		if len(res.Msg) > 0 {
			_, _ = w.Write([]byte(res.Msg))
		}
	}()
	log.Infof("Http request: [%s]", r.URL.Path)

	proxyInfoResp := GetProxyInfoResp{}
	proxyInfoResp.Proxies = svr.getProxyStatsByType(proxyType)
	slices.SortFunc(proxyInfoResp.Proxies, func(a, b *ProxyStatsInfo) int {
		return cmp.Compare(a.Name, b.Name)
	})

	buf, _ := json.Marshal(&proxyInfoResp)
	res.Msg = string(buf)
}

func (svr *Service) getProxyStatsByType(proxyType string) (proxyInfos []*ProxyStatsInfo) {
	proxyStats := mem.StatsCollector.GetProxiesByType(proxyType)
	proxyInfos = make([]*ProxyStatsInfo, 0, len(proxyStats))
	for _, ps := range proxyStats {
		proxyInfo := &ProxyStatsInfo{}
		if pxy, ok := svr.pxyManager.GetByName(ps.Name); ok {
			content, err := json.Marshal(pxy.GetConfigurer())
			if err != nil {
				log.Warnf("解析隧道配置错误: %v", err)
				continue
			}
			proxyInfo.Conf = getConfByType(ps.Type)
			if err = json.Unmarshal(content, &proxyInfo.Conf); err != nil {
				log.Warnf("解析隧道配置错误: %v", err)
				continue
			}
			proxyInfo.Status = "online"
			if msg := pxy.GetLoginMsg(); msg != nil {
				proxyInfo.RunID = msg.RunID
			}
		} else {
			proxyInfo.Status = "offline"
		}
		proxyInfo.Name = ps.Name
		proxyInfo.TodayTrafficIn = ps.TodayTrafficIn
		proxyInfo.TodayTrafficOut = ps.TodayTrafficOut
		proxyInfo.CurConns = ps.CurConns
		proxyInfo.LastStartTime = ps.LastStartTime
		proxyInfo.LastCloseTime = ps.LastCloseTime
		if pxy, ok := svr.pxyManager.GetByName(ps.Name); ok && pxy.GetLoginMsg() != nil {
			proxyInfo.ClientVersion = pxy.GetLoginMsg().Version
		}

		proxyInfos = append(proxyInfos, proxyInfo)
	}
	return
}

// GET /api/proxy/{type}/{name}
type GetProxyStatsResp struct {
	Name            string `json:"name"`
	RunID           string `json:"runId"`
	Conf            any    `json:"conf"`
	TodayTrafficIn  int64  `json:"todayTrafficIn"`
	TodayTrafficOut int64  `json:"todayTrafficOut"`
	CurConns        int64  `json:"curConns"`
	LastStartTime   string `json:"lastStartTime"`
	LastCloseTime   string `json:"lastCloseTime"`
	Status          string `json:"status"`
}

func (svr *Service) apiProxyByTypeAndName(w http.ResponseWriter, r *http.Request) {
	res := GeneralResponse{Code: 200}
	params := mux.Vars(r)
	proxyType := params["type"]
	name := params["name"]

	defer func() {
		log.Infof("Http response [%s]: code [%d]", r.URL.Path, res.Code)
		w.WriteHeader(res.Code)
		if len(res.Msg) > 0 {
			_, _ = w.Write([]byte(res.Msg))
		}
	}()
	log.Infof("Http request: [%s]", r.URL.Path)

	var proxyStatsResp GetProxyStatsResp
	proxyStatsResp, res.Code, res.Msg = svr.getProxyStatsByTypeAndName(proxyType, name)
	if res.Code != 200 {
		return
	}

	buf, _ := json.Marshal(&proxyStatsResp)
	res.Msg = string(buf)
}

func (svr *Service) getProxyStatsByTypeAndName(proxyType string, proxyName string) (proxyInfo GetProxyStatsResp, code int, msg string) {
	proxyInfo.Name = proxyName
	ps := mem.StatsCollector.GetProxiesByTypeAndName(proxyType, proxyName)
	if ps == nil {
		code = 404
		msg = "未找到隧道信息"
		return
	}
	if pxy, ok := svr.pxyManager.GetByName(proxyName); ok {
		content, err := json.Marshal(pxy.GetConfigurer())
		if err != nil {
			log.Warnf("解析隧道配置错误: %v", err)
			code = 400
			msg = "解析配置错误"
			return
		}
		proxyInfo.Conf = getConfByType(ps.Type)
		if err = json.Unmarshal(content, &proxyInfo.Conf); err != nil {
			log.Warnf("解析隧道配置错误: %v", err)
			code = 400
			msg = "解析配置错误"
			return
		}
		proxyInfo.Status = "online"
		if msg := pxy.GetLoginMsg(); msg != nil {
			proxyInfo.RunID = msg.RunID
		}
	} else {
		proxyInfo.Status = "offline"
	}
	proxyInfo.TodayTrafficIn = ps.TodayTrafficIn
	proxyInfo.TodayTrafficOut = ps.TodayTrafficOut
	proxyInfo.CurConns = ps.CurConns
	proxyInfo.LastStartTime = ps.LastStartTime
	proxyInfo.LastCloseTime = ps.LastCloseTime
	code = 200
	return
}

// GET /api/traffic/:name
type GetProxyTrafficResp struct {
	Name       string  `json:"name"`
	TrafficIn  []int64 `json:"trafficIn"`
	TrafficOut []int64 `json:"trafficOut"`
}

func (svr *Service) apiProxyTraffic(w http.ResponseWriter, r *http.Request) {
	res := GeneralResponse{Code: 200}
	params := mux.Vars(r)
	name := params["name"]

	defer func() {
		log.Infof("Http response [%s]: code [%d]", r.URL.Path, res.Code)
		w.WriteHeader(res.Code)
		if len(res.Msg) > 0 {
			_, _ = w.Write([]byte(res.Msg))
		}
	}()
	log.Infof("Http request: [%s]", r.URL.Path)

	trafficResp := GetProxyTrafficResp{
		Name: name,
	}
	metrics := mem.StatsCollector.GetProxyTraffic(name)

	if metrics == nil {
		res.Code = 404
		res.Msg = "未找到隧道信息"
		return
	}

	trafficResp.TrafficIn = metrics.TrafficIn
	trafficResp.TrafficOut = metrics.TrafficOut

	buf, _ := json.Marshal(&trafficResp)
	res.Msg = string(buf)
}

func (svr *Service) deleteProxies(w http.ResponseWriter, r *http.Request) {
	res := GeneralResponse{Code: 200}

	defer func() {
		log.Infof("Http response [%s]: code [%d]", r.URL.Path, res.Code)
		w.WriteHeader(res.Code)
		if len(res.Msg) > 0 {
			_, _ = w.Write([]byte(res.Msg))
		}
	}()

	log.Infof("Http request: [%s]", r.URL.Path)
	status := r.URL.Query().Get("status")
	if status != "offline" {
		res.Code = 400
		res.Msg = "status only support offline"
		return
	}
	cleared, total := mem.StatsCollector.ClearOfflineProxies()
	log.Infof("cleared [%d] offline proxies, total [%d] proxies", cleared, total)
}

// POST /api/client/kick
type CloseUserResp struct {
	Status int    `json:"status"`
	Msg    string `json:"message"`
}

func (svr *Service) kickClient(w http.ResponseWriter, r *http.Request) {
	var (
		buf  []byte
		resp = CloseUserResp{}
	)
	var bodyMap struct {
		RunID string `json:"runId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&bodyMap); err != nil {
		resp.Status = 400
		resp.Msg = "请求错误"
		buf, _ = json.Marshal(&resp)
		w.Write(buf)
		return
	}
	runId := bodyMap.RunID
	log.Infof("Http request: [%s] kick runId [%s]", r.URL.Path, runId)
	defer func() {
		log.Infof("Http response [%s]: code [%d]", r.URL.Path, resp.Status)
	}()

	if ctl, ok := svr.ctlManager.GetByID(runId); ok {
		// 将客户端加入黑名单，禁止30分钟内重连
		svr.AddToBlacklist(runId, 30*time.Minute)
		// 关闭连接
		ctl.Close()
		resp.Status = 200
		resp.Msg = "success"
	} else {
		resp.Status = 404
		resp.Msg = "该隧道未运行"
	}

	buf, _ = json.Marshal(&resp)
	w.Write(buf)
	return
}

// GET /api/traffic
// 获取所有隧道的流量数据聚合
func (svr *Service) apiAllProxiesTraffic(w http.ResponseWriter, r *http.Request) {
	res := GeneralResponse{Code: 200}

	defer func() {
		log.Infof("Http response [%s]: code [%d]", r.URL.Path, res.Code)
		w.WriteHeader(res.Code)
		if len(res.Msg) > 0 {
			_, _ = w.Write([]byte(res.Msg))
		}
	}()
	log.Infof("Http request: [%s]", r.URL.Path)

	// 获取所有隧道类型的隧道
	var allProxies []string
	proxyTypes := []string{"tcp", "udp", "http", "https", "stcp", "sudp", "xtcp", "tcpmux"}

	for _, proxyType := range proxyTypes {
		proxiesOfType := mem.StatsCollector.GetProxiesByType(proxyType)
		for _, proxy := range proxiesOfType {
			allProxies = append(allProxies, proxy.Name)
		}
	}

	// 聚合数据结构
	trafficResp := GetProxyTrafficResp{
		Name:       "all",
		TrafficIn:  make([]int64, 30), // 默认30个时间点
		TrafficOut: make([]int64, 30), // 默认30个时间点
	}

	// 收集并合并所有隧道的流量数据
	for _, proxyName := range allProxies {
		metrics := mem.StatsCollector.GetProxyTraffic(proxyName)
		if metrics == nil {
			continue
		}

		// 确保长度一致
		for i := 0; i < len(metrics.TrafficIn) && i < len(trafficResp.TrafficIn); i++ {
			trafficResp.TrafficIn[i] += metrics.TrafficIn[i]
		}

		for i := 0; i < len(metrics.TrafficOut) && i < len(trafficResp.TrafficOut); i++ {
			trafficResp.TrafficOut[i] += metrics.TrafficOut[i]
		}
	}

	buf, _ := json.Marshal(&trafficResp)
	res.Msg = string(buf)
}

// GET /api/traffic/trend
// 流量趋势数据
type TrafficTrendResp struct {
	Timestamps []string `json:"timestamps"`
	InData     []int64  `json:"inData"`
	OutData    []int64  `json:"outData"`
}

func (svr *Service) apiTrafficTrend(w http.ResponseWriter, r *http.Request) {
	res := GeneralResponse{Code: 200}

	defer func() {
		log.Infof("Http response [%s]: code [%d]", r.URL.Path, res.Code)
		w.WriteHeader(res.Code)
		if len(res.Msg) > 0 {
			_, _ = w.Write([]byte(res.Msg))
		}
	}()
	log.Infof("Http request: [%s]", r.URL.Path)

	// 获取时间范围参数
	timeRange := r.URL.Query().Get("range")
	if timeRange == "" {
		timeRange = "day" // 默认为一天
	}

	// 确定时间点数量
	var pointsCount int
	var interval time.Duration

	switch timeRange {
	case "day":
		pointsCount = 24
		interval = time.Hour
	case "3days":
		pointsCount = 72
		interval = time.Hour
	case "week":
		pointsCount = 7
		interval = 24 * time.Hour
	case "14days":
		pointsCount = 14
		interval = 24 * time.Hour
	case "month":
		pointsCount = 30
		interval = 24 * time.Hour
	default:
		pointsCount = 24
		interval = time.Hour
	}

	// 准备响应数据
	now := time.Now()
	trendResp := TrafficTrendResp{
		Timestamps: make([]string, pointsCount),
		InData:     make([]int64, pointsCount),
		OutData:    make([]int64, pointsCount),
	}

	// 获取所有隧道
	var allProxies []string
	proxyTypes := []string{"tcp", "udp", "http", "https", "stcp", "sudp", "xtcp", "tcpmux"}

	for _, proxyType := range proxyTypes {
		proxiesOfType := mem.StatsCollector.GetProxiesByType(proxyType)
		for _, proxy := range proxiesOfType {
			allProxies = append(allProxies, proxy.Name)
		}
	}

	// 生成时间戳
	for i := 0; i < pointsCount; i++ {
		timePoint := now.Add(-time.Duration(pointsCount-1-i) * interval)
		if interval >= 24*time.Hour {
			trendResp.Timestamps[i] = timePoint.Format("01-02")
		} else {
			trendResp.Timestamps[i] = timePoint.Format("01-02 15:04")
		}

		// 收集这个时间点所有隧道的流量数据
		for _, proxyName := range allProxies {
			metrics := mem.StatsCollector.GetProxyTraffic(proxyName)
			if metrics == nil || i >= len(metrics.TrafficIn) || i >= len(metrics.TrafficOut) {
				continue
			}

			trendResp.InData[i] += metrics.TrafficIn[i]
			trendResp.OutData[i] += metrics.TrafficOut[i]
		}
	}

	buf, _ := json.Marshal(&trendResp)
	res.Msg = string(buf)
}
