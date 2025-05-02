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
            <el-form-item label="运行端口">
              <span>{{ data.bindPort }}</span>
            </el-form-item>
            <el-form-item label="KCP Bind Port" v-if="data.kcpBindPort != 0">
              <span>{{ data.kcpBindPort }}</span>
            </el-form-item>
            <el-form-item label="QUIC Bind Port" v-if="data.quicBindPort != 0">
              <span>{{ data.quicBindPort }}</span>
            </el-form-item>
            <el-form-item label="HTTP Port" v-if="data.vhostHTTPPort != 0">
              <span>{{ data.vhostHTTPPort }}</span>
            </el-form-item>
            <el-form-item label="HTTPS Port" v-if="data.vhostHTTPSPort != 0">
              <span>{{ data.vhostHTTPSPort }}</span>
            </el-form-item>
            <el-form-item
              label="TCPMux HTTPConnect Port"
              v-if="data.tcpmuxHTTPConnectPort != 0"
            >
              <span>{{ data.tcpmuxHTTPConnectPort }}</span>
            </el-form-item>
            <el-form-item
              label="Subdomain Host"
              v-if="data.subdomainHost != ''"
            >
              <LongSpan :content="data.subdomainHost" :length="30"></LongSpan>
            </el-form-item>
            <el-form-item label="最大连接数">
              <span>{{ data.maxPoolCount }}</span>
            </el-form-item>
            <el-form-item label="最大端口数">
              <span>{{ data.maxPortsPerClient }}</span>
            </el-form-item>
            <el-form-item label="允许端口" v-if="data.allowPortsStr != ''">
              <LongSpan :content="data.allowPortsStr" :length="30"></LongSpan>
            </el-form-item>
            <el-form-item label="TLS Force" v-if="data.tlsForce === true">
              <span>{{ data.tlsForce }}</span>
            </el-form-item>
            <el-form-item label="心跳超时">
              <span>{{ data.heartbeatTimeout }}</span>
            </el-form-item>
            <el-form-item label="客户端数">
              <span>{{ data.clientCounts }}</span>
            </el-form-item>
            <el-form-item label="当前连接数">
              <span>{{ data.curConns }}</span>
            </el-form-item>
            <el-form-item label="隧道数">
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

    <!-- 新增流量趋势图表区域 -->
    <el-row style="margin-top: 40px;">
      <el-col :span="24">
        <div class="traffic-trend-container">
          <div class="traffic-trend-header">
            <h3>流量趋势</h3>
            <div class="traffic-trend-controls">
              <el-radio-group v-model="timeRange" size="small" @change="fetchTrafficTrend">
                <el-radio-button label="day">天</el-radio-button>
                <el-radio-button label="3days">3天</el-radio-button>
                <el-radio-button label="week">周</el-radio-button>
                <el-radio-button label="14days">14天</el-radio-button>
                <el-radio-button label="month">月</el-radio-button>
              </el-radio-group>
              
              <el-radio-group v-model="trafficType" size="small" @change="updateTrafficChart" style="margin-left: 15px;">
                <el-radio-button label="all">全部</el-radio-button>
                <el-radio-button label="in">入站</el-radio-button>
                <el-radio-button label="out">出站</el-radio-button>
              </el-radio-group>
            </div>
          </div>
          <div id="trafficTrend" style="width: 100%; height: 400px;"></div>
        </div>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, watch, onUnmounted } from 'vue'
import { ElMessage } from 'element-plus'
import { DrawTrafficChart, DrawProxyChart } from '../utils/chart'
import LongSpan from './LongSpan.vue'
import * as echarts from 'echarts'

// 声明组件的数据和方法
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

// 流量趋势图配置
const timeRange = ref('day') // 默认显示当天
const trafficType = ref('all') // 默认显示所有流量
let trafficChart: echarts.ECharts | null = null
let trafficData = ref({
  timestamps: [] as string[],
  inData: [] as number[],
  outData: [] as number[]
})

// 添加缓存数据的ref
const cachedTrafficData = ref({
  trafficIn: [] as number[],
  trafficOut: [] as number[]
})

