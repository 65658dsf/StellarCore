<template>
  <ProxyView :proxies="proxies" proxyType="udp" @refresh="fetchData" />
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { getProxiesByType } from '../api/proxy'
import { UDPProxy } from '../utils/proxy'
import ProxyView from './ProxyView.vue'

const proxies = ref<UDPProxy[]>([])

const fetchData = async () => {
  try {
    const json = await getProxiesByType('udp')
    proxies.value = json.proxies.map((proxyStats) => new UDPProxy(proxyStats))
  } catch (error: any) {
    ElMessage({
      message: `获取 UDP 代理失败: ${error.message}`,
      type: 'error',
    })
  }
}

onMounted(() => {
  fetchData()
})
</script>
