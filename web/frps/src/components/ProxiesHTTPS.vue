<template>
  <ProxyView :proxies="proxies" proxyType="https" @refresh="fetchData" />
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { getProxiesByType } from '../api/proxy'
import { getServerInfo } from '../api/server'
import { HTTPSProxy } from '../utils/proxy'
import ProxyView from './ProxyView.vue'

const proxies = ref<HTTPSProxy[]>([])

const fetchData = async () => {
  try {
    const [serverInfo, proxyInfo] = await Promise.all([
      getServerInfo(),
      getProxiesByType('https'),
    ])

    if (!serverInfo.vhostHTTPSPort) {
      proxies.value = []
      return
    }

    proxies.value = proxyInfo.proxies.map(
      (proxyStats) =>
        new HTTPSProxy(
          proxyStats,
          serverInfo.vhostHTTPSPort,
          serverInfo.subdomainHost,
        ),
    )
  } catch (error: any) {
    ElMessage({
      message: `获取 HTTPS 代理失败: ${error.message}`,
      type: 'error',
    })
  }
}

onMounted(() => {
  fetchData()
})
</script>
