<template>
  <ProxyView :proxies="proxies" proxyType="tcpmux" @refresh="fetchData" />
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { getProxiesByType } from '../api/proxy'
import { getServerInfo } from '../api/server'
import { TCPMuxProxy } from '../utils/proxy'
import ProxyView from './ProxyView.vue'

const proxies = ref<TCPMuxProxy[]>([])

const fetchData = async () => {
  try {
    const [serverInfo, proxyInfo] = await Promise.all([
      getServerInfo(),
      getProxiesByType('tcpmux'),
    ])

    if (!serverInfo.tcpmuxHTTPConnectPort) {
      proxies.value = []
      return
    }

    proxies.value = proxyInfo.proxies.map(
      (proxyStats) =>
        new TCPMuxProxy(
          proxyStats,
          serverInfo.tcpmuxHTTPConnectPort,
          serverInfo.subdomainHost,
        ),
    )
  } catch (error: any) {
    ElMessage({
      message: `获取 TCPMux 代理失败: ${error.message}`,
      type: 'error',
    })
  }
}

onMounted(() => {
  fetchData()
})
</script>
