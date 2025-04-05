<template>
  <div>
    <n-card title="使用帮助" class="help-card">
      <n-space vertical>
        <n-collapse>
          <n-collapse-item title="什么是StellarFrp?" name="what-is">
            <n-p>
              StellarFrp是一个基于frp开发的内网穿透工具，它可以帮助您轻松地将内网服务暴露到公网，实现远程访问和管理。
              本工具基于开源项目frp构建，提供了更友好的用户界面和更简便的配置方式。
            </n-p>
          </n-collapse-item>
          
          <n-collapse-item title="如何配置隧道?" name="how-to-config">
            <n-p>
              您可以在"配置"页面中编辑frpc的配置文件。一个基本的配置示例如下：
            </n-p>
            <n-code language="ini">
[common]
server_addr = frp服务器地址
server_port = 7000
token = 认证令牌

[web]
type = tcp
local_ip = 127.0.0.1
local_port = 80
remote_port = 8080
            </n-code>
            <n-p>
              编辑完成后，点击"上传配置"按钮保存配置。配置更新后，您可能需要重启frpc客户端以应用新的配置。
            </n-p>
          </n-collapse-item>
          
          <n-collapse-item title="常见问题解答" name="faq">
            <n-h3>1. 连接服务器失败怎么办？</n-h3>
            <n-p>
              请检查以下几点：
            </n-p>
            <n-ul>
              <n-li>确认服务器地址和端口是否正确</n-li>
              <n-li>检查认证令牌是否匹配</n-li>
              <n-li>确认网络连接是否正常，服务器是否可以访问</n-li>
              <n-li>查看防火墙设置，确保相关端口已开放</n-li>
            </n-ul>
            
            <n-h3>2. 隧道创建成功但无法访问怎么办？</n-h3>
            <n-p>
              请检查以下几点：
            </n-p>
            <n-ul>
              <n-li>确认本地服务是否正常运行</n-li>
              <n-li>检查本地IP和端口配置是否正确</n-li>
              <n-li>查看隧道状态是否为"运行中"</n-li>
              <n-li>确认远程端口是否被其他服务占用</n-li>
            </n-ul>
          </n-collapse-item>
          
          <n-collapse-item title="联系我们" name="contact">
            <n-p>
              如果您在使用过程中遇到任何问题，或有任何建议和反馈，欢迎联系我们：
            </n-p>
            <n-ul>
              <n-li>官方网站：<n-a href="https://github.com/fatedier/frp" target="_blank">https://github.com/fatedier/frp</n-a></n-li>
              <n-li>问题反馈：请在GitHub上提交issue</n-li>
            </n-ul>
          </n-collapse-item>
        </n-collapse>
      </n-space>
    </n-card>
  </div>
</template>

<script setup lang="ts">
import { NCard, NSpace, NCollapse, NCollapseItem, NP, NCode, NH3, NUl, NLi, NA } from 'naive-ui'
</script>

<style scoped>
.help-card {
  margin-bottom: 16px;
}
</style> 