package sub

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/fatedier/frp/pkg/util/log"
)

const api = "https://api.stellarfrp.top"

type Response struct {
	Code       int64             `json:"code"`
	Msg        string            `json:"msg"`
	Pagination Pagination        `json:"pagination"`
	Tunnel     map[string]Tunnel `json:"tunnel"`
}

type Pagination struct {
	Page     int64 `json:"page"`
	PageSize int64 `json:"page_size"`
	Pages    int64 `json:"pages"`
	Total    int64 `json:"total"`
}

type Tunnel struct {
	Data       string `json:"data"`
	Domains    string `json:"Domains"`
	ID         int64  `json:"Id"`
	Link       string `json:"Link"`
	LocalIP    string `json:"LocalIp"`
	LocalPort  int64  `json:"LocalPort"`
	NodeID     int64  `json:"NodeId"`
	NodeName   string `json:"NodeName"`
	ProxyName  string `json:"ProxyName"`
	ProxyType  string `json:"ProxyType"`
	RemotePort int64  `json:"RemotePort"`
	Status     string `json:"Status"`
	Timestamp  string `json:"Timestamp"`
	Type       string `json:"Type"`
}

// 新增函数：获取用户所有的隧道列表
func getAllTunnels(token string) (map[string]Tunnel, error) {
	// 创建请求
	req, err := http.NewRequest("GET", api+"/api/v1/proxy/get", nil)
	if err != nil {
		log.Errorf("创建请求失败: %v", err)
		return nil, fmt.Errorf("获取隧道列表失败: 创建请求错误")
	}

	// 设置请求头，传递token
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Errorf("发送请求失败: %v", err)
		return nil, fmt.Errorf("获取隧道列表失败: 发送请求错误")
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Errorf("获取隧道列表失败: %s", resp.Status)
		return nil, fmt.Errorf("获取隧道列表失败: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("读取响应失败: %v", err)
		return nil, fmt.Errorf("获取隧道列表失败: 读取响应错误")
	}

	var response Response
	if err := json.Unmarshal(body, &response); err != nil {
		log.Errorf("解析响应失败: %v", err)
		return nil, fmt.Errorf("获取隧道列表失败: 解析响应错误")
	}

	if response.Code != 200 {
		log.Errorf("获取隧道列表失败: %s", response.Msg)
		return nil, fmt.Errorf("获取隧道列表失败: %s", response.Msg)
	}

	return response.Tunnel, nil
}