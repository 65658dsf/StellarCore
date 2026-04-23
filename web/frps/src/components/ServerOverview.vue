<template>
  <div class="server-overview">
    <el-row :gutter="20" class="stats-row">
      <el-col :xs="24" :sm="12" :lg="6">
        <StatCard
          label="客户端"
          :value="data.clientCounts"
          type="clients"
          subtitle="当前在线客户端"
        />
      </el-col>
      <el-col :xs="24" :sm="12" :lg="6">
        <StatCard
          label="代理"
          :value="data.proxyCounts"
          type="proxies"
          subtitle="当前代理总数"
          to="/proxies/tcp"
        />
      </el-col>
      <el-col :xs="24" :sm="12" :lg="6">
        <StatCard
          label="连接数"
          :value="data.curConns"
          type="connections"
          subtitle="当前活跃连接"
        />
      </el-col>
      <el-col :xs="24" :sm="12" :lg="6">
        <StatCard
          label="总流量"
          :value="formatTrafficTotal"
          type="traffic"
          subtitle="今日累计流量"
        />
      </el-col>
    </el-row>

    <el-row :gutter="20" class="charts-row">
      <el-col :xs="24" :md="12">
        <el-card class="chart-card" shadow="hover">
          <template #header>
            <div class="card-header">
              <span class="card-title">流量概览</span>
              <el-tag size="small" type="info">今日</el-tag>
            </div>
          </template>
          <div id="traffic-pie" class="chart-body"></div>
        </el-card>
      </el-col>

      <el-col :xs="24" :md="12">
        <el-card class="chart-card" shadow="hover">
          <template #header>
            <div class="card-header">
              <span class="card-title">代理类型分布</span>
              <el-tag size="small" type="info">当前</el-tag>
            </div>
          </template>
          <div v-if="hasActiveProxies" id="proxy-pie" class="chart-body"></div>
          <div v-else class="chart-empty">当前没有活跃代理</div>
        </el-card>
      </el-col>
    </el-row>

    <el-card class="trend-card" shadow="hover">
      <template #header>
        <div class="card-header">
          <div>
            <span class="card-title">聚合流量趋势</span>
            <p class="card-subtitle">按最近 30 个统计点展示入站 / 出站流量</p>
          </div>
          <el-button :loading="loading" @click="fetchTraffic">
            刷新趋势
          </el-button>
        </div>
      </template>
      <div id="traffic-trend" class="trend-body"></div>
    </el-card>

    <el-card class="config-card" shadow="hover">
      <template #header>
        <div class="card-header">
          <div>
            <span class="card-title">服务配置</span>
            <p class="card-subtitle">当前 frps 运行参数与端口信息</p>
          </div>
          <div class="header-actions">
            <el-button :loading="loading" @click="fetchData">刷新</el-button>
            <el-button type="danger" :loading="restarting" @click="handleRestart">
              重启服务
            </el-button>
          </div>
        </div>
      </template>

      <div class="config-grid">
        <div class="config-item">
          <span class="config-label">版本</span>
          <span class="config-value">{{ data.version || '-' }}</span>
        </div>
        <div class="config-item">
          <span class="config-label">监听端口</span>
          <span class="config-value">{{ data.bindPort }}</span>
        </div>
        <div v-if="data.kcpBindPort" class="config-item">
          <span class="config-label">KCP 端口</span>
          <span class="config-value">{{ data.kcpBindPort }}</span>
        </div>
        <div v-if="data.quicBindPort" class="config-item">
          <span class="config-label">QUIC 端口</span>
          <span class="config-value">{{ data.quicBindPort }}</span>
        </div>
        <div v-if="data.vhostHTTPPort" class="config-item">
          <span class="config-label">HTTP 端口</span>
          <span class="config-value">{{ data.vhostHTTPPort }}</span>
        </div>
        <div v-if="data.vhostHTTPSPort" class="config-item">
          <span class="config-label">HTTPS 端口</span>
          <span class="config-value">{{ data.vhostHTTPSPort }}</span>
        </div>
        <div v-if="data.tcpmuxHTTPConnectPort" class="config-item">
          <span class="config-label">TCPMux 端口</span>
          <span class="config-value">{{ data.tcpmuxHTTPConnectPort }}</span>
        </div>
        <div v-if="data.subdomainHost" class="config-item">
          <span class="config-label">子域名 Host</span>
          <span class="config-value">{{ data.subdomainHost }}</span>
        </div>
        <div class="config-item">
          <span class="config-label">最大连接池</span>
          <span class="config-value">{{ data.maxPoolCount }}</span>
        </div>
        <div class="config-item">
          <span class="config-label">单客户端最大端口数</span>
          <span class="config-value">{{ data.maxPortsPerClient }}</span>
        </div>
        <div v-if="data.allowPortsStr" class="config-item">
          <span class="config-label">允许端口范围</span>
          <span class="config-value">{{ data.allowPortsStr }}</span>
        </div>
        <div class="config-item">
          <span class="config-label">TLS 强制启用</span>
          <span class="config-value">{{ data.tlsForce ? '是' : '否' }}</span>
        </div>
        <div class="config-item">
          <span class="config-label">心跳超时</span>
          <span class="config-value">{{ data.heartbeatTimeout }}s</span>
        </div>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import * as echarts from 'echarts'
