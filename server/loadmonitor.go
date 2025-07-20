// Copyright 2024-2025 StellarCore, ningmeng@stellarfrp.top
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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/65658dsf/StellarCore/pkg/metrics/mem"
	"github.com/65658dsf/StellarCore/pkg/util/log"
	"github.com/shirou/gopsutil/v3/cpu"
	gopsutilmem "github.com/shirou/gopsutil/v3/mem"
)

const (
	// 负载计算权重
	weightConnRatio     = 0.25 // α：连接数相对负载
	weightTrafficRatio  = 0.25 // β：流量相对负载
	weightCPU           = 0.20 // γ：CPU使用率
	weightMem           = 0.10 // δ：内存使用率
	weightConnGrowth    = 0.10 // ε：连接数增长速率
	weightTrafficGrowth = 0.10 // ζ：流量增长速率

	// 历史数据窗口大小
	historyWindow  = 24 * time.Hour   // 24小时窗口
	growthWindow   = 5 * time.Minute  // 5分钟窗口
	reportInterval = 30 * time.Second // 30秒上报一次
	webhookURL     = "https://api.stellarfrp.top/api/v1/nodes/load/webhook"
)

// LoadMonitor 负载监控器
type LoadMonitor struct {
	// 当前状态
	currentConns   int64
	currentTraffic int64

	// 历史峰值
	peakConns   int64
	peakTraffic int64

	// 历史数据
	connHistory    map[time.Time]int64
	trafficHistory map[time.Time]int64

	// 公网IP
	publicIP string

	// 互斥锁
	mu sync.RWMutex

	// 上下文和取消函数
	ctx    context.Context
	cancel context.CancelFunc
}

// LoadData 负载上报数据结构
type LoadData struct {
	IP                string  `json:"ip"`
	LoadScore         float64 `json:"load_score"`
	CurrentConns      int64   `json:"current_conns"`
	PeakConns         int64   `json:"peak_conns"`
	CurrentTraffic    int64   `json:"current_traffic"`
	PeakTraffic       int64   `json:"peak_traffic"`
	CPUUsage          float64 `json:"cpu_usage"`
	MemUsage          float64 `json:"mem_usage"`
	ConnGrowthRate    float64 `json:"conn_growth_rate"`
	TrafficGrowthRate float64 `json:"traffic_growth_rate"`
	Timestamp         int64   `json:"timestamp"`
}

// NewLoadMonitor 创建新的负载监控器
func NewLoadMonitor() *LoadMonitor {
	ctx, cancel := context.WithCancel(context.Background())
	return &LoadMonitor{
		connHistory:    make(map[time.Time]int64),
		trafficHistory: make(map[time.Time]int64),
		ctx:            ctx,
		cancel:         cancel,
	}
}

// Start 启动负载监控
func (lm *LoadMonitor) Start() {
	log.Infof("负载监控服务启动")

	// 获取公网IP
	go lm.updatePublicIP()

	// 定期清理过期历史数据
	go lm.cleanHistoryData()

	// 定期上报负载
	go lm.reportLoadPeriodically()
}

// Stop 停止负载监控
func (lm *LoadMonitor) Stop() {
	lm.cancel()
}

// updatePublicIP 更新公网IP
func (lm *LoadMonitor) updatePublicIP() {
	for {
		ip, err := getPublicIP()
		if err != nil {
			log.Errorf("获取公网IP失败: %v", err)
		} else {
			lm.mu.Lock()
			lm.publicIP = ip
			lm.mu.Unlock()
			log.Infof("获取公网IP成功: %s", ip)
		}

		// 每小时更新一次IP
		select {
		case <-lm.ctx.Done():
			return
		case <-time.After(1 * time.Hour):
		}
	}
}

// cleanHistoryData 清理过期的历史数据
func (lm *LoadMonitor) cleanHistoryData() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-lm.ctx.Done():
			return
		case <-ticker.C:
			lm.mu.Lock()
			now := time.Now()

			// 清理连接数历史
			for t := range lm.connHistory {
				if now.Sub(t) > historyWindow {
					delete(lm.connHistory, t)
				}
			}

			// 清理流量历史
			for t := range lm.trafficHistory {
				if now.Sub(t) > historyWindow {
					delete(lm.trafficHistory, t)
				}
			}
			lm.mu.Unlock()
		}
	}
}

// reportLoadPeriodically 定期上报负载
func (lm *LoadMonitor) reportLoadPeriodically() {
	ticker := time.NewTicker(reportInterval)
	defer ticker.Stop()

	for {
		select {
		case <-lm.ctx.Done():
			return
		case <-ticker.C:
			loadData, err := lm.calculateLoad()
			if err != nil {
				log.Errorf("计算负载失败: %v", err)
				continue
			}

			if err := lm.reportLoad(loadData); err != nil {
				log.Errorf("上报负载失败: %v", err)
			} else {
				log.Debugf("上报负载成功: 分数=%.2f, 连接数=%d, 流量=%d",
					loadData.LoadScore, loadData.CurrentConns, loadData.CurrentTraffic)
			}
		}
	}
}

