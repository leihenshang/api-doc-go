<script setup lang="ts">
// 通知铃铛：未读红点 + 悬停展开最近通知，支持标记已读与跳转全部列表。
import { listNotifications, markNotificationRead } from '@/api/misc'
import { ErrCode } from '@/types/api'
import type { Notification } from '@/types/entities'
import { fmtTime } from '@/utils/format'
import { NotificationsOutline } from '@vicons/ionicons5'
import { NBadge, NButton, NEmpty, NIcon, NPopover, NSpin, useMessage } from 'naive-ui'
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'

const router = useRouter()
const message = useMessage()
const { t } = useI18n()

const items = ref<Notification[]>([])
const loading = ref(false)
const unread = computed(() => items.value.filter((n) => n.read !== 1).length)

async function reload(): Promise<void> {
  loading.value = true
  try {
    const res = await listNotifications({ page_size: 30 })
    items.value = res.code === ErrCode.Success && res.data ? res.data.items : []
  } finally {
    loading.value = false
  }
}

async function read(id: number): Promise<void> {
  const res = await markNotificationRead(id)
  if (res.code !== ErrCode.Success) message.error(res.msg || t('common.opFailed'))
  await reload()
}

function goAll(): void {
  void router.push({ name: 'notifications' })
}

onMounted(reload)
</script>

<template>
  <n-popover trigger="hover" placement="bottom-end" :width="380" @update:show="(v: boolean) => v && reload()">
    <template #trigger>
      <span class="bell" role="button" tabindex="0" :aria-label="t('notice.title')" :title="t('notice.title')" @keyup.enter="goAll">
        <n-badge :value="unread" :max="99" :show="unread > 0" :offset="[-2, 2]">
          <n-icon :component="NotificationsOutline" :size="20" />
        </n-badge>
      </span>
    </template>

    <div class="panel">
      <div class="hd">
        <span class="hd-title">{{ t('notice.title') }}</span>
        <span class="meta">{{ t('notice.unread', { n: unread }) }}</span>
      </div>

      <n-spin :show="loading">
        <n-empty v-if="!items.length && !loading" size="small" class="empty" :description="t('notice.empty')" />
        <ul v-else class="list">
          <li v-for="n in items" :key="n.id" :class="{ unread: n.read !== 1 }">
            <div class="row">
              <span class="title ellipsis" :title="n.title">{{ n.title }}</span>
              <span class="meta">{{ fmtTime(n.create_time) }}</span>
            </div>
            <div class="body ellipsis" :title="n.content">{{ n.content }}</div>
            <n-button v-if="n.read !== 1" text type="primary" size="tiny" @click="read(n.id)">{{ t('notice.markRead') }}</n-button>
          </li>
        </ul>
      </n-spin>

      <div class="ft">
        <n-button text type="primary" size="small" @click="goAll">{{ t('notice.viewAll') }}</n-button>
      </div>
    </div>
  </n-popover>
</template>

<style scoped>
.bell {
  display: inline-flex;
  align-items: center;
  color: var(--app-muted);
  cursor: pointer;
}

.bell:hover {
  color: var(--app-accent);
}

.hd {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding-bottom: 8px;
  border-bottom: 1px solid var(--app-border);
}

.hd-title {
  font-size: 14px;
  font-weight: 600;
}

.list {
  list-style: none;
  margin: 0;
  padding: 0;
  max-height: 320px;
  overflow: auto;
}

.list li {
  padding: 8px 0;
  border-bottom: 1px solid var(--app-border);
}

.list li:last-child {
  border-bottom: 0;
}

.row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.title {
  font-size: 13px;
  font-weight: 500;
}

.unread .title {
  color: var(--app-accent);
}

.body {
  margin-top: 2px;
  font-size: 12px;
  color: var(--app-muted);
}

.empty {
  padding: 16px 0;
}

.ft {
  margin-top: 8px;
  text-align: right;
}
</style>
