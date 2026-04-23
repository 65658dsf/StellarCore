<template>
  <ProxyView :proxies="proxies" proxyType="sudp" @refresh="fetchData" />
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { getProxiesByType } from '../api/proxy'
import { SUDPProxy } from '../utils/proxy'
import ProxyView from './ProxyView.vue'

const proxies = ref<SUDPProxy[]>([])

const fetchData = async () => {
  try {
    const json = await getProxiesByType('sudp')
    proxies.value = json.proxies.map((proxyStats) => new SUDPProxy(proxyStats))
  } catch (error: any) {
    ElMessage({
      message: `获取 SUDP 代理失败: ${error.message}`,
      type: 'error',
    })
  }
}

onMounted(() => {
  fetchData()
})
</script>
