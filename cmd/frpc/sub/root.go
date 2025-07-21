// Copyright 2018 fatedier, fatedier@gmail.com
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
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/65658dsf/StellarCore/client"
	"github.com/65658dsf/StellarCore/pkg/config"
	v1 "github.com/65658dsf/StellarCore/pkg/config/v1"
	"github.com/65658dsf/StellarCore/pkg/config/v1/validation"
	"github.com/65658dsf/StellarCore/pkg/util/log"
	"github.com/65658dsf/StellarCore/pkg/util/version"
)

var (
	cfgFile          string
	cfgDir           string
	showVersion      bool
	strictConfigMode bool
	token            string
	tunnelId         []string
	tunnelData       map[string]Tunnel
)

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "./frpc.ini", "需要被启动的隧道的配置文件。")
	rootCmd.PersistentFlags().StringVarP(&cfgDir, "config_dir", "", "", "需要被启动的隧道的配置文件目录。")
	rootCmd.PersistentFlags().BoolVarP(&showVersion, "version", "v", false, "输出版本号。")
	rootCmd.PersistentFlags().BoolVarP(&strictConfigMode, "strict_config", "", true, "严格配置解析模式，未知字段将导致错误。")
	rootCmd.PersistentFlags().StringVarP(&token, "token", "u", "", "从StellarConsole获取的Token。")
	rootCmd.PersistentFlags().StringSliceVarP(&tunnelId, "tunnel", "t", []string{}, "需要被启动的隧道id，多个隧道以英文逗号分隔。")
}

var rootCmd = &cobra.Command{
	Use:   "StellarFrpc",
	Short: "frpc is the client of frp (https://github.com/65658dsf/StellarCore)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if showVersion {
			fmt.Println(version.Full())
			return nil
		}

		// 如果 cfgDir 非空，则为 cfgDir 目录下的每个配置文件启动一个 frpc 服务。
		// 注意：此功能仅用于测试，不保证稳定性。
		if token != "" && len(tunnelId) != 0 {
			var err error
			tunnelData, err = getAllTunnels(token)
			if err != nil {
				fmt.Printf("错误: %v\n", err)
				// 等待用户按任意键退出
				fmt.Println("\n按任意键退出...")
				fmt.Scanln()
				return err
			}
			runTunnels := make(map[string]string)
			for _, id := range tunnelId {
				runTunnels[id] = tunnelData[id].Data
			}
			runMultipleClientsWithContent(runTunnels)
		} else {
			if cfgDir != "" {
				_ = runMultipleClients(cfgDir)
				return nil
			}

			// 此处不显示命令用法。
			err := runClient(cfgFile)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		}
		return nil
	},
}

func runMultipleClients(cfgDir string) error {
	var wg sync.WaitGroup
	err := filepath.WalkDir(cfgDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		wg.Add(1)
		time.Sleep(time.Millisecond)
		go func() {
			defer wg.Done()
			err := runClient(path)
			if err != nil {
				log.Errorf("frpc 服务运行失败，配置文件：%s\n", path)
			}
		}()
		return nil
	})
	wg.Wait()
	return err
}

func runMultipleClientsWithContent(Contents map[string]string) {
	var wg sync.WaitGroup
	for id, content := range Contents {
		wg.Add(1)
		time.Sleep(time.Millisecond)
		go func() {
			defer wg.Done()
			err := runClientFromConfigContent(content)
			if err != nil {
				log.Errorf("frpc 服务运行失败，隧道id：%s\n", id)
			}
		}()
	}
	wg.Wait()
}

func Execute() error {
	// 目测没用，先测试一下
	if shouldRunInteractiveMode() {
		err := runInteractiveMode()
		if err != nil {
			fmt.Printf("错误: %v\n", err)
			// 等待用户按任意键退出
			fmt.Println("\n按任意键退出...")
			fmt.Scanln()
			return err
		}
	}
	rootCmd.SetGlobalNormalizationFunc(config.WordSepNormalizeFunc)
	if err := rootCmd.Execute(); err != nil {
		return err
	}
	return nil
}

// 捕获退出信号
func handleTermSignal(svr *client.Service) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	svr.GracefulClose(500 * time.Millisecond)
}

func runClient(cfgFilePath string) error {
	cfg, proxyCfgs, visitorCfgs, isLegacyFormat, err := config.LoadClientConfig(cfgFilePath, strictConfigMode)
	if err != nil {
		return err
	}
	if isLegacyFormat {
		fmt.Printf("警告：ini 格式已被弃用，未来将不再支持，请使用 yaml/json/toml 格式替代！\n")
	}

	warning, err := validation.ValidateAllClientConfig(cfg, proxyCfgs, visitorCfgs)
	if warning != nil {
		fmt.Printf("警告：%v\n", warning)
	}
	if err != nil {
		return err
	}
	return startService(cfg, proxyCfgs, visitorCfgs, cfgFilePath)
}

func runClientFromConfigContent(content string) error {
	cfg, proxyCfgs, visitorCfgs, err := config.LoadClientConfigFromContent(content, strictConfigMode)
	if err != nil {
		return err
	}

	warning, err := validation.ValidateAllClientConfig(cfg, proxyCfgs, visitorCfgs)
	if warning != nil {
		fmt.Printf("警告：%v\n", warning)
	}
	if err != nil {
		return err
	}
	return startService(cfg, proxyCfgs, visitorCfgs, "")
}

func startService(
	cfg *v1.ClientCommonConfig,
	proxyCfgs []v1.ProxyConfigurer,
	visitorCfgs []v1.VisitorConfigurer,
	cfgFile string,
) error {
	log.InitLogger(cfg.Log.To, cfg.Log.Level, int(cfg.Log.MaxDays), cfg.Log.DisablePrintColor)

	if cfgFile != "" {
		log.Infof("启动 frpc 服务，配置文件：%s", cfgFile)
		defer log.Infof("frpc 服务已停止，配置文件：%s", cfgFile)
	}
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
	// 如果使用 kcp 或 quic，则捕获退出信号。
	if shouldGracefulClose {
		go handleTermSignal(svr)
	}
	return svr.Run(context.Background())
}
