<template>
  <div>
    <el-page-header :icon="null" class="page-header">
      <template #title>
        <span>{{ proxyType.toUpperCase() }}</span>
      </template>
      <template #extra>
        <el-button @click="$emit('refresh')">刷新</el-button>
      </template>
    </el-page-header>

    <el-table
      :data="proxies"
      :default-sort="{ prop: 'name', order: 'ascending' }"
      style="width: 100%"
    >
      <el-table-column type="expand">
        <template #default="props">
          <ProxyViewExpand :row="props.row" :proxyType="proxyType" />
        </template>
      </el-table-column>
      <el-table-column label="代理名称" prop="name" sortable />
      <el-table-column label="端口" prop="port" sortable />
      <el-table-column label="连接数" prop="conns" sortable />
      <el-table-column
        label="入站流量"
        prop="trafficIn"
        :formatter="formatTrafficIn"
        sortable
      />
      <el-table-column
        label="出站流量"
        prop="trafficOut"
        :formatter="formatTrafficOut"
        sortable
      />
      <el-table-column label="客户端版本" prop="clientVersion" sortable />
      <el-table-column label="状态" prop="status" sortable>
        <template #default="scope">
          <el-tag v-if="scope.row.status === 'online'" type="success">
            online
          </el-tag>
          <el-tag v-else type="danger">offline</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作">
        <template #default="scope">
          <el-dropdown @command="handleCommand($event, scope.row)">
            <el-button type="primary">操作</el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="traffic">查看流量</el-dropdown-item>
                <el-dropdown-item
                  v-if="scope.row.status === 'online'"
                  command="close"
                >
                  踢下线
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </template>
      </el-table-column>
    </el-table>
  </div>

  <el-dialog
    v-model="trafficDialogVisible"
    :destroy-on-close="true"
    :title="trafficDialogProxyName"
    width="700px"
  >
    <Traffic :proxyName="trafficDialogProxyName" />
  </el-dialog>

  <el-dialog v-model="closeDialogVisible" title="踢客户端下线" width="400px">
    <span>确定要将代理 “{{ selectedProxy?.name }}” 对应的客户端踢下线吗？</span>
    <template #footer>
      <span class="dialog-footer">
        <el-button @click="closeDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="closingProxy" @click="confirmCloseProxy">
          确定
        </el-button>
      </span>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import * as Humanize from 'humanize-plus'
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import type { TableColumnCtx } from 'element-plus'
import type { BaseProxy } from '../utils/proxy'
import { kickClient } from '../api/client'
import ProxyViewExpand from './ProxyViewExpand.vue'
import Traffic from './Traffic.vue'

defineProps<{
  proxies: BaseProxy[]
  proxyType: string
}>()

const emit = defineEmits<{
  (event: 'refresh'): void
}>()

const trafficDialogVisible = ref(false)
const trafficDialogProxyName = ref('')
const closeDialogVisible = ref(false)
const selectedProxy = ref<BaseProxy | null>(null)
const closingProxy = ref(false)

const formatTrafficIn = (row: BaseProxy, _: TableColumnCtx<BaseProxy>) =>
  Humanize.fileSize(row.trafficIn)

const formatTrafficOut = (row: BaseProxy, _: TableColumnCtx<BaseProxy>) =>
  Humanize.fileSize(row.trafficOut)

const handleCommand = (command: string, row: BaseProxy) => {
  if (command === 'traffic') {
    trafficDialogProxyName.value = row.name
    trafficDialogVisible.value = true
    return
  }
  if (command === 'close') {
    selectedProxy.value = row
    closeDialogVisible.value = true
  }
}

const confirmCloseProxy = async () => {
  if (!selectedProxy.value?.runId) {
    closeDialogVisible.value = false
    ElMessage({
      message: '当前代理缺少可用的 runId，无法踢下线',
      type: 'warning',
    })
    return
  }

  closingProxy.value = true
  try {
    const response = await kickClient(selectedProxy.value.runId)
    closeDialogVisible.value = false

    if (response.status === 200) {
      emit('refresh')
      ElMessage({
        message: '客户端已踢下线',
        type: 'success',
      })
      return
    }

    ElMessage({
      message: `踢下线失败: ${response.message || '未知错误'}`,
      type: 'error',
    })
  } catch (error: any) {
    ElMessage({
      message: `踢下线请求失败: ${error.message}`,
      type: 'error',
    })
  } finally {
    closingProxy.value = false
  }
}
</script>

<style scoped>
.page-header {
  width: 100%;
  margin-left: 30px;
  margin-bottom: 20px;
}

:deep(.el-page-header__title) {
  font-size: 20px;
}
</style>
