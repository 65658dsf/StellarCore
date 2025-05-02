<template>
  <ProxyView :proxies="proxies" proxyType="http" @refresh="fetchData"/>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { HTTPProxy } from '../utils/proxy.ts'
import ProxyView from './ProxyView.vue'

let proxies = ref<HTTPProxy[]>([])

const fetchData = () => {
  let vhostHTTPPort: number
  let subdomainHost: string
  
  fetch('/api/serverinfo', { credentials: 'include' })
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
      vhostHTTPPort = json.vhostHTTPPort
      subdomainHost = json.subdomainHost
      if (vhostHTTPPort == null || vhostHTTPPort == 0) {
        return
      }
      
      return fetch('/api/proxy/http', { credentials: 'include' })
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
            proxies.value.push(new HTTPProxy(proxyStats, vhostHTTPPort, subdomainHost))
          }
        })
    })
    .catch((err) => {
      console.error('获取HTTP隧道列表失败:', err)
    })
}

fetchData()
</script>

<style></style>
