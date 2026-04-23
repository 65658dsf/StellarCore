<template>
  <ProxyView :proxies="proxies" proxyType="tcp" @refresh="fetchData" />
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { getProxiesByType } from '../api/proxy'
import { TCPProxy } from '../utils/proxy'
import ProxyView from './ProxyView.vue'

const proxies = ref<TCPProxy[]>([])

const fetchData = async () => {
  try {
    const json = await getProxiesByType('tcp')
    proxies.value = json.proxies.map((proxyStats) => new TCPProxy(proxyStats))
  } catch (error: any) {
    ElMessage({
      message: `获取 TCP 代理失败: ${error.message}`,
      type: 'error',
    })
  }
}

onMounted(() => {
  fetchData()
})
</script>
