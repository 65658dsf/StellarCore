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

package api

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/65658dsf/StellarCore/pkg/config/types"
	v1 "github.com/65658dsf/StellarCore/pkg/config/v1"
	"github.com/65658dsf/StellarCore/pkg/metrics/mem"
	httppkg "github.com/65658dsf/StellarCore/pkg/util/http"
	"github.com/65658dsf/StellarCore/pkg/util/log"
	"github.com/65658dsf/StellarCore/pkg/util/version"
	"github.com/65658dsf/StellarCore/server/proxy"
	"github.com/65658dsf/StellarCore/server/registry"
)

type KickClientFunc func(runID string) (found bool, err error)
type ConfigReader func() ([]byte, error)
type RestartServiceFunc func() error
type UpdateServiceFunc func(context.Context) (UpdateResp, error)

type Controller struct {
	serverCfg      *v1.ServerConfig
	clientRegistry *registry.ClientRegistry
	pxyManager     ProxyManager
	kickClient     KickClientFunc
	readConfig     ConfigReader
	restartService RestartServiceFunc
	updateService  UpdateServiceFunc
}

type ControllerParams struct {
	ServerCfg      *v1.ServerConfig
	ClientRegistry *registry.ClientRegistry
	ProxyManager   ProxyManager
	KickClient     KickClientFunc
	ReadConfig     ConfigReader
	RestartService RestartServiceFunc
	UpdateService  UpdateServiceFunc
}

type ProxyManager interface {
	GetByName(name string) (proxy.Proxy, bool)
}

func NewController(params ControllerParams) *Controller {
	return &Controller{
		serverCfg:      params.ServerCfg,
		clientRegistry: params.ClientRegistry,
		pxyManager:     params.ProxyManager,
		kickClient:     params.KickClient,
		readConfig:     params.ReadConfig,
		restartService: params.RestartService,
		updateService:  params.UpdateService,
	}
}

func (c *Controller) Healthz(_ *httppkg.Context) (any, error) {
	return nil, nil
}

func (c *Controller) APIServerInfo(_ *httppkg.Context) (any, error) {
	serverStats := mem.StatsCollector.GetServer()
	return ServerInfoResp{
		Version:               version.Full(),
		BindPort:              c.serverCfg.BindPort,
		VhostHTTPPort:         c.serverCfg.VhostHTTPPort,
		VhostHTTPSPort:        c.serverCfg.VhostHTTPSPort,
		TCPMuxHTTPConnectPort: c.serverCfg.TCPMuxHTTPConnectPort,
		KCPBindPort:           c.serverCfg.KCPBindPort,
		QUICBindPort:          c.serverCfg.QUICBindPort,
		SubdomainHost:         c.serverCfg.SubDomainHost,
		MaxPoolCount:          c.serverCfg.Transport.MaxPoolCount,
		MaxPortsPerClient:     c.serverCfg.MaxPortsPerClient,
		HeartBeatTimeout:      c.serverCfg.Transport.HeartbeatTimeout,
		AllowPortsStr:         types.PortsRangeSlice(c.serverCfg.AllowPorts).String(),
		TLSForce:              c.serverCfg.Transport.TLS.Force,
		TotalTrafficIn:        serverStats.TotalTrafficIn,
		TotalTrafficOut:       serverStats.TotalTrafficOut,
		CurConns:              serverStats.CurConns,
		ClientCounts:          serverStats.ClientCounts,
		ProxyTypeCounts:       serverStats.ProxyTypeCounts,
	}, nil
}

