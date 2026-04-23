<template>
  <div :id="proxyName" style="width: 600px; height: 400px"></div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { getProxyTraffic } from '../api/proxy'
import { DrawProxyTrafficChart } from '../utils/chart'

const props = defineProps<{
  proxyName: string
}>()

const fetchData = async () => {
  try {
    const json = await getProxyTraffic(props.proxyName)
    DrawProxyTrafficChart(props.proxyName, json.trafficIn, json.trafficOut)
  } catch (error: any) {
    ElMessage({
      showClose: true,
      message: `获取流量信息失败: ${error.message}`,
      type: 'warning',
    })
  }
}

onMounted(() => {
  fetchData()
})
</script>