import StatCard from './StatCard.vue'
import { getAllTraffic, getServerInfo, restartService } from '../api/server'
import { formatFileSize } from '../utils/format'
import { DrawProxyChart, DrawTrafficChart } from '../utils/chart'

type ServerOverviewData = {
  version: string
  bindPort: number
  kcpBindPort: number
  quicBindPort: number
  vhostHTTPPort: number
  vhostHTTPSPort: number
  tcpmuxHTTPConnectPort: number
  subdomainHost: string
  maxPoolCount: number
  maxPortsPerClient: string
  allowPortsStr: string
  tlsForce: boolean
  heartbeatTimeout: number
  clientCounts: number
  curConns: number
  proxyCounts: number
  totalTrafficIn: number
  totalTrafficOut: number
  proxyTypeCounts: Record<string, number>
}

const data = ref<ServerOverviewData>({
  version: '',
  bindPort: 0,
  kcpBindPort: 0,
  quicBindPort: 0,
  vhostHTTPPort: 0,
  vhostHTTPSPort: 0,
  tcpmuxHTTPConnectPort: 0,
  subdomainHost: '',
  maxPoolCount: 0,
  maxPortsPerClient: '0',
  allowPortsStr: '',
  tlsForce: false,
  heartbeatTimeout: 0,
  clientCounts: 0,
  curConns: 0,
  proxyCounts: 0,
  totalTrafficIn: 0,
  totalTrafficOut: 0,
  proxyTypeCounts: {},
})

const loading = ref(false)
const restarting = ref(false)
const trafficSeries = ref({
  trafficIn: [] as number[],
  trafficOut: [] as number[],
})

let trendChart: echarts.ECharts | null = null

const formatTrafficTotal = computed(() =>
  formatFileSize(data.value.totalTrafficIn + data.value.totalTrafficOut),
)

const hasActiveProxies = computed(() =>
  Object.values(data.value.proxyTypeCounts).some((value) => value > 0),
)

const updateSummaryCharts = async () => {
  await nextTick()
  DrawTrafficChart('traffic-pie', data.value.totalTrafficIn, data.value.totalTrafficOut)
  if (hasActiveProxies.value) {
    DrawProxyChart('proxy-pie', { proxyTypeCount: data.value.proxyTypeCounts })
  }
}

const updateTrendChart = async () => {
  await nextTick()
  const container = document.getElementById('traffic-trend')
  if (!container) {
    return
  }

  if (!trendChart) {
    trendChart = echarts.init(container)
  }

  const pointCount = Math.max(
    trafficSeries.value.trafficIn.length,
    trafficSeries.value.trafficOut.length,
  )
  const timestamps = Array.from({ length: pointCount }, (_, index) => {
    const date = new Date()
    date.setDate(date.getDate() - (pointCount - index - 1))
    return `${date.getMonth() + 1}-${date.getDate()}`
  })

  const inbound = [...trafficSeries.value.trafficIn].reverse()
  const outbound = [...trafficSeries.value.trafficOut].reverse()

  trendChart.setOption({
    tooltip: {
      trigger: 'axis',
      valueFormatter: (value: number) => formatFileSize(value),
    },
    legend: {
      data: ['入站', '出站'],
    },
    grid: {
      left: '3%',
      right: '4%',
      bottom: '3%',
      containLabel: true,
    },
    xAxis: {
      type: 'category',
      data: timestamps,
      boundaryGap: false,
    },
    yAxis: {
      type: 'value',
      axisLabel: {
        formatter: (value: number) => formatFileSize(value),
      },
    },
    series: [
      {
        name: '入站',
        type: 'line',
        smooth: true,
        data: inbound,
      },
      {
        name: '出站',
        type: 'line',
        smooth: true,
        data: outbound,
      },
    ],
  })
}

