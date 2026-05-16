<template>
  <div class="logs-page">
    <div class="page-header">
      <div>
        <h1 class="page-title">日志</h1>
        <p class="page-subtitle">查看 frps 最近运行日志与更新进度</p>
      </div>
      <div class="page-actions">
        <el-switch
          v-model="autoRefresh"
          inline-prompt
          active-text="自动"
          inactive-text="暂停"
        />
        <el-button :icon="Refresh" :loading="loading" @click="refreshLogs">
          刷新
        </el-button>
      </div>
    </div>

    <el-card class="logs-card" shadow="hover">
      <template #header>
        <div class="logs-toolbar">
          <el-radio-group v-model="selectedLevel" size="small">
            <el-radio-button
              v-for="item in levelOptions"
              :key="item.value"
              :label="item.value"
            >
              {{ item.label }}
            </el-radio-button>
          </el-radio-group>
          <div class="logs-meta">
            <span>{{ entries.length }} 条</span>
            <el-tag v-if="truncated" type="warning" size="small">
              已截断
            </el-tag>
          </div>
        </div>
      </template>

      <div
        ref="logBody"
        v-loading="loading && entries.length === 0"
        class="log-body"
      >
        <el-empty
          v-if="entries.length === 0 && !loading"
          description="暂无日志"
        />
        <template v-else>
          <div v-for="entry in entries" :key="entry.id" class="log-line">
            <span class="log-time">{{ formatTime(entry.time) }}</span>
            <span class="log-level" :class="`level-${entry.level}`">
              {{ entry.level }}
            </span>
            <span class="log-message">{{ entry.message }}</span>
          </div>
        </template>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { Refresh } from '@element-plus/icons-vue'
import { getLogs, type LogEntry } from '../api/server'

const levelOptions = [
  { label: '全部', value: '' },
  { label: 'Trace', value: 'trace' },
  { label: 'Debug', value: 'debug' },
  { label: 'Info', value: 'info' },
  { label: 'Warn', value: 'warn' },
  { label: 'Error', value: 'error' },
]

const entries = ref<LogEntry[]>([])
const loading = ref(false)
const autoRefresh = ref(true)
const selectedLevel = ref('')
const truncated = ref(false)
const cursor = ref(0)
const logBody = ref<HTMLElement>()

let refreshTimer: number | undefined

const isNearBottom = () => {
  const el = logBody.value
  if (!el) return true
  return el.scrollHeight - el.scrollTop - el.clientHeight < 80
}

const scrollToBottom = async () => {
  await nextTick()
  const el = logBody.value
  if (el) {
    el.scrollTop = el.scrollHeight
  }
}

const fetchLogs = async (reset = false) => {
  const stickToBottom = isNearBottom()
  loading.value = true
  try {
    const response = await getLogs({
      cursor: reset ? undefined : cursor.value,
      limit: 300,
      level: selectedLevel.value,
    })
    cursor.value = response.nextCursor
    truncated.value = response.truncated

    if (reset) {
      entries.value = response.entries
    } else if (response.entries.length > 0) {
      const seen = new Set(entries.value.map((entry) => entry.id))
      entries.value = entries.value
        .concat(response.entries.filter((entry) => !seen.has(entry.id)))
        .slice(-1000)
    }

    if (stickToBottom) {
      await scrollToBottom()
    }
  } catch (error: any) {
    ElMessage({
      showClose: true,
      message: `读取日志失败: ${error.message}`,
      type: 'error',
    })
  } finally {
    loading.value = false
  }
}

const refreshLogs = () => {
  cursor.value = 0
  fetchLogs(true)
}

const startAutoRefresh = () => {
  window.clearInterval(refreshTimer)
  if (autoRefresh.value) {
    refreshTimer = window.setInterval(() => {
      fetchLogs(false)
    }, 2500)
  }
}

const formatTime = (value: string) => {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleTimeString('zh-CN', { hour12: false })
}

watch(autoRefresh, startAutoRefresh)
watch(selectedLevel, () => {
  refreshLogs()
})

onMounted(() => {
  refreshLogs()
  startAutoRefresh()
})

onUnmounted(() => {
  window.clearInterval(refreshTimer)
})
</script>

<style scoped>
.logs-page {
  display: flex;
  flex-direction: column;
  gap: 20px;
  padding-bottom: 20px;
}

.page-header,
.logs-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
}

.page-title {
  margin: 0;
  font-size: 24px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.page-subtitle {
  margin: 6px 0 0;
  font-size: 14px;
  color: var(--el-text-color-secondary);
}

.page-actions,
.logs-meta {
  display: flex;
  align-items: center;
  gap: 12px;
}

.logs-card {
  border-radius: 8px;
  border: 1px solid #e4e7ed;
}

html.dark .logs-card {
  border-color: #3a3d5c;
  background: #27293d;
}

.logs-meta {
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

.log-body {
  height: min(68vh, 720px);
  min-height: 420px;
  overflow: auto;
  padding: 10px 0;
  background: #0f172a;
  border-radius: 8px;
  font-family:
    Fira Code,
    Consolas,
    Monaco,
    monospace;
  font-size: 12px;
  line-height: 1.7;
}

.log-line {
  display: grid;
  grid-template-columns: 88px 58px minmax(0, 1fr);
  gap: 12px;
  padding: 2px 14px;
  color: #e2e8f0;
  white-space: pre-wrap;
  word-break: break-word;
}

.log-line:hover {
  background: rgba(148, 163, 184, 0.12);
}

.log-time {
  color: #94a3b8;
}

.log-level {
  text-transform: uppercase;
  font-weight: 700;
}

.level-trace {
  color: #93c5fd;
}

.level-debug {
  color: #67e8f9;
}

.level-info {
  color: #86efac;
}

.level-warn {
  color: #fde68a;
}

.level-error {
  color: #fca5a5;
}

.log-message {
  min-width: 0;
}

@media (max-width: 768px) {
  .page-header,
  .logs-toolbar {
    align-items: flex-start;
    flex-direction: column;
  }

  .page-actions {
    width: 100%;
  }

  .log-body {
    min-height: 360px;
  }

  .log-line {
    grid-template-columns: 76px 48px minmax(0, 1fr);
    gap: 8px;
    padding: 3px 10px;
    font-size: 11px;
  }
}
</style>
