<template>
  <div class="config-page">
    <div class="page-header">
      <div>
        <h1 class="page-title">配置浏览</h1>
        <p class="page-subtitle">查看当前 frps 主配置文件内容</p>
      </div>
      <el-button :icon="Refresh" :loading="loading" @click="fetchConfig">
        刷新
      </el-button>
    </div>

    <el-card class="config-card" shadow="hover">
      <template #header>
        <div class="card-header">
          <span class="card-title">frps 配置文件</span>
          <el-tag type="info" size="small">只读</el-tag>
        </div>
      </template>

      <el-input
        v-model="content"
        type="textarea"
        readonly
        :autosize="{ minRows: 20, maxRows: 40 }"
        class="config-editor"
        placeholder="# 当前配置将在这里显示"
      />
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Refresh } from '@element-plus/icons-vue'
import { getConfigContent } from '../api/server'

const loading = ref(false)
const content = ref('')

const fetchConfig = async () => {
  loading.value = true
  try {
    content.value = await getConfigContent()
  } catch (error: any) {
    ElMessage({
      showClose: true,
      message: `读取配置失败: ${error.message}`,
      type: 'error',
    })
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchConfig()
})
</script>

<style scoped>
.config-page {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
}

.page-title {
  margin: 0;
  font-size: 28px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.page-subtitle {
  margin: 6px 0 0;
  color: var(--el-text-color-secondary);
  font-size: 14px;
}

.config-card {
  border-radius: 12px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.card-title {
  font-size: 16px;
  font-weight: 600;
}

.config-editor :deep(.el-textarea__inner) {
  font-family:
    ui-monospace, SFMono-Regular, SFMono, Menlo, Monaco, Consolas, monospace;
  font-size: 13px;
  line-height: 1.6;
}
</style>
