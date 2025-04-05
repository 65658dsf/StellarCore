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

package vhost

import (
	"bytes"
	"io"
	"net/http"
	"os"

	"github.com/fatedier/frp/pkg/util/log"
	"github.com/fatedier/frp/pkg/util/version"
)

var NotFoundPagePath = ""

const (
	NotFound = `<!DOCTYPE html>
<html>
<head>
<title>页面未找到 - StellarFrp</title>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<style>
    body {
        font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
        line-height: 1.6;
        color: #333;
        max-width: 650px;
        margin: 0 auto;
        padding: 2rem 1rem;
        background-color: #f9f9f9;
    }
    .container {
        background-color: #fff;
        border-radius: 8px;
        box-shadow: 0 2px 10px rgba(0, 0, 0, 0.1);
        padding: 2rem;
        text-align: center;
    }
    h1 {
        color: #2080f0;
        margin-top: 0;
        font-size: 2rem;
    }
    .icon {
        font-size: 6rem;
        color: #2080f0;
        margin: 1rem 0;
    }
    p {
        margin: 1rem 0;
        color: #666;
    }
    a {
        color: #2080f0;
        text-decoration: none;
        transition: color 0.2s;
    }
    a:hover {
        color: #40a9ff;
        text-decoration: underline;
    }
    .footer {
        margin-top: 2rem;
        font-size: 0.9rem;
        color: #999;
    }
    .btn {
        display: inline-block;
        margin-top: 1rem;
        padding: 0.6rem 1.5rem;
        background-color: #2080f0;
        color: white;
        border-radius: 4px;
        text-decoration: none;
        transition: background-color 0.2s;
    }
    .btn:hover {
        background-color: #40a9ff;
        text-decoration: none;
        color: white;
    }
    @media (max-width: 480px) {
        h1 {
            font-size: 1.6rem;
        }
        .icon {
            font-size: 4rem;
        }
    }
</style>
</head>
<body>
<div class="container">
    <div class="icon">404</div>
    <h1>很抱歉，您请求的页面未找到</h1>
    <p>您访问的页面可能已被移除、名称已更改或暂时不可用。</p>
    <p>请稍后再试，或返回首页。</p>
    <a href="https://www.stellarfrp.top" class="btn">返回首页</a>
    <div class="footer">
        <p>此服务由 <a href="https://www.stellarfrp.top" target="_blank">StellarFrp</a> 提供技术支持</p>
        <p><em>竭诚为您服务，StellarFrp团队</em></p>
    </div>
</div>
</body>
</html>
`
)

func getNotFoundPageContent() []byte {
	var (
		buf []byte
		err error
	)
	if NotFoundPagePath != "" {
		buf, err = os.ReadFile(NotFoundPagePath)
		if err != nil {
			log.Warnf("read custom 404 page error: %v", err)
			buf = []byte(NotFound)
		}
	} else {
		buf = []byte(NotFound)
	}
	return buf
}

func NotFoundResponse() *http.Response {
	header := make(http.Header)
	header.Set("server", version.Brand()+"/"+version.Full())
	header.Set("Content-Type", "text/html; charset=utf-8")

	content := getNotFoundPageContent()
	res := &http.Response{
		Status:        "Not Found",
		StatusCode:    404,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        header,
		Body:          io.NopCloser(bytes.NewReader(content)),
		ContentLength: int64(len(content)),
	}
	return res
}
