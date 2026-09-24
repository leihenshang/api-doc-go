<script setup lang="ts">
// 我的通知：列表与标记已读。
import { listNotifications, markNotificationRead } from '@/api/misc'
import TablePager from '@/components/TablePager.vue'
import { ErrCode } from '@/types/api'
import type { Notification } from '@/types/entities'
import { fmtTime } from '@/utils/format'
import { NButton, NCard, NEmpty, NSpin, NTable, NTag, useMessage } from 'naive-ui'
import { onMounted, ref } from 'vue'

const message = useMessage()
const rows = ref<Notification[]>([])
const total = ref(0)
const pageNum = ref(1)
const pageSize = ref(20)
const loading = ref(false)

async function load(): Promise<void> {
  loading.value = true
  try {
    const res = await listNotifications({ page_num: pageNum.value, page_size: pageSize.value })
    if (res.code === ErrCode.Success && res.data) {
      rows.value = res.data.items
      total.value = res.data.total
    }
  } finally {
    loading.value = false
  }
}

function reload(num = 1): void {
  pageNum.value = num
  void load()
}

async function read(id: number): Promise<void> {
  const res = await markNotificationRead(id)
  if (res.code !== ErrCode.Success) {
    message.error(res.msg)
    return
  }
  await load()
}

const sendType: Record<number, string> = { 1: '邮件', 2: '短信', 3: '站内' }

onMounted(load)
</script>

<template>
  <div class="page">
    <div class="page-hd">
      <div>
        <h1 class="page-title">我的通知</h1>
        <p class="page-sub">共 {{ total }} 条消息</p>
      </div>
    </div>

    <n-card :bordered="true" class="table-card" content-style="padding: 0">
      <n-spin :show="loading">
        <n-empty v-if="!rows.length && !loading" class="empty-block" description="暂无通知" />
        <n-table v-else :single-line="false" size="medium">
          <thead>
            <tr>
              <th style="width: 170px">时间</th>
              <th style="width: 24%">标题</th>
              <th>内容</th>
              <th style="width: 90px">渠道</th>
              <th style="width: 80px">状态</th>
              <th style="width: 100px">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="n in rows" :key="n.id" :class="{ unread: n.read !== 1 }">
              <td class="muted">{{ fmtTime(n.create_time) }}</td>
              <td>{{ n.title }}</td>
              <td class="ellipsis" :title="n.content">{{ n.content }}</td>
              <td class="muted">{{ sendType[n.send_type] ?? n.send_type }}</td>
              <td>
                <n-tag :type="n.read === 1 ? 'default' : 'info'" size="small" :bordered="false">
                  {{ n.read === 1 ? '已读' : '未读' }}
                </n-tag>
              </td>
              <td>
                <n-button v-if="n.read !== 1" text type="primary" size="small" @click="read(n.id)">标记已读</n-button>
              </td>
            </tr>
          </tbody>
        </n-table>
      </n-spin>
      <table-pager :total="total" :page-num="pageNum" :page-size="pageSize" @update:page-num="reload" />
    </n-card>
  </div>
</template>

<style scoped>
.unread td {
  font-weight: 500;
}
</style>
