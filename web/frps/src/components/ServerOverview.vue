<template>
  <div>
    <el-row>
      <el-col :md="12">
        <div class="source">
          <el-form
            label-position="left"
            label-width="220px"
            class="server_info"
          >
            <el-form-item label="版本">
              <span>{{ data.version }}</span>
            </el-form-item>
            <el-form-item label="绑定端口">
              <span>{{ data.bindPort }}</span>
            </el-form-item>
            <el-form-item label="KCP 绑定端口" v-if="data.kcpBindPort != 0">
              <span>{{ data.kcpBindPort }}</span>
            </el-form-item>
            <el-form-item label="QUIC 绑定端口" v-if="data.quicBindPort != 0">
              <span>{{ data.quicBindPort }}</span>
            </el-form-item>
            <el-form-item label="HTTP 端口" v-if="data.vhostHTTPPort != 0">
              <span>{{ data.vhostHTTPPort }}</span>
            </el-form-item>
            <el-form-item label="HTTPS 端口" v-if="data.vhostHTTPSPort != 0">
              <span>{{ data.vhostHTTPSPort }}</span>
            </el-form-item>
            <el-form-item
              label="TCPMux HTTP连接端口"
              v-if="data.tcpmuxHTTPConnectPort != 0"
            >
              <span>{{ data.tcpmuxHTTPConnectPort }}</span>
            </el-form-item>
            <el-form-item
              label="子域名主机"
              v-if="data.subdomainHost != ''"
            >
              <LongSpan :content="data.subdomainHost" :length="30"></LongSpan>
            </el-form-item>
            <el-form-item label="最大连接池数量">
              <span>{{ data.maxPoolCount }}</span>
            </el-form-item>
            <el-form-item label="每客户端最大端口数">
              <span>{{ data.maxPortsPerClient }}</span>
            </el-form-item>
            <el-form-item label="允许的端口" v-if="data.allowPortsStr != ''">
              <LongSpan :content="data.allowPortsStr" :length="30"></LongSpan>
            </el-form-item>
            <el-form-item label="强制TLS" v-if="data.tlsForce === true">
              <span>{{ data.tlsForce }}</span>
            </el-form-item>
            <el-form-item label="心跳超时">
              <span>{{ data.heartbeatTimeout }}</span>
            </el-form-item>
            <el-form-item label="客户端数量">
              <span>{{ data.clientCounts }}</span>
            </el-form-item>
            <el-form-item label="当前连接数">
              <span>{{ data.curConns }}</span>
            </el-form-item>
            <el-form-item label="代理数量">
              <span>{{ data.proxyCounts }}</span>
            </el-form-item>
          </el-form>
        </div>
      </el-col>
      <el-col :md="12">
        <div
          id="traffic"
          style="width: 400px; height: 250px; margin-bottom: 30px"
        ></div>
        <div id="proxies" style="width: 400px; height: 250px"></div>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { DrawTrafficChart, DrawProxyChart } from '../utils/chart'
import LongSpan from './LongSpan.vue'

let data = ref({
  version: '',
  bindPort: 0,
  kcpBindPort: 0,
  quicBindPort: 0,
  vhostHTTPPort: 0,
  vhostHTTPSPort: 0,
  tcpmuxHTTPConnectPort: 0,
  subdomainHost: '',
  maxPoolCount: 0,
  maxPortsPerClient: '',
  allowPortsStr: '',
  tlsForce: false,
  heartbeatTimeout: 0,
  clientCounts: 0,
  curConns: 0,
  proxyCounts: 0,
})

const fetchData = () => {
  fetch('../api/serverinfo', { credentials: 'include' })
    .then((res) => res.json())
    .then((json) => {
      data.value.version = json.version
      data.value.bindPort = json.bindPort
      data.value.kcpBindPort = json.kcpBindPort
      data.value.quicBindPort = json.quicBindPort
      data.value.vhostHTTPPort = json.vhostHTTPPort
      data.value.vhostHTTPSPort = json.vhostHTTPSPort
      data.value.tcpmuxHTTPConnectPort = json.tcpmuxHTTPConnectPort
      data.value.subdomainHost = json.subdomainHost
      data.value.maxPoolCount = json.maxPoolCount
      data.value.maxPortsPerClient = json.maxPortsPerClient
      if (data.value.maxPortsPerClient == '0') {
        data.value.maxPortsPerClient = '无限制'
      }
      data.value.allowPortsStr = json.allowPortsStr
      data.value.tlsForce = json.tlsForce
      data.value.heartbeatTimeout = json.heartbeatTimeout
      data.value.clientCounts = json.clientCounts
      data.value.curConns = json.curConns
      data.value.proxyCounts = 0
      if (json.proxyTypeCount != null) {
        if (json.proxyTypeCount.tcp != null) {
          data.value.proxyCounts += json.proxyTypeCount.tcp
        }
        if (json.proxyTypeCount.udp != null) {
          data.value.proxyCounts += json.proxyTypeCount.udp
        }
        if (json.proxyTypeCount.http != null) {
          data.value.proxyCounts += json.proxyTypeCount.http
        }
        if (json.proxyTypeCount.https != null) {
          data.value.proxyCounts += json.proxyTypeCount.https
        }
        if (json.proxyTypeCount.stcp != null) {
          data.value.proxyCounts += json.proxyTypeCount.stcp
        }
        if (json.proxyTypeCount.sudp != null) {
          data.value.proxyCounts += json.proxyTypeCount.sudp
        }
        if (json.proxyTypeCount.xtcp != null) {
          data.value.proxyCounts += json.proxyTypeCount.xtcp
        }
      }

      // draw chart
      DrawTrafficChart('traffic', json.totalTrafficIn, json.totalTrafficOut)
      DrawProxyChart('proxies', json)
    })
    .catch(() => {
      ElMessage({
        showClose: true,
        message: '从frps获取服务器信息失败！',
        type: 'warning',
      })
    })
}
fetchData()
</script>

<style>
.source {
  border-radius: 4px;
  transition: 0.2s;
  padding-left: 24px;
  padding-right: 24px;
}

.server_info {
  margin-left: 40px;
  font-size: 0px;
}

.server_info .el-form-item__label {
  color: #99a9bf;
  height: 40px;
  line-height: 40px;
}

.server_info .el-form-item__content {
  height: 40px;
  line-height: 40px;
}

.server_info .el-form-item {
  margin-right: 0;
  margin-bottom: 0;
  width: 100%;
}
</style>
