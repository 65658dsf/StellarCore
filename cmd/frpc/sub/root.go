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

package sub

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"bytes"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/fatedier/frp/client"
	"github.com/fatedier/frp/pkg/config"
	v1 "github.com/fatedier/frp/pkg/config/v1"
	"github.com/fatedier/frp/pkg/config/v1/validation"
	"github.com/fatedier/frp/pkg/util/log"
	"github.com/fatedier/frp/pkg/util/version"
)

var (
	cfgFile          string
	cfgDir           string
	showVersion      bool
	strictConfigMode bool
	token            string
	tunnels          []string
)

const api = "https://api.stellarfrp.top"

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "./frpc.ini", "需要被启动的隧道的配置文件。")
	rootCmd.PersistentFlags().StringVarP(&cfgDir, "config_dir", "", "", "需要被启动的隧道的配置文件目录。")
	rootCmd.PersistentFlags().BoolVarP(&showVersion, "version", "v", false, "输出版本号。")
	rootCmd.PersistentFlags().BoolVarP(&strictConfigMode, "strict_config", "", true, "严格配置解析模式，未知字段将导致错误。")
	rootCmd.PersistentFlags().StringVarP(&token, "token", "u", "", "从StellarConsole获取的Token。")
	rootCmd.PersistentFlags().StringSliceVarP(&tunnels, "tunnel", "t", []string{}, "需要被启动的隧道id，多个隧道以英文逗号分隔。")
}

var rootCmd = &cobra.Command{
	Use:   "StellarFrpc",
	Short: "frpc is the client of frp (https://github.com/65658dsf/StellarCore)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if showVersion {
			fmt.Println(version.Full())
			return nil
		}

		// 检查是否需要进入交互模式
		if shouldRunInteractiveMode() {
			config, err := runInteractiveMode()
			if err != nil {
				fmt.Printf("错误: %v\n", err)
				// 等待用户按任意键退出
				fmt.Println("\n按任意键退出...")
				fmt.Scanln()
				return err
			}
			// 设置从交互式配置获取的值
			token = config.Token
			tunnels = config.Tunnels
		}

		if token != "" && len(tunnels) != 0 {
			log.Infof("正在获取隧道配置...")

			// 存储所有隧道的数据
			allTunnelData := make(map[string]map[string]interface{})

			// 逐个获取每个隧道的数据
			for _, tunnel := range tunnels {
				// 临时保存当前要查询的隧道ID
				originalTunnels := tunnels
				tunnels = []string{tunnel}

				// 获取单个隧道数据
				data, err := getUserTunnels()

				// 恢复隧道ID列表
				tunnels = originalTunnels

				if err != nil {
					log.Warnf("获取隧道 [%s] 信息失败: %v", tunnel, err)
					continue
				}

				// 将获取到的隧道数据添加到总数据中
				for id, tunnelData := range data {
					allTunnelData[id] = tunnelData.(map[string]interface{})
				}
			}

			if len(allTunnelData) == 0 {
				return fmt.Errorf("错误: 没有获取到任何隧道数据")
			}

			var wg sync.WaitGroup
			for _, tunnel := range tunnels {
				// 查找隧道ID对应的数据
				if tunnelData, ok := allTunnelData[tunnel]; ok {
					// 使用data字段获取隧道配置内容
					content := tunnelData["data"].(string)
					wg.Add(1)
					time.Sleep(time.Millisecond)
					go func(tunnelID string) {
						defer wg.Done()
						err := runClient(content, true)
						if err != nil {
							fmt.Printf("隧道启动失败: [%s]\n", tunnelID)
						}
					}(tunnel)
				} else {
					log.Warnf("此隧道ID不存在: %s", tunnel)
				}
			}
			wg.Wait()
		} else {
			// If cfgDir is not empty, run multiple frpc service for each config file in cfgDir.
			// Note that it's only designed for testing. It's not guaranteed to be stable.
			if cfgDir != "" {
				_ = runMultipleClients(cfgDir, false)
				return nil
			}

			// Do not show command usage here.
			err := runClient(cfgFile, false)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		}
		return nil
	},
}

func runMultipleClients(cfgDir string, alreadyRead bool) error {
	var wg sync.WaitGroup
	err := filepath.WalkDir(cfgDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		wg.Add(1)
		time.Sleep(time.Millisecond)
		go func() {
			defer wg.Done()
			err := runClient(path, alreadyRead)
			if err != nil {
				fmt.Printf("隧道启动失败: [%s]\n", path)
			}
		}()
		return nil
	})
	wg.Wait()
	return err
}