func (c *Controller) APIClientList(ctx *httppkg.Context) (any, error) {
	if c.clientRegistry == nil {
		return nil, fmt.Errorf("client registry unavailable")
	}

	userFilter := ctx.Query("user")
	clientIDFilter := ctx.Query("clientId")
	runIDFilter := ctx.Query("runId")
	statusFilter := strings.ToLower(ctx.Query("status"))

	records := c.clientRegistry.List()
	items := make([]ClientInfoResp, 0, len(records))
	for _, info := range records {
		if userFilter != "" && info.User != userFilter {
			continue
		}
		if clientIDFilter != "" && info.ClientID() != clientIDFilter {
			continue
		}
		if runIDFilter != "" && info.RunID != runIDFilter {
			continue
		}
		if !matchStatusFilter(info.Online, statusFilter) {
			continue
		}
		items = append(items, buildClientInfoResp(info))
	}

	slices.SortFunc(items, func(a, b ClientInfoResp) int {
		if v := cmp.Compare(a.User, b.User); v != 0 {
			return v
		}
		if v := cmp.Compare(a.ClientID, b.ClientID); v != 0 {
			return v
		}
		return cmp.Compare(a.Key, b.Key)
	})

	return items, nil
}

func (c *Controller) APIClientDetail(ctx *httppkg.Context) (any, error) {
	key := ctx.Param("key")
	if key == "" {
		return nil, fmt.Errorf("missing client key")
	}
	if c.clientRegistry == nil {
		return nil, fmt.Errorf("client registry unavailable")
	}

	info, ok := c.clientRegistry.GetByKey(key)
	if !ok {
		return nil, httppkg.NewError(http.StatusNotFound, fmt.Sprintf("client %s not found", key))
	}
	return buildClientInfoResp(info), nil
}

func (c *Controller) APIProxyByType(ctx *httppkg.Context) (any, error) {
	proxyType := ctx.Param("type")
	resp := GetProxyInfoResp{
		Proxies: c.getProxyStatsByType(proxyType),
	}
	slices.SortFunc(resp.Proxies, func(a, b *ProxyStatsInfo) int {
		return cmp.Compare(a.Name, b.Name)
	})
	return resp, nil
}

func (c *Controller) APIProxyByTypeAndName(ctx *httppkg.Context) (any, error) {
	proxyType := ctx.Param("type")
	name := ctx.Param("name")

	proxyStatsResp, code, msg := c.getProxyStatsByTypeAndName(proxyType, name)
	if code != http.StatusOK {
		return nil, httppkg.NewError(code, msg)
	}
	return proxyStatsResp, nil
}

func (c *Controller) APIProxyByName(ctx *httppkg.Context) (any, error) {
	name := ctx.Param("name")
	ps := mem.StatsCollector.GetProxyByName(name)
	if ps == nil {
		return nil, httppkg.NewError(http.StatusNotFound, "no proxy info found")
	}

	proxyInfo := GetProxyStatsResp{
		Name:            ps.Name,
		TodayTrafficIn:  ps.TodayTrafficIn,
		TodayTrafficOut: ps.TodayTrafficOut,
		CurConns:        ps.CurConns,
		LastStartTime:   ps.LastStartTime,
		LastCloseTime:   ps.LastCloseTime,
	}

	if pxy, ok := c.pxyManager.GetByName(name); ok {
		content, err := json.Marshal(pxy.GetConfigurer())
		if err != nil {
			log.Warnf("marshal proxy [%s] conf info error: %v", name, err)
			return nil, httppkg.NewError(http.StatusBadRequest, "parse conf error")
		}
		proxyInfo.Conf = getConfByType(ps.Type)
		if err = json.Unmarshal(content, &proxyInfo.Conf); err != nil {
			log.Warnf("unmarshal proxy [%s] conf info error: %v", name, err)
			return nil, httppkg.NewError(http.StatusBadRequest, "parse conf error")
		}
		proxyInfo.Status = "online"
		c.fillProxyClientInfo(&proxyClientInfo{
			user:          &proxyInfo.User,
			clientID:      &proxyInfo.ClientID,
			clientVersion: &proxyInfo.ClientVersion,
			runID:         &proxyInfo.RunID,
		}, pxy)
	} else {
		proxyInfo.Status = "offline"
	}

	return proxyInfo, nil
}

