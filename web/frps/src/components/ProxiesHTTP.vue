<template>
  <ProxyView :proxies="proxies" proxyType="http" @refresh="fetchData" />
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { getProxiesByType } from '../api/proxy'
import { getServerInfo } from '../api/server'
import { HTTPProxy } from '../utils/proxy'
import ProxyView from './ProxyView.vue'

const proxies = ref<HTTPProxy[]>([])

const fetchData = async () => {
  try {
    const [serverInfo, proxyInfo] = await Promise.all([
      getServerInfo(),
      getProxiesByType('http'),
    ])

    if (!serverInfo.vhostHTTPPort) {
      proxies.value = []
      return
    }

    proxies.value = proxyInfo.proxies.map(
      (proxyStats) =>
        new HTTPProxy(
          proxyStats,
          serverInfo.vhostHTTPPort,
          serverInfo.subdomainHost,
        ),
    )
  } catch (error: any) {
    ElMessage({
      message: `获取 HTTP 代理失败: ${error.message}`,
      type: 'error',
    })
  }
}

onMounted(() => {
  fetchData()
})
</script>
