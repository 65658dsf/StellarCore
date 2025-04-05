package sub

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/AlecAivazis/survey/v2"
)

type InteractiveConfig struct {
	Token   string
	Tunnels []string
}

func runInteractiveMode() (*InteractiveConfig, error) {
	config := &InteractiveConfig{}

	// 欢迎信息
	fmt.Println("欢迎使用 StellarFrpc 交互式配置")
	fmt.Println("=============================")

	// Token输入
	prompt := &survey.Password{
		Message: "请输入您的Token:",
	}
	if err := survey.AskOne(prompt, &config.Token); err != nil {
		return nil, err
	}
	token = config.Token // 将用户输入的token赋值给全局变量

	// 获取可用隧道
	fmt.Println("正在获取隧道信息...")

	// 获取所有可用隧道列表
	allTunnelData, err := getAllTunnels()
	if err != nil {
		return nil, err
	}

	tunnelOptions := []string{}
	tunnelDisplayNames := make(map[string]string)

	// 显示隧道列表
	fmt.Println("\n可用的隧道列表:")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "隧道ID\t隧道名称\t节点名称\t本地端口\t远程端口\t状态\t创建时间")
	fmt.Fprintln(w, "--------\t--------\t--------\t--------\t--------\t----\t--------")

	// 构建隧道选项和ID到名称的映射
	for id, tunnelInfo := range allTunnelData {
		// 使用ProxyName作为隧道名称
		proxyName := tunnelInfo["ProxyName"].(string)
		// 构建选项显示格式：ID (名称)
		displayName := fmt.Sprintf("%s (%s)", id, proxyName)
		tunnelOptions = append(tunnelOptions, displayName)
		tunnelDisplayNames[displayName] = id

		// 解析时间戳
		timestamp := tunnelInfo["Timestamp"].(string)
		expireTime, _ := time.Parse("2006-01-02 15:04:05", timestamp)

		fmt.Fprintf(w, "%s\t%s\t%s\t%v\t%v\t%s\t%s\n",
			id,
			proxyName,
			tunnelInfo["NodeName"].(string),
			tunnelInfo["LocalPort"].(float64),
			tunnelInfo["RemotePort"].(float64),
			tunnelInfo["Status"].(string),
			expireTime.Format("2006-01-02"),
		)
	}
	w.Flush()
	fmt.Println()

	// 隧道选择
	var selectedOptions []string
	tunnelPrompt := &survey.MultiSelect{
		Message: "请选择要启动的隧道 (使用空格键选择，回车确认):",
		Options: tunnelOptions,
	}
	if err := survey.AskOne(tunnelPrompt, &selectedOptions); err != nil {
		return nil, err
	}

	// 将选择的显示名称转换回隧道ID
	for _, option := range selectedOptions {
		if id, ok := tunnelDisplayNames[option]; ok {
			config.Tunnels = append(config.Tunnels, id)
		}
	}

	if len(config.Tunnels) == 0 {
		return nil, fmt.Errorf("未选择任何隧道")
	}

	// 显示确认信息
	tunnelDisplayList := make([]string, 0, len(config.Tunnels))
	for _, id := range config.Tunnels {
		for displayName, tunnelID := range tunnelDisplayNames {
			if tunnelID == id {
				tunnelDisplayList = append(tunnelDisplayList, displayName)
				break
			}
		}
	}
	fmt.Printf("\n已选择以下隧道:\n%s\n", strings.Join(tunnelDisplayList, "\n"))

	return config, nil
}

func shouldRunInteractiveMode() bool {
	// 检查是否是通过双击运行的（Windows环境下）
	// 只有在没有任何命令行参数时才进入交互模式
	if len(os.Args) <= 1 {
		return true
	}
	return false
}