func (c *Controller) DeleteProxies(ctx *httppkg.Context) (any, error) {
	status := ctx.Query("status")
	if status != "offline" {
		return nil, httppkg.NewError(http.StatusBadRequest, "status only support offline")
	}
	cleared, total := mem.StatsCollector.ClearOfflineProxies()
	log.Infof("cleared [%d] offline proxies, total [%d] proxies", cleared, total)
	return nil, nil
}

func (c *Controller) APIProxyTraffic(ctx *httppkg.Context) (any, error) {
	name := ctx.Param("name")
	trafficResp := GetProxyTrafficResp{Name: name}
	proxyTrafficInfo := mem.StatsCollector.GetProxyTraffic(name)
	if proxyTrafficInfo == nil {
		return nil, httppkg.NewError(http.StatusNotFound, "no proxy info found")
	}
	trafficResp.TrafficIn = proxyTrafficInfo.TrafficIn
	trafficResp.TrafficOut = proxyTrafficInfo.TrafficOut
	return trafficResp, nil
}

func (c *Controller) APIAllProxiesTraffic(_ *httppkg.Context) (any, error) {
	trafficResp := GetProxyTrafficResp{
		Name:       "all",
		TrafficIn:  make([]int64, 30),
		TrafficOut: make([]int64, 30),
	}

	for _, proxyName := range c.listAllProxyNames() {
		metrics := mem.StatsCollector.GetProxyTraffic(proxyName)
		if metrics == nil {
			continue
		}
		for i := 0; i < len(metrics.TrafficIn) && i < len(trafficResp.TrafficIn); i++ {
			trafficResp.TrafficIn[i] += metrics.TrafficIn[i]
		}
		for i := 0; i < len(metrics.TrafficOut) && i < len(trafficResp.TrafficOut); i++ {
			trafficResp.TrafficOut[i] += metrics.TrafficOut[i]
		}
	}
	return trafficResp, nil
}

func (c *Controller) APITrafficTrend(ctx *httppkg.Context) (any, error) {
	timeRange := ctx.Query("range")
	if timeRange == "" {
		timeRange = "day"
	}

	var (
		pointsCount int
		interval    time.Duration
	)
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

	now := time.Now()
	trendResp := TrafficTrendResp{
		Timestamps: make([]string, pointsCount),
		InData:     make([]int64, pointsCount),
		OutData:    make([]int64, pointsCount),
	}

	for i := 0; i < pointsCount; i++ {
		timePoint := now.Add(-time.Duration(pointsCount-1-i) * interval)
		if interval >= 24*time.Hour {
			trendResp.Timestamps[i] = timePoint.Format("01-02")
		} else {
			trendResp.Timestamps[i] = timePoint.Format("01-02 15:04")
		}

		for _, proxyName := range c.listAllProxyNames() {
			metrics := mem.StatsCollector.GetProxyTraffic(proxyName)
			if metrics == nil || i >= len(metrics.TrafficIn) || i >= len(metrics.TrafficOut) {
				continue
			}
			trendResp.InData[i] += metrics.TrafficIn[i]
			trendResp.OutData[i] += metrics.TrafficOut[i]
		}
	}
	return trendResp, nil
}

func (c *Controller) KickClient(ctx *httppkg.Context) (any, error) {
	resp := CloseUserResp{}
	if c.kickClient == nil {
		resp.Status = http.StatusNotImplemented
		resp.Msg = "kick client is unavailable"
		return resp, nil
	}

	var bodyMap struct {
		RunID string `json:"runId"`
	}
	if err := ctx.BindJSON(&bodyMap); err != nil || bodyMap.RunID == "" {
		resp.Status = http.StatusBadRequest
		resp.Msg = "request error"
		return resp, nil
	}

	found, err := c.kickClient(bodyMap.RunID)
	if err != nil {
		resp.Status = http.StatusInternalServerError
		resp.Msg = err.Error()
		return resp, nil
	}
	if !found {
		resp.Status = http.StatusNotFound
		resp.Msg = "client not running"
		return resp, nil
	}

	resp.Status = http.StatusOK
	resp.Msg = "success"
	return resp, nil
}

