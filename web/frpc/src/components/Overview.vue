<template>
  <div>
    <n-card title="隧道状态" class="card">
      <n-data-table
        :columns="columns"
        :data="status"
        :pagination="pagination"
        :bordered="false"
        :loading="loading"
        striped
      />
    </n-card>
  </div>
</template>

<script setup lang="ts">
import { ref, h, onMounted } from 'vue'
import { NCard, NDataTable, NTag, NSpace, NButton, NIcon, useMessage, NPopconfirm } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'

interface StatusItem {
  name: string
  type: string
  local_addr: string
  plugin: string
  remote_addr: string
  status: string
  err: string
  run_id?: string
}

const message = useMessage()
const status = ref<StatusItem[]>([])
const loading = ref(true)
const pagination = {
  pageSize: 10
}

// 关闭隧道
const closeProxy = (name: string, type: string) => {
  if (!name) {
    message.error('无法关闭隧道：隧道名称为空')
    return
  }
  
  // 先获取隧道详情以获取runId
  fetch(`/api/proxy/${type}/${name}`, { credentials: 'include' })
    .then(res => res.json())
    .then(data => {
      const runId = data.runId
      if (!runId) {
        message.error('无法关闭隧道：未找到对应的运行ID')
        return
      }
      
      // 使用runId关闭隧道
      fetch('/api/client/kick', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ runId }),
        credentials: 'include'
      })
        .then(res => res.json())
        .then(data => {
          if (data.status === 200) {
            message.success('隧道已关闭')
            // 刷新数据
            fetchData()
          } else {
            message.error(`关闭隧道失败: ${data.message}`)
          }
        })
        .catch(err => {
          message.error(`关闭隧道请求失败: ${err}`)
        })
    })
    .catch(err => {
      message.error(`获取隧道信息失败: ${err}`)
    })
}

// 数据表格列定义
const columns: DataTableColumns<StatusItem> = [
  {
    title: '名称',
    key: 'name',
    sorter: 'default'
  },
  {
    title: '类型',
    key: 'type',
    sorter: 'default',
    width: 100,
    render(row) {
      return h(
        NTag,
        {
          type: 'info',
          bordered: false
        },
        { default: () => row.type }
      )
    }
  },
  {
    title: '本地地址',
    key: 'local_addr',
    width: 150
  },
  {
    title: '插件',
    key: 'plugin',
    width: 120
  },
  {
    title: '远程地址',
    key: 'remote_addr',
    width: 180
  },
  {
    title: '状态',
    key: 'status',
    width: 100,
    render(row) {
      const type = row.status === 'running' ? 'success' : 'error'
      const text = row.status === 'running' ? '运行中' : '已停止'
      return h(
        NTag,
        {
          type,
          bordered: false
        },
        { default: () => text }
      )
    }
  },
  {
    title: '信息',
    key: 'err',
    ellipsis: {
      tooltip: true
    }
  },
  {
    title: '操作',
    key: 'actions',
    width: 150,
    render(row) {
      return h(
        NSpace,
        { align: 'center' },
        {
          default: () => [
            h(
              NButton,
              {
                text: true,
                type: 'primary',
                onClick: fetchData
              },
              { default: () => '刷新' }
            ),
            row.status === 'running' ? h(
              NPopconfirm,
              {
                onPositiveClick: () => closeProxy(row.name, row.type),
              },
              {
                default: () => '确定要关闭该隧道吗？',
                trigger: () => h(
                  NButton,
                  {
                    text: true,
                    type: 'error',
                    disabled: row.status !== 'running'
                  },
                  { default: () => '关闭' }
                )
              }
            ) : null
          ]
        }
      )
    }
  }
]

// 获取数据
const fetchData = () => {
  loading.value = true
  fetch('/api/status', { credentials: 'include' })
    .then((res) => {
      return res.json()
    })
    .then((json) => {
      status.value = []
      for (let key in json) {
        for (let ps of json[key]) {
          status.value.push(ps)
        }
      }
      loading.value = false
    })
    .catch((err) => {
      message.error('获取隧道状态信息失败：' + err)
      loading.value = false
    })
}

onMounted(() => {
  fetchData()
})
</script>

<style scoped>
.card {
  margin-bottom: 16px;
}
</style>