// calculateLoad 计算当前负载
func (lm *LoadMonitor) calculateLoad() (*LoadData, error) {
	// 获取当前连接数和流量
	stats := mem.StatsCollector.GetServer()
	currentConns := stats.CurConns
	currentTraffic := stats.TotalTrafficIn + stats.TotalTrafficOut

	// 更新历史数据
	now := time.Now()
	lm.mu.Lock()

	// 记录当前数据点
	lm.connHistory[now] = currentConns
	lm.trafficHistory[now] = currentTraffic

	// 更新峰值
	if currentConns > lm.peakConns {
		lm.peakConns = currentConns
	}
	if currentTraffic > lm.peakTraffic {
		lm.peakTraffic = currentTraffic
	}

	// 计算增长率
	var connGrowthRate, trafficGrowthRate float64
	fiveMinutesAgo := now.Add(-growthWindow)

	// 找到最接近5分钟前的数据点
	var prevConns, prevTraffic int64
	minConnDiff, minTrafficDiff := growthWindow, growthWindow

	for t, conn := range lm.connHistory {
		diff := fiveMinutesAgo.Sub(t)
		if diff >= 0 && diff < minConnDiff {
			minConnDiff = diff
			prevConns = conn
		}
	}

	for t, traffic := range lm.trafficHistory {
		diff := fiveMinutesAgo.Sub(t)
		if diff >= 0 && diff < minTrafficDiff {
			minTrafficDiff = diff
			prevTraffic = traffic
		}
	}

	// 如果有历史数据，计算增长率
	if prevConns > 0 {
		connGrowthRate = float64(currentConns-prevConns) / float64(prevConns)
	}
	if prevTraffic > 0 {
		trafficGrowthRate = float64(currentTraffic-prevTraffic) / float64(prevTraffic)
	}

	// 保存当前值以便下次计算
	lm.currentConns = currentConns
	lm.currentTraffic = currentTraffic

	// 获取公网IP
	publicIP := lm.publicIP
	lm.mu.Unlock()

	// 获取CPU和内存使用率
	cpuUsage, err := getCPUUsage()
	if err != nil {
		return nil, fmt.Errorf("获取CPU使用率失败: %v", err)
	}

	memUsage, err := getMemoryUsage()
	if err != nil {
		return nil, fmt.Errorf("获取内存使用率失败: %v", err)
	}

	// 计算相对负载
	var connRatio, trafficRatio float64
	if lm.peakConns > 0 {
		connRatio = float64(currentConns) / float64(lm.peakConns)
	}
	if lm.peakTraffic > 0 {
		trafficRatio = float64(currentTraffic) / float64(lm.peakTraffic)
	}

	// 计算最终负载分数
	loadScore := weightConnRatio*connRatio +
		weightTrafficRatio*trafficRatio +
		weightCPU*cpuUsage +
		weightMem*memUsage +
		weightConnGrowth*connGrowthRate +
		weightTrafficGrowth*trafficGrowthRate

	// 确保负载分数在0-1之间
	if loadScore < 0 {
		loadScore = 0
	} else if loadScore > 1 {
		loadScore = 1
	}

	return &LoadData{
		IP:                publicIP,
		LoadScore:         loadScore,
		CurrentConns:      currentConns,
		PeakConns:         lm.peakConns,
		CurrentTraffic:    currentTraffic,
		PeakTraffic:       lm.peakTraffic,
		CPUUsage:          cpuUsage,
		MemUsage:          memUsage,
		ConnGrowthRate:    connGrowthRate,
		TrafficGrowthRate: trafficGrowthRate,
		Timestamp:         now.Unix(),
	}, nil
}

// reportLoad 上报负载数据
func (lm *LoadMonitor) reportLoad(data *LoadData) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("序列化负载数据失败: %v", err)
	}

	req, err := http.NewRequest("POST", webhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("创建HTTP请求失败: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("发送HTTP请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应内容
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应内容失败: %v", err)
	}

	// 检查HTTP状态码
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("服务器返回非200状态码: %d, 响应: %s", resp.StatusCode, string(body))
	}

	// 解析JSON响应
	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("解析响应JSON失败: %v", err)
	}

	// 检查业务状态码
	if result.Code != 200 {
		return fmt.Errorf("业务处理失败: code=%d, msg=%s", result.Code, result.Msg)
	}

	return nil
}

// getCPUUsage 获取CPU使用率
func getCPUUsage() (float64, error) {
	percentage, err := cpu.Percent(time.Second, false)
	if err != nil {
		return 0, err
	}

	if len(percentage) == 0 {
		return 0, fmt.Errorf("无法获取CPU使用率")
	}

	// 返回0-1之间的值
	return percentage[0] / 100.0, nil
}

// getMemoryUsage 获取内存使用率
func getMemoryUsage() (float64, error) {
	memInfo, err := gopsutilmem.VirtualMemory()
	if err != nil {
		return 0, err
	}

	// 返回0-1之间的值
	return memInfo.UsedPercent / 100.0, nil
}

// getPublicIP 获取公网IP
func getPublicIP() (string, error) {
	// 直接从外部服务获取
	resp, err := http.Get("https://api.ipify.cn")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	ip, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(ip), nil
}
