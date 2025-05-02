<template>
  <ProxyView :proxies="proxies" proxyType="tcp" @refresh="fetchData" />
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { TCPProxy } from '../utils/proxy.js'
import ProxyView from './ProxyView.vue'

let proxies = ref<TCPProxy[]>([])

const fetchData = () => {
  fetch('/api/proxy/tcp', { credentials: 'include' })
    .then((res) => {
      // 检查响应状态
      if (!res.ok) {
        return res.text().then(text => {
          throw new Error(`服务器返回错误: ${text || res.status}`)
        })
      }
      
      // 尝试解析为JSON
      return res.text().then(text => {
        try {
          return JSON.parse(text)
        } catch (e) {
          throw new Error(`解析JSON失败: ${text.substring(0, 50)}${text.length > 50 ? '...' : ''}`)
        }
      })
    })
    .then((json) => {
      proxies.value = []
      for (let proxyStats of json.proxies) {
        proxies.value.push(new TCPProxy(proxyStats))
      }
    })
    .catch((err) => {
      console.error('获取TCP隧道列表失败:', err)
    })
}
fetchData()
</script>

<style></style>