// 分离数据处理逻辑
const processTrafficData = () => {
  const totalDays = cachedTrafficData.value.trafficIn.length
  console.log('总天数:', totalDays)
  
  let dataPoints = 0
  let startPosition = 0
  let endPosition = totalDays
  
  switch(timeRange.value) {
    case 'day':
      startPosition = 0
      endPosition = 1
      dataPoints = 1
      break
    case '3days':
      startPosition = 0
      endPosition = Math.min(3, totalDays)
      dataPoints = endPosition - startPosition
      break
    case 'week':
      startPosition = 0
      endPosition = Math.min(7, totalDays)
      dataPoints = endPosition - startPosition
      break
    case '14days':
      startPosition = 0
      endPosition = Math.min(14, totalDays)
      dataPoints = endPosition - startPosition
      break
    case 'month':
    default:
      startPosition = 0
      endPosition = totalDays
      dataPoints = totalDays
      break
  }
  
  console.log('数据范围:', { startPosition, endPosition, dataPoints })
  
  // 生成时间戳（从今天开始往前推）
  const now = new Date()
  const timestamps = []
  const inData = []
  const outData = []
  
  for (let i = 0; i < dataPoints; i++) {
    const date = new Date(now)
    date.setDate(date.getDate() - i)
    timestamps.unshift(date.toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' }))
    
    inData.unshift(cachedTrafficData.value.trafficIn[i] || 0)
    outData.unshift(cachedTrafficData.value.trafficOut[i] || 0)
  }
  
  console.log('处理后的数据:', {
    timestamps,
    inData,
    outData
  })
  
  return {
    timestamps,
    inData,
    outData
  }
}

// 修改获取流量数据的函数
const fetchTrafficTrend = () => {
  // 如果已有缓存数据，直接处理
  if (cachedTrafficData.value.trafficIn.length > 0) {
    trafficData.value = processTrafficData()
    updateTrafficChart()
    return
  }

  // 没有缓存数据时才请求API
  fetch('../api/traffic', { credentials: 'include' })
    .then(res => res.json())
    .then(json => {
      console.log('原始流量数据:', json)
      
      if (!json || !Array.isArray(json.trafficIn) || !Array.isArray(json.trafficOut)) {
        throw new Error('数据格式错误: ' + JSON.stringify(json))
      }
      
      // 缓存数据
      cachedTrafficData.value = {
        trafficIn: json.trafficIn,
        trafficOut: json.trafficOut
      }
      
      // 处理数据并更新图表
      trafficData.value = processTrafficData()
      updateTrafficChart()
    })
    .catch((error) => {
      console.error('获取流量数据错误:', error)
      ElMessage({
        showClose: true,
        message: '获取流量趋势数据失败: ' + error.message,
        type: 'warning'
      })
      
      // 模拟数据用于开发展示
      generateMockData()
      updateTrafficChart()
    })
}

// 修改时间范围变化的处理函数
const handleTimeRangeChange = () => {
  trafficData.value = processTrafficData()
  updateTrafficChart()
}

// 生成模拟数据用于开发
const generateMockData = () => {
  const now = new Date()
  const timestamps: string[] = []
  const inData: number[] = []
  const outData: number[] = []
  
  let days = 1
  switch(timeRange.value) {
    case '3days': days = 3; break;
    case 'week': days = 7; break;
    case '14days': days = 14; break;
    case 'month': days = 30; break;
    default: days = 1;
  }
  
  const points = days * 24 // 每小时一个点
  for (let i = 0; i < points; i++) {
    const time = new Date(now.getTime() - (points - i) * 60 * 60 * 1000)
    timestamps.push(time.toISOString().substr(5, 11))
    inData.push(Math.random() * 1000)
    outData.push(Math.random() * 800)
  }
  
  trafficData.value = {
    timestamps,
    inData,
    outData
  }
}

// 更新流量图表
const updateTrafficChart = () => {
  if (!trafficChart) return
  
  // 清除现有的图表数据
  trafficChart.clear()
  
  const option: echarts.EChartsOption = {
    title: {
      text: '服务器流量趋势',
      textStyle: {
        fontSize: 16,
        fontWeight: 'normal'
      },
      left: 'center',
      top: 0,
      padding: [10, 0]
    },
    tooltip: {
      trigger: 'axis',
      backgroundColor: 'rgba(255, 255, 255, 0.9)',
      borderColor: '#eee',
      borderWidth: 1,
      textStyle: {
        color: '#333'
      },
      padding: [10, 15],
      formatter: function(params: any) {
        let result = `<div style="font-weight: bold; margin-bottom: 5px;">${params[0].name}</div>`
        params.forEach((param: any) => {
          const bytes = param.value
          let unit = 'B'
          let value = bytes
          
          if (bytes >= 1024 * 1024 * 1024) {
            value = bytes / (1024 * 1024 * 1024)
            unit = 'GB'
          } else if (bytes >= 1024 * 1024) {
            value = bytes / (1024 * 1024)
            unit = 'MB'
          } else if (bytes >= 1024) {
            value = bytes / 1024
            unit = 'KB'
          }
          
          const color = param.seriesName === '入站流量' ? '#409EFF' : '#67C23A'
          result += `<div style="display: flex; justify-content: space-between; align-items: center; margin: 5px 0;">
            <span style="display: inline-block; width: 10px; height: 10px; border-radius: 50%; background-color: ${color}; margin-right: 8px;"></span>
            <span style="flex: 1;">${param.seriesName}:</span>
            <span style="font-weight: bold; margin-left: 15px;">${value.toFixed(2)} ${unit}</span>
          </div>`
        })
        return result
      }
    },
    legend: {
      bottom: 0,
      padding: [15, 0],
      itemGap: 30,
      itemWidth: 14,
      itemHeight: 14,
      textStyle: {
        padding: [0, 0, 0, 4]
      }
    } as echarts.LegendComponentOption,
    grid: {
      left: '3%',
      right: '4%',
      bottom: '15%',
      top: '15%',
      containLabel: true
    },
    xAxis: {
      type: 'category',
      boundaryGap: timeRange.value === 'day',
      data: trafficData.value.timestamps,
      axisLine: {
        lineStyle: {
          color: '#ddd'
        }
      },
      axisTick: {
        alignWithLabel: true
      },
      axisLabel: {
        color: '#666',
        margin: 12
      }
    },
    yAxis: {
      type: 'value',
      axisLine: {
        show: false
      },
      axisTick: {
        show: false
      },
      splitLine: {
        lineStyle: {
          color: '#eee',
          type: 'dashed'
        }
      },
      axisLabel: {
        color: '#666',
        margin: 16,
        formatter: function(value: number) {
          if (value >= 1024 * 1024 * 1024) {
            return (value / (1024 * 1024 * 1024)).toFixed(2) + ' GB'
          } else if (value >= 1024 * 1024) {
            return (value / (1024 * 1024)).toFixed(2) + ' MB'
          } else if (value >= 1024) {
            return (value / 1024).toFixed(2) + ' KB'
          }
          return value + ' B'
        }
      }
    },
    series: []
  }
  
  const seriesData: any[] = []
  
  // 入站流量配置
  const inFlowSeries = {
    name: '入站流量',
    type: timeRange.value === 'day' ? 'bar' : 'line',
    smooth: timeRange.value !== 'day',
    barWidth: timeRange.value === 'day' ? '25%' : undefined,
    barGap: '10%',
    data: trafficData.value.inData,
    itemStyle: {
      color: '#409EFF',
      borderRadius: timeRange.value === 'day' ? [4, 4, 0, 0] : undefined
    },
    emphasis: {
      itemStyle: {
        color: timeRange.value === 'day' ? '#66b1ff' : undefined
      }
    },
    areaStyle: timeRange.value !== 'day' ? {
      color: {
        type: 'linear',
        x: 0,
        y: 0,
        x2: 0,
        y2: 1,
        colorStops: [
          { offset: 0, color: 'rgba(64, 158, 255, 0.35)' },
          { offset: 1, color: 'rgba(64, 158, 255, 0.05)' }
        ]
      }
    } : undefined,
    showSymbol: false,
    lineStyle: timeRange.value !== 'day' ? {
      width: 3
    } : undefined
  }

  // 出站流量配置
  const outFlowSeries = {
    name: '出站流量',
    type: timeRange.value === 'day' ? 'bar' : 'line',
    smooth: timeRange.value !== 'day',
    barWidth: timeRange.value === 'day' ? '25%' : undefined,
    barGap: '10%',
    data: trafficData.value.outData,
    itemStyle: {
      color: '#67C23A',
      borderRadius: timeRange.value === 'day' ? [4, 4, 0, 0] : undefined
    },
    emphasis: {
      itemStyle: {
        color: timeRange.value === 'day' ? '#85ce61' : undefined
      }
    },
    areaStyle: timeRange.value !== 'day' ? {
      color: {
        type: 'linear',
        x: 0,
        y: 0,
        x2: 0,
        y2: 1,
        colorStops: [
          { offset: 0, color: 'rgba(103, 194, 58, 0.35)' },
          { offset: 1, color: 'rgba(103, 194, 58, 0.05)' }
        ]
      }
    } : undefined,
    showSymbol: false,
    lineStyle: timeRange.value !== 'day' ? {
      width: 3
    } : undefined
  }

  // 根据选择的流量类型添加对应的系列
  switch (trafficType.value) {
    case 'all':
      seriesData.push(inFlowSeries, outFlowSeries)
      break
    case 'in':
      seriesData.push(inFlowSeries)
      break
    case 'out':
      seriesData.push(outFlowSeries)
      break
  }

  // 设置图例数据
  const legendOption = option.legend as echarts.LegendComponentOption
  legendOption.data = seriesData.map(item => item.name)
  
  option.series = seriesData
  
  // 使用setOption的第二个参数来强制更新
  trafficChart.setOption(option, true)
}

// 修改暗色模式下的样式
const updateDarkMode = () => {
  const isDark = document.documentElement.classList.contains('dark')
  if (!trafficChart) return
  
  trafficChart.setOption({
    title: {
      textStyle: {
        color: isDark ? '#E5EAF3' : '#333'
      }
    },
    tooltip: {
      backgroundColor: isDark ? 'rgba(30, 30, 30, 0.9)' : 'rgba(255, 255, 255, 0.9)',
      borderColor: isDark ? '#555' : '#eee',
      textStyle: {
        color: isDark ? '#E5EAF3' : '#333'
      }
    },
    xAxis: {
      axisLine: {
        lineStyle: {
          color: isDark ? '#555' : '#ddd'
        }
      },
      axisLabel: {
        color: isDark ? '#909399' : '#666'
      }
    },
    yAxis: {
      splitLine: {
        lineStyle: {
          color: isDark ? '#555' : '#eee'
        }
      },
      axisLabel: {
        color: isDark ? '#909399' : '#666'
      }
    }
  })
}

// 监听主题变化
const observer = new MutationObserver(() => {
  updateDarkMode()
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
        data.value.maxPortsPerClient = 'no limit'
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
        message: 'Get server info from frps failed!',
        type: 'warning',
      })
    })
}