func (c *Controller) GetConfig(ctx *httppkg.Context) (any, error) {
	if c.readConfig == nil {
		return nil, httppkg.NewError(http.StatusBadRequest, "frps has no config file path")
	}

	content, err := c.readConfig()
	if err != nil {
		return nil, err
	}
	ctx.Resp.Header().Set("Content-Type", "text/plain; charset=utf-8")
	return string(content), nil
}

func (c *Controller) RestartService(_ *httppkg.Context) (any, error) {
	if c.restartService == nil {
		return nil, httppkg.NewError(http.StatusNotImplemented, "restart is not supported on this platform")
	}
	if err := c.restartService(); err != nil {
		return nil, err
	}
	return nil, nil
}

func (c *Controller) GetLogs(ctx *httppkg.Context) (any, error) {
	cursor, err := parseOptionalInt64Query(ctx, "cursor")
	if err != nil {
		return nil, err
	}
	limit, err := parseOptionalIntQuery(ctx, "limit")
	if err != nil {
		return nil, err
	}
	return log.QueryEntries(cursor, limit, ctx.Query("level")), nil
}

func (c *Controller) UpdateService(ctx *httppkg.Context) (any, error) {
	if c.updateService == nil {
		return nil, httppkg.NewError(http.StatusNotImplemented, "update is not supported on this platform")
	}
	return c.updateService(ctx.Req.Context())
}

func (c *Controller) getProxyStatsByType(proxyType string) (proxyInfos []*ProxyStatsInfo) {
	proxyStats := mem.StatsCollector.GetProxiesByType(proxyType)
	proxyInfos = make([]*ProxyStatsInfo, 0, len(proxyStats))
	for _, ps := range proxyStats {
		proxyInfo := &ProxyStatsInfo{}
		if pxy, ok := c.pxyManager.GetByName(ps.Name); ok {
			content, err := json.Marshal(pxy.GetConfigurer())
			if err != nil {
				log.Warnf("marshal proxy [%s] conf info error: %v", ps.Name, err)
				continue
			}
			proxyInfo.Conf = getConfByType(ps.Type)
			if err = json.Unmarshal(content, &proxyInfo.Conf); err != nil {
				log.Warnf("unmarshal proxy [%s] conf info error: %v", ps.Name, err)
				continue
			}
			proxyInfo.Status = "online"
			c.fillProxyClientInfo(&proxyClientInfo{
				user:          &proxyInfo.User,
				clientID:      &proxyInfo.ClientID,
				clientVersion: &proxyInfo.ClientVersion,
				runID:         &proxyInfo.RunID,
			}, pxy)
		} else {
			proxyInfo.Status = "offline"
		}
		proxyInfo.Name = ps.Name
		proxyInfo.TodayTrafficIn = ps.TodayTrafficIn
		proxyInfo.TodayTrafficOut = ps.TodayTrafficOut
		proxyInfo.CurConns = ps.CurConns
		proxyInfo.LastStartTime = ps.LastStartTime
		proxyInfo.LastCloseTime = ps.LastCloseTime
		proxyInfos = append(proxyInfos, proxyInfo)
	}
	return
}

func (c *Controller) getProxyStatsByTypeAndName(proxyType string, proxyName string) (proxyInfo GetProxyStatsResp, code int, msg string) {
	proxyInfo.Name = proxyName
	ps := mem.StatsCollector.GetProxiesByTypeAndName(proxyType, proxyName)
	if ps == nil {
		return proxyInfo, http.StatusNotFound, "no proxy info found"
	}

	if pxy, ok := c.pxyManager.GetByName(proxyName); ok {
		content, err := json.Marshal(pxy.GetConfigurer())
		if err != nil {
			log.Warnf("marshal proxy [%s] conf info error: %v", ps.Name, err)
			return proxyInfo, http.StatusBadRequest, "parse conf error"
		}
		proxyInfo.Conf = getConfByType(ps.Type)
		if err = json.Unmarshal(content, &proxyInfo.Conf); err != nil {
			log.Warnf("unmarshal proxy [%s] conf info error: %v", ps.Name, err)
			return proxyInfo, http.StatusBadRequest, "parse conf error"
		}
		proxyInfo.Status = "online"
		c.fillProxyClientInfo(&proxyClientInfo{
			user:          &proxyInfo.User,
			clientID:      &proxyInfo.ClientID,
			clientVersion: &proxyInfo.ClientVersion,
			runID:         &proxyInfo.RunID,
		}, pxy)
	} else {
		proxyInfo.Status = "offline"
	}

	proxyInfo.TodayTrafficIn = ps.TodayTrafficIn
	proxyInfo.TodayTrafficOut = ps.TodayTrafficOut
	proxyInfo.CurConns = ps.CurConns
	proxyInfo.LastStartTime = ps.LastStartTime
	proxyInfo.LastCloseTime = ps.LastCloseTime
	return proxyInfo, http.StatusOK, ""
}

