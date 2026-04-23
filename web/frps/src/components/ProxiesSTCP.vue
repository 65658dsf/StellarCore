<template>
  <ProxyView :proxies="proxies" proxyType="stcp" @refresh="fetchData" />
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { getProxiesByType } from '../api/proxy'
import { STCPProxy } from '../utils/proxy'
import ProxyView from './ProxyView.vue'

const proxies = ref<STCPProxy[]>([])

const fetchData = async () => {
  try {
    const json = await getProxiesByType('stcp')
    proxies.value = json.proxies.map((proxyStats) => new STCPProxy(proxyStats))
  } catch (error: any) {
    ElMessage({
      message: `获取 STCP 代理失败: ${error.message}`,
      type: 'error',
    })
  }
}

onMounted(() => {
  fetchData()
})
</script>
