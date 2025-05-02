<template>
  <div>
    <el-page-header
      :icon="null"
      style="width: 100%; margin-left: 30px; margin-bottom: 20px"
    >
      <template #title>
        <span>{{ proxyType }}</span>
      </template>
      <template #content> </template>
      <template #extra>
        <div class="flex items-center" style="margin-right: 30px">
          <el-button @click="$emit('refresh')">刷新</el-button>
        </div>
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
      <el-table-column label="隧道名称" prop="name" sortable> </el-table-column>
      <el-table-column label="端口" prop="port" sortable> </el-table-column>
      <el-table-column label="连接数" prop="conns" sortable>
      </el-table-column>
      <el-table-column
        label="入站流量"
        prop="trafficIn"
        :formatter="formatTrafficIn"
        sortable
      >
      </el-table-column>
      <el-table-column
        label="出站流量"
        prop="trafficOut"
        :formatter="formatTrafficOut"
        sortable
      >
      </el-table-column>
      <el-table-column label="客户端版本" prop="clientVersion" sortable>
      </el-table-column>
      <el-table-column label="状态" prop="status" sortable>
        <template #default="scope">
          <el-tag v-if="scope.row.status === 'online'" type="success">{{
            scope.row.status
          }}</el-tag>
          <el-tag v-else type="danger">{{ scope.row.status }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作">
        <template #default="scope">
          <el-dropdown @command="handleCommand($event, scope.row)">
            <el-button type="primary">
              操作 ▼
            </el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="traffic">查看流量</el-dropdown-item>
                <el-dropdown-item 
                  v-if="scope.row.status === 'online'"
                  command="close"
                  :disabled="scope.row.status !== 'online'">
                  关闭隧道
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </template>
      </el-table-column>
    </el-table>
  </div>

  <el-dialog
    v-model="dialogVisible"
    :destroy-on-close="true"
    :title="dialogVisibleName"
    width="700px">
    <Traffic :proxyName="dialogVisibleName" />
  </el-dialog>
  
  <!-- 确认关闭隧道对话框 -->
  <el-dialog
    v-model="closeDialogVisible"
    title="关闭隧道"
    width="400px">
    <span>确定要关闭隧道 "{{ selectedProxy?.name }}" 吗？</span>
    <template #footer>
      <span class="dialog-footer">
        <el-button @click="closeDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="confirmCloseProxy" :loading="closingProxy">
          确定
        </el-button>
      </span>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import * as Humanize from 'humanize-plus'
import type { TableColumnCtx } from 'element-plus'
import type { BaseProxy } from '../utils/proxy.js'
import { ElMessage } from 'element-plus'
import ProxyViewExpand from './ProxyViewExpand.vue'
import { ref } from 'vue'

defineProps<{
  proxies: BaseProxy[]
  proxyType: string
}>()

const emit = defineEmits(['refresh'])

const dialogVisible = ref(false)
const dialogVisibleName = ref("")
const closeDialogVisible = ref(false)
const selectedProxy = ref<BaseProxy | null>(null)
const closingProxy = ref(false)

const formatTrafficIn = (row: BaseProxy, _: TableColumnCtx<BaseProxy>) => {
  return Humanize.fileSize(row.trafficIn)
}

const formatTrafficOut = (row: BaseProxy, _: TableColumnCtx<BaseProxy>) => {
  return Humanize.fileSize(row.trafficOut)
}

const handleCommand = (command: string, row: BaseProxy) => {
  if (command === 'traffic') {
    dialogVisibleName.value = row.name
    dialogVisible.value = true
  } else if (command === 'close') {
    selectedProxy.value = row
    closeDialogVisible.value = true
  }
}

const confirmCloseProxy = () => {
  if (!selectedProxy.value) return
  
  closingProxy.value = true
  
  // 构建关闭请求数据
  const closeData = {
    runId: selectedProxy.value.runId
  }
  
  fetch('../api/client/kick', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    credentials: 'include',  // 包含凭据（cookies等）
    body: JSON.stringify(closeData)
  })
  .then(async response => {
    const data = await response.json()
    closingProxy.value = false
    closeDialogVisible.value = false
    
    if (data.status === 200) {
      // 先执行刷新
      await emit('refresh')
      
      ElMessage({
        message: '隧道关闭成功',
        type: 'success'
      })
    } else {
      ElMessage({
        message: '关闭隧道失败: ' + (data.message || '未知错误'),
        type: 'error'
      })
    }
  })
  .catch(error => {
    closingProxy.value = false
    closeDialogVisible.value = false
    ElMessage({
      message: '关闭隧道请求失败: ' + error.message,
      type: 'error'
    })
  })
}
</script>

<style>
.el-page-header__title {
  font-size: 20px;
}
</style>