func buildClientInfoResp(info registry.ClientInfo) ClientInfoResp {
	resp := ClientInfoResp{
		Key:              info.Key,
		User:             info.User,
		ClientID:         info.ClientID(),
		RunID:            info.RunID,
		Hostname:         info.Hostname,
		ClientIP:         info.IP,
		FirstConnectedAt: toUnix(info.FirstConnectedAt),
		LastConnectedAt:  toUnix(info.LastConnectedAt),
		Online:           info.Online,
	}
	if !info.DisconnectedAt.IsZero() {
		resp.DisconnectedAt = info.DisconnectedAt.Unix()
	}
	return resp
}

type proxyClientInfo struct {
	user          *string
	clientID      *string
	clientVersion *string
	runID         *string
}

func (c *Controller) fillProxyClientInfo(proxyInfo *proxyClientInfo, pxy proxy.Proxy) {
	loginMsg := pxy.GetLoginMsg()
	if loginMsg == nil {
		return
	}
	if proxyInfo.user != nil {
		*proxyInfo.user = loginMsg.User
	}
	if proxyInfo.clientVersion != nil {
		*proxyInfo.clientVersion = loginMsg.Version
	}
	if proxyInfo.runID != nil {
		*proxyInfo.runID = loginMsg.RunID
	}
	if c.clientRegistry != nil {
		if info, ok := c.clientRegistry.GetByRunID(loginMsg.RunID); ok {
			if proxyInfo.clientID != nil {
				*proxyInfo.clientID = info.ClientID()
			}
			return
		}
	}
	if proxyInfo.clientID != nil {
		*proxyInfo.clientID = loginMsg.RunID
	}
}

func (c *Controller) listAllProxyNames() []string {
	proxyTypes := []string{"tcp", "udp", "http", "https", "stcp", "sudp", "xtcp", "tcpmux"}
	allProxies := make([]string, 0)
	for _, proxyType := range proxyTypes {
		proxiesOfType := mem.StatsCollector.GetProxiesByType(proxyType)
		for _, proxyInfo := range proxiesOfType {
			allProxies = append(allProxies, proxyInfo.Name)
		}
	}
	return allProxies
}

func toUnix(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}

func matchStatusFilter(online bool, filter string) bool {
	switch strings.ToLower(filter) {
	case "", "all":
		return true
	case "online":
		return online
	case "offline":
		return !online
	default:
		return true
	}
}

func parseOptionalInt64Query(ctx *httppkg.Context, key string) (int64, error) {
	raw := ctx.Query(key)
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < 0 {
		return 0, httppkg.NewError(http.StatusBadRequest, fmt.Sprintf("invalid %s", key))
	}
	return value, nil
}

func parseOptionalIntQuery(ctx *httppkg.Context, key string) (int, error) {
	raw := ctx.Query(key)
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return 0, httppkg.NewError(http.StatusBadRequest, fmt.Sprintf("invalid %s", key))
	}
	return value, nil
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
	case v1.ProxyTypeSUDP:
		return &SUDPOutConf{}
	case v1.ProxyTypeXTCP:
		return &XTCPOutConf{}
	default:
		return nil
	}
}
