<template>
  <div>
    <n-card title="客户端配置" class="config-card">
      <template #header-extra>
        <n-space>
          <n-button @click="fetchData" type="default" size="small">
            <template #icon>
              <n-icon><RefreshIcon /></n-icon>
            </template>
            刷新
          </n-button>
          <n-button @click="uploadConfig" type="primary" size="small">
            <template #icon>
              <n-icon><UploadIcon /></n-icon>
            </template>
            上传配置
          </n-button>
        </n-space>
      </template>
      <n-input
        v-model:value="configContent"
        type="textarea"
        placeholder="请输入frpc配置内容"
        :autosize="{ minRows: 20, maxRows: 30 }"
      />
    </n-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { NCard, NInput, NButton, NSpace, NIcon, useMessage, useDialog } from 'naive-ui'
import { Refresh as RefreshIcon, Upload as UploadIcon } from '@vicons/ionicons5'

const message = useMessage()
const dialog = useDialog()
const configContent = ref('')

const fetchData = () => {
  fetch('/api/config', { credentials: 'include' })
    .then((res) => {
      return res.text()
    })
    .then((text) => {
      configContent.value = text
    })
    .catch((err) => {
      message.error('获取配置信息失败：' + err)
    })
}

const uploadConfig = () => {
  dialog.warning({
    title: '确认上传',
    content: '确定要上传新的配置吗？上传后需要重启frpc才能生效。',
    positiveText: '确认',
    negativeText: '取消',
    onPositiveClick: () => {
      fetch('/api/config', {
        method: 'POST',
        credentials: 'include',
        headers: {
          'Content-Type': 'text/plain',
        },
        body: configContent.value,
      })
        .then((res) => {
          if (res.status === 200) {
            message.success('配置上传成功')
            return
          }
          return res.text().then((text) => {
            throw new Error(text)
          })
        })
        .catch((err) => {
          message.error('配置上传失败：' + err)
        })
    }
  })
}

onMounted(() => {
  fetchData()
})
</script>

<style scoped>
.config-card {
  margin-bottom: 16px;
}
</style>
