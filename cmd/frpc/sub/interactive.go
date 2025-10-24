package sub

import (
	"fmt"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
)

func runInteractiveMode() (error) {
	// 欢迎信息
	fmt.Println("欢迎使用 StellarFrpc 交互式配置")
	fmt.Println("=============================")

	// Token输入
	prompt := &survey.Password{
		Message: "请输入您的Token:",
	}
	if err := survey.AskOne(prompt, &token); err != nil {
		return err
	}

	// 获取可用隧道
	fmt.Println("正在获取隧道信息...")

	// 获取所有可用隧道列表
	allTunnelData, err := getAllTunnels(token)
	if err != nil {
		return err
	}

	// 构建隧道展示表格
	fmt.Printf("%-8s %-12s %-6s %-8s %-8s %-20s %s\n", 
		"隧道ID", "隧道名称", "类型", "远程端口", "状态", "节点名称", "创建时间")
	fmt.Println(strings.Repeat("-", 80))
	tunnelNames := make([]string, 0, len(allTunnelData))
	for _, tunnel := range allTunnelData {
		fmt.Printf("%-8d %-12s %-6s %-8d %-8s %-20s %s\n",
			tunnel.ID,
			tunnel.ProxyName,
			tunnel.Type,
			tunnel.RemotePort,
			tunnel.Status,
			tunnel.NodeName,
			tunnel.Timestamp,
		)
		tunnelNames = append(tunnelNames, fmt.Sprintf("%d %s", tunnel.ID, tunnel.ProxyName))
	}

	// 让用户选择要启用的隧道
	var selectedTunnels []string
	selectPrompt := &survey.MultiSelect{
		Message: "请选择要启用的隧道（可多选，空格选择，回车确认）：",
		Options: tunnelNames,
	}
	if err := survey.AskOne(selectPrompt, &selectedTunnels); err != nil {
		return err
	}
	runTunnels := make(map[string]string)
	for _, displayName := range selectedTunnels {
		ls := strings.Split(displayName, " ")
		tunnelID := ls[0]
		// 根据隧道ID找到对应的隧道数据，使用data字段作为配置内容
		for id, tunnel := range allTunnelData {
			if id == tunnelID {
				runTunnels[tunnelID] = tunnel.Data
				break
			}
		}
	}
	runMultipleClientsWithContent(runTunnels)
	return nil
}

func shouldRunInteractiveMode() bool {
	// 检查是否是通过双击运行的（Windows环境下）
	// 只有在没有任何命令行参数时才进入交互模式
	if len(os.Args) <= 1 {
		return true
	}
	return false
}
