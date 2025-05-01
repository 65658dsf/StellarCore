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

package util

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	mathrand "math/rand"
	"net"
	"strconv"
	"strings"
	"time"
)

// RandID return a rand string used in frp.
func RandID() (id string, err error) {
	return RandIDWithLen(16)
}

// RandIDWithLen return a rand string with idLen length.
func RandIDWithLen(idLen int) (id string, err error) {
	if idLen <= 0 {
		return "", nil
	}
	b := make([]byte, idLen/2+1)
	_, err = rand.Read(b)
	if err != nil {
		return
	}

	id = fmt.Sprintf("%x", b)
	return id[:idLen], nil
}

func GetAuthKey(token string, timestamp int64) (key string) {
	md5Ctx := md5.New()
	md5Ctx.Write([]byte(token))
	md5Ctx.Write([]byte(strconv.FormatInt(timestamp, 10)))
	data := md5Ctx.Sum(nil)
	return hex.EncodeToString(data)
}

func CanonicalAddr(host string, port int) (addr string) {
	if port == 80 || port == 443 {
		addr = host
	} else {
		addr = net.JoinHostPort(host, strconv.Itoa(port))
	}
	return
}

func ParseRangeNumbers(rangeStr string) (numbers []int64, err error) {
	rangeStr = strings.TrimSpace(rangeStr)
	numbers = make([]int64, 0)
	// e.g. 1000-2000,2001,2002,3000-4000
	numRanges := strings.Split(rangeStr, ",")
	for _, numRangeStr := range numRanges {
		// 1000-2000 or 2001
		numArray := strings.Split(numRangeStr, "-")
		// length: only 1 or 2 is correct
		rangeType := len(numArray)
		switch rangeType {
		case 1:
			// single number
			singleNum, errRet := strconv.ParseInt(strings.TrimSpace(numArray[0]), 10, 64)
			if errRet != nil {
				err = fmt.Errorf("range number is invalid, %v", errRet)
				return
			}
			numbers = append(numbers, singleNum)
		case 2:
			// range numbers
			minValue, errRet := strconv.ParseInt(strings.TrimSpace(numArray[0]), 10, 64)
			if errRet != nil {
				err = fmt.Errorf("range number is invalid, %v", errRet)
				return
			}
			maxValue, errRet := strconv.ParseInt(strings.TrimSpace(numArray[1]), 10, 64)
			if errRet != nil {
				err = fmt.Errorf("range number is invalid, %v", errRet)
				return
			}
			if maxValue < minValue {
				err = fmt.Errorf("range number is invalid")
				return
			}
			for i := minValue; i <= maxValue; i++ {
				numbers = append(numbers, i)
			}
		default:
			err = fmt.Errorf("range number is invalid")
			return
		}
	}
	return
}

func GenerateResponseErrorString(summary string, err error, detailed bool) string {
	if detailed {
		// 将英文错误消息转换为中文
		errMsg := err.Error()

		// 常见错误消息映射
		errorMap := map[string]string{
			"proxy already exists":            "隧道已存在",
			"port already used":               "端口已被占用",
			"port not allowed":                "端口不允许使用",
			"invalid auth":                    "验证失败",
			"invalid timestamp":               "时间戳无效",
			"privilege key":                   "特权密钥无效",
			"password incorrect":              "密码不正确",
			"exceed the max_ports_per_client": "超过客户端最大端口数限制",
			"no auth plugin configured":       "未配置验证插件",
			"token":                           "令牌无效",
			"connection":                      "连接错误",
			"timeout":                         "连接超时",
			"port unavailable":                "端口不可用",
			"proxy name conflict":             "隧道名称冲突",
			"authorization":                   "授权失败",
			"tls":                             "TLS连接错误",
			"error port name format":          "端口名称格式错误",
		}

		// 遍历错误映射进行替换
		for eng, chn := range errorMap {
			if strings.Contains(strings.ToLower(errMsg), strings.ToLower(eng)) {
				errMsg = strings.Replace(errMsg, eng, chn, -1)
			}
		}

		return errMsg
	}

	// 简单错误信息映射
	summaryMap := map[string]string{
		"register control error":      "注册控制连接错误",
		"register visitor conn error": "注册访问者连接错误",
		"invalid NewWorkConn":         "无效的工作连接",
		"new proxy":                   "创建隧道错误",
		"parse conf error":            "解析配置错误",
		"invalid ping":                "无效的心跳包",
	}

	// 遍历简单错误映射进行替换
	for eng, chn := range summaryMap {
		if strings.Contains(summary, eng) {
			return strings.Replace(summary, eng, chn, -1)
		}
	}

	return summary
}

func RandomSleep(duration time.Duration, minRatio, maxRatio float64) time.Duration {
	minValue := int64(minRatio * 1000.0)
	maxValue := int64(maxRatio * 1000.0)
	var n int64
	if maxValue <= minValue {
		n = minValue
	} else {
		n = minValue + mathrand.Int63n(maxValue-minValue)
	}
	d := duration * time.Duration(n) / time.Duration(1000)
	time.Sleep(d)
	return d
}

func ConstantTimeEqString(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