func startInteractiveService(config *InteractiveConfig) error {
	// 设置基本配置
	token = config.Token
	tunnels = config.Tunnels

	// 获取隧道配置并启动服务
	log.Infof("正在获取隧道配置...")

	// 存储所有隧道的数据
	allTunnelData := make(map[string]map[string]interface{})

	// 逐个获取每个隧道的数据
	for _, tunnel := range tunnels {
		// 临时保存当前要查询的隧道ID
		originalTunnels := tunnels
		tunnels = []string{tunnel}

		// 获取单个隧道数据
		data, err := getUserTunnels()

		// 恢复隧道ID列表
		tunnels = originalTunnels

		if err != nil {
			log.Warnf("获取隧道 [%s] 信息失败: %v", tunnel, err)
			continue
		}

		// 将获取到的隧道数据添加到总数据中
		for id, tunnelData := range data {
			allTunnelData[id] = tunnelData.(map[string]interface{})
		}
	}

	if len(allTunnelData) == 0 {
		return fmt.Errorf("错误: 没有获取到任何隧道数据")
	}

	var wg sync.WaitGroup
	for _, tunnel := range tunnels {
		// 查找隧道ID对应的数据
		if tunnelData, ok := allTunnelData[tunnel]; ok {
			// 使用data字段获取隧道配置内容
			content := tunnelData["data"].(string)
			wg.Add(1)
			time.Sleep(time.Millisecond)
			go func(tunnelID string) {
				defer wg.Done()
				err := runClient(content, true)
				if err != nil {
					fmt.Printf("隧道启动失败: [%s]\n", tunnelID)
				}
			}(tunnel)
		} else {
			log.Warnf("此隧道ID不存在: %s", tunnel)
		}
	}
	wg.Wait()
	return nil
}

func Execute() error {
	// 检查是否需要进入交互模式
	if shouldRunInteractiveMode() {
		config, err := runInteractiveMode()
		if err != nil {
			fmt.Printf("错误: %v\n", err)
			// 等待用户按任意键退出
			fmt.Println("\n按任意键退出...")
			fmt.Scanln()
			return err
		}

		// 使用新的启动函数处理交互式模式
		return startInteractiveService(config)
	}

	// 继续执行原有的启动逻辑
	return rootCmd.Execute()
}

func handleTermSignal(svr *client.Service) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	svr.GracefulClose(500 * time.Millisecond)
}

func runClient(cfgFilePath string, alreadyRead bool) error {
	cfg, proxyCfgs, visitorCfgs, isLegacyFormat, err := config.LoadClientConfig(cfgFilePath, strictConfigMode)
	if err != nil {
		return err
	}
	if isLegacyFormat {
		fmt.Printf("ini 格式已不再推荐使用，将在以后的版本中移除支持，请改用 yaml/json/toml 格式！")
	}

	warning, err := validation.ValidateAllClientConfig(cfg, proxyCfgs, visitorCfgs)
	if warning != nil {
		fmt.Printf("警告: %v\n", warning)
	}
	if err != nil {
		return err
	}
	return startService(cfg, proxyCfgs, visitorCfgs, cfgFilePath)
}

func startService(
	cfg *v1.ClientCommonConfig,
	proxyCfgs []v1.ProxyConfigurer,
	visitorCfgs []v1.VisitorConfigurer,
	cfgFile string,
) error {
	log.InitLogger(cfg.Log.To, cfg.Log.Level, int(cfg.Log.MaxDays), cfg.Log.DisablePrintColor)

	svr, err := client.NewService(client.ServiceOptions{
		Common:         cfg,
		ProxyCfgs:      proxyCfgs,
		VisitorCfgs:    visitorCfgs,
		ConfigFilePath: cfgFile,
	})
	if err != nil {
		return err
	}

	shouldGracefulClose := cfg.Transport.Protocol == "kcp" || cfg.Transport.Protocol == "quic"
	// Capture the exit signal if we use kcp or quic.
	if shouldGracefulClose {
		go handleTermSignal(svr)
	}
	return svr.Run(context.Background())
}

func getUserTunnels() (map[string]interface{}, error) {
	// 检查是否有指定的隧道ID
	if len(tunnels) == 0 {
		return nil, fmt.Errorf("未指定隧道ID，请使用 -t 参数指定需要启动的隧道ID")
	}

	// 使用第一个指定的隧道ID
	tunnelID := tunnels[0]
	// 转换为数字ID
	var numericID int
	_, err := fmt.Sscanf(tunnelID, "%d", &numericID)
	if err != nil {
		log.Warnf("隧道ID转换为数字失败: %s", tunnelID)
		return nil, fmt.Errorf("隧道ID必须是数字")
	}

	// 创建请求体，使用用户指定的ID
	payload := map[string]interface{}{
		"id": numericID,
	}
	jsonValue, _ := json.Marshal(payload)
	// 创建请求
	req, err := http.NewRequest("GET", api+"/api/v1/proxy/get", bytes.NewBuffer(jsonValue))
	if err != nil {
		log.Errorf("创建请求失败: %v", err)
		return nil, fmt.Errorf("获取隧道信息失败: 创建请求错误")
	}

	// 设置请求头，通过Authorization传递token
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", token)

	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Errorf("请求失败: %v", err)
		return nil, fmt.Errorf("获取隧道信息失败: 网络连接错误，请检查网络连接")
	}
	defer resp.Body.Close()

	// 读取响应内容
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("读取响应失败: %v", err)
		return nil, fmt.Errorf("获取隧道信息失败: 读取响应数据错误")
	}

	if resp.StatusCode != 200 {
		log.Errorf("API响应错误: status=%d, body=%s", resp.StatusCode, string(respBody))
		return nil, fmt.Errorf("获取隧道信息失败: 服务器返回错误(状态码: %d)", resp.StatusCode)
	}

	// 解析新的响应格式
	var response struct {
		Code   int                    `json:"code"`
		Msg    string                 `json:"msg"`
		Tunnel map[string]interface{} `json:"tunnel"`
	}
	if err := json.Unmarshal(respBody, &response); err != nil {
		log.Errorf("解析JSON失败: %v, body=%s", err, string(respBody))
		return nil, fmt.Errorf("获取隧道信息失败: 解析响应数据错误")
	}

	// 检查响应状态码
	if response.Code != 200 {
		log.Errorf("API返回错误: code=%d, msg=%s", response.Code, response.Msg)
		if response.Code == 401 {
			return nil, fmt.Errorf("获取隧道信息失败: Token无效或未提供，请检查Token是否正确")
		}
		return nil, fmt.Errorf("获取隧道信息失败: %s", response.Msg)
	}

	// 直接返回tunnel字段下的数据
	if response.Tunnel == nil {
		log.Errorf("响应中没有隧道数据: %s", string(respBody))
		return nil, fmt.Errorf("获取隧道信息失败: 响应数据中没有隧道信息")
	}

	return response.Tunnel, nil
}

// 新增函数：获取用户所有的隧道列表
func getAllTunnels() (map[string]map[string]interface{}, error) {
	// 创建请求
	req, err := http.NewRequest("GET", api+"/api/v1/proxy/get", nil)
	if err != nil {
		log.Errorf("创建请求失败: %v", err)
		return nil, fmt.Errorf("获取隧道列表失败: 创建请求错误")
	}

	// 设置请求头，传递token
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", token)

	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Errorf("请求失败: %v", err)
		return nil, fmt.Errorf("获取隧道列表失败: 网络连接错误，请检查网络连接")
	}
	defer resp.Body.Close()

	// 读取响应内容
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("读取响应失败: %v", err)
		return nil, fmt.Errorf("获取隧道列表失败: 读取响应数据错误")
	}

	if resp.StatusCode != 200 {
		log.Errorf("API响应错误: status=%d, body=%s", resp.StatusCode, string(respBody))
		return nil, fmt.Errorf("获取隧道列表失败: 服务器返回错误(状态码: %d)", resp.StatusCode)
	}

	// 解析响应格式 - 修改为tunnel字段
	var response struct {
		Code   int                               `json:"code"`
		Msg    string                            `json:"msg"`
		Tunnel map[string]map[string]interface{} `json:"tunnel"`
	}

	if err := json.Unmarshal(respBody, &response); err != nil {
		log.Errorf("解析JSON失败: %v, body=%s", err, string(respBody))
		return nil, fmt.Errorf("获取隧道列表失败: 解析响应数据错误")
	}

	// 检查响应状态码
	if response.Code != 200 {
		log.Errorf("API返回错误: code=%d, msg=%s", response.Code, response.Msg)
		if response.Code == 401 {
			return nil, fmt.Errorf("获取隧道列表失败: Token无效或未提供，请检查Token是否正确")
		}
		return nil, fmt.Errorf("获取隧道列表失败: %s", response.Msg)
	}

	// 返回隧道列表
	if response.Tunnel == nil || len(response.Tunnel) == 0 {
		return nil, fmt.Errorf("获取隧道列表失败: 没有可用的隧道")
	}

	return response.Tunnel, nil
}

func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "StellarFrpc",
		Short: "frpc is the client of frp (https://github.com/65658dsf/StellarCore)",
		RunE:  rootCmd.RunE,
	}

	// 添加与 rootCmd 相同的标志
	cmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "./frpc.ini", "需要被启动的隧道的配置文件。")
	cmd.PersistentFlags().StringVarP(&cfgDir, "config_dir", "", "", "需要被启动的隧道的配置文件目录。")
	cmd.PersistentFlags().BoolVarP(&showVersion, "version", "v", false, "输出版本号。")
	cmd.PersistentFlags().BoolVarP(&strictConfigMode, "strict_config", "", true, "严格配置解析模式，未知字段将导致错误。")
	cmd.PersistentFlags().StringVarP(&token, "token", "u", "", "从StellarConsole获取的Token。")
	cmd.PersistentFlags().StringSliceVarP(&tunnels, "tunnel", "t", []string{}, "需要被启动的隧道id，多个隧道以英文逗号分隔。")

	return cmd
}