onMounted(() => {
  fetchData()
  
  // 初始化流量趋势图表
  trafficChart = echarts.init(document.getElementById('trafficTrend') as HTMLElement)
  fetchTrafficTrend()
  
  // 响应容器大小变化
  window.addEventListener('resize', () => {
    trafficChart?.resize()
  })
  
  // 监听主题变化
  observer.observe(document.documentElement, {
    attributes: true,
    attributeFilter: ['class']
  })
  
  // 初始化主题
  updateDarkMode()
})

onUnmounted(() => {
  // 清理主题监听
  observer.disconnect()
})

// 监听时间范围变化
watch(timeRange, () => {
  handleTimeRangeChange()
})

// 监听流量类型变化
watch(trafficType, () => {
  if (trafficChart) {
    updateTrafficChart()
  }
})
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

/* 流量趋势图样式 */
.traffic-trend-container {
  background-color: white;
  border-radius: 4px;
  padding: 20px;
  box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.1);
}

.traffic-trend-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.traffic-trend-header h3 {
  margin: 0;
  font-size: 18px;
  font-weight: bold;
}

.traffic-trend-controls {
  display: flex;
  align-items: center;
}

html.dark .traffic-trend-container {
  background-color: #1d1e1f;
  box-shadow: 0 2px 12px 0 rgba(0, 0, 0, 0.3);
}

html.dark .traffic-trend-header h3 {
  color: #e5eaf3;
}
</style>