const applyServerInfo = async (serverInfo: any) => {
  data.value.version = serverInfo.version
  data.value.bindPort = serverInfo.bindPort
  data.value.kcpBindPort = serverInfo.kcpBindPort
  data.value.quicBindPort = serverInfo.quicBindPort
  data.value.vhostHTTPPort = serverInfo.vhostHTTPPort
  data.value.vhostHTTPSPort = serverInfo.vhostHTTPSPort
  data.value.tcpmuxHTTPConnectPort = serverInfo.tcpmuxHTTPConnectPort
  data.value.subdomainHost = serverInfo.subdomainHost
  data.value.maxPoolCount = serverInfo.maxPoolCount
  data.value.maxPortsPerClient =
    String(serverInfo.maxPortsPerClient) === '0'
      ? '无限制'
      : String(serverInfo.maxPortsPerClient)
  data.value.allowPortsStr = serverInfo.allowPortsStr
  data.value.tlsForce = serverInfo.tlsForce
  data.value.heartbeatTimeout = serverInfo.heartbeatTimeout
  data.value.clientCounts = serverInfo.clientCounts
  data.value.curConns = serverInfo.curConns
  data.value.totalTrafficIn = serverInfo.totalTrafficIn
  data.value.totalTrafficOut = serverInfo.totalTrafficOut
  data.value.proxyTypeCounts = serverInfo.proxyTypeCount || {}
  data.value.proxyCounts = Object.values(data.value.proxyTypeCounts).reduce(
    (sum, value) => sum + value,
    0,
  )

  await updateSummaryCharts()
}

const fetchTraffic = async () => {
  try {
    const traffic = await getAllTraffic()
    trafficSeries.value = {
      trafficIn: traffic.trafficIn || [],
      trafficOut: traffic.trafficOut || [],
    }
    await updateTrendChart()
  } catch (error: any) {
    ElMessage({
      showClose: true,
      message: `获取聚合流量失败: ${error.message}`,
      type: 'warning',
    })
  }
}

const fetchData = async () => {
  loading.value = true
  try {
    const serverInfo = await getServerInfo()
    await applyServerInfo(serverInfo)
    await fetchTraffic()
  } catch (error: any) {
    ElMessage({
      showClose: true,
      message: `获取服务信息失败: ${error.message}`,
      type: 'error',
    })
  } finally {
    loading.value = false
  }
}

const pollUntilRecovered = async () => {
  const start = Date.now()
  while (Date.now() - start < 30000) {
    await new Promise((resolve) => window.setTimeout(resolve, 1500))
    try {
      const serverInfo = await getServerInfo()
      await applyServerInfo(serverInfo)
      await fetchTraffic()
      ElMessage({
        message: '服务已恢复',
        type: 'success',
      })
      return
    } catch {
      // keep polling until timeout
    }
  }

  ElMessage({
    showClose: true,
    message: '服务重启指令已发送，但面板暂未恢复，请稍后手动刷新',
    type: 'warning',
  })
}

const handleRestart = async () => {
  try {
    await ElMessageBox.confirm(
      '重启 frps 会导致管理面板短暂断开，确定继续吗？',
      '确认重启',
      {
        confirmButtonText: '重启',
        cancelButtonText: '取消',
        type: 'warning',
      },
    )
  } catch {
    return
  }

  restarting.value = true
  try {
    await restartService()
    ElMessage({
      message: '重启指令已发送，正在等待服务恢复',
      type: 'success',
    })
    await pollUntilRecovered()
  } catch (error: any) {
    ElMessage({
      showClose: true,
      message: `重启服务失败: ${error.message}`,
      type: 'error',
    })
  } finally {
    restarting.value = false
  }
}

const handleResize = () => {
  trendChart?.resize()
}

onMounted(() => {
  fetchData()
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
  trendChart?.dispose()
  trendChart = null
})
</script>

<style scoped>
.server-overview {
  padding-bottom: 20px;
}

.stats-row,
.charts-row {
  margin-bottom: 20px;
}

.chart-card,
.trend-card,
.config-card {
  border-radius: 12px;
  border: 1px solid #e4e7ed;
}

html.dark .chart-card,
html.dark .trend-card,
html.dark .config-card {
  border-color: #3a3d5c;
  background: #27293d;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
}

.card-title {
  font-size: 16px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.card-subtitle {
  margin: 4px 0 0;
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

.header-actions {
  display: flex;
  gap: 12px;
}

.chart-body {
  width: 100%;
  height: 260px;
}

.trend-body {
  width: 100%;
  height: 360px;
}

.chart-empty {
  min-height: 260px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--el-text-color-secondary);
}

.config-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 16px;
}

.config-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 12px;
  background: #f8f9fa;
  border-radius: 8px;
}

html.dark .config-item {
  background: #1e1e2d;
}

.config-label {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.config-value {
  font-size: 14px;
  font-weight: 600;
  color: var(--el-text-color-primary);
  word-break: break-all;
}

@media (max-width: 768px) {
  .card-header {
    flex-direction: column;
    align-items: flex-start;
  }

  .header-actions {
    width: 100%;
  }
}
</style>
