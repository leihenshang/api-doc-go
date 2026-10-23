<script setup lang="ts">
// 接口/文档历史版本（RM-05）：版本列表 + 字段级 diff + 回滚。
// 复用于 ApiDetailView / DocDetailView，避免两套分叉逻辑。
import { getRevisionDiff, listRevisions, restoreRevision } from '@/api/content'
import { ErrCode } from '@/types/api'
import type { RevisionDiff, RevisionItem } from '@/types/entities'
import { fmtTime } from '@/utils/format'
import { NButton, NModal, NSpin, useDialog, useMessage } from 'naive-ui'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

const props = defineProps<{
  show: boolean
  type: 'api' | 'doc'
  id: number
}>()
const emit = defineEmits<{
  (e: 'update:show', value: boolean): void
  (e: 'restored'): void
}>()

const { t } = useI18n()
const message = useMessage()
const dialog = useDialog()

const open = computed({
  get: () => props.show,
  set: (v: boolean) => emit('update:show', v),
})

const loading = ref(false)
const revisions = ref<RevisionItem[]>([])
const revisionId = ref<number | null>(null)
const revisionDiff = ref<RevisionDiff | null>(null)
const diffLoading = ref(false)
const restoring = ref(false)

function actionLabel(action: string): string {
  if (action === 'create') return t('history.actionCreate')
  if (action === 'restore') return t('history.actionRestore')
  return t('history.actionUpdate')
}

/** 历史值/当前值统一转文本（对象/数组用 JSON，空值显示破折号）。 */
function diffText(v: unknown): string {
  if (v === null || v === undefined || v === '') return '—'
  if (typeof v === 'object') return JSON.stringify(v)
  return String(v)
}

async function selectRevision(id: number): Promise<void> {
  revisionId.value = id
  revisionDiff.value = null
  diffLoading.value = true
  try {
    const res = await getRevisionDiff(props.type, props.id, id)
    if (res.code === ErrCode.Success && res.data) {
      revisionDiff.value = res.data
    } else {
      message.error(res.msg || t('history.loadFailed'))
    }
  } finally {
    diffLoading.value = false
  }
}

async function load(): Promise<void> {
  loading.value = true
  revisions.value = []
  revisionDiff.value = null
  revisionId.value = null
  try {
    const res = await listRevisions(props.type, props.id)
    if (res.code !== ErrCode.Success || !res.data) {
      message.error(res.msg || t('history.loadFailed'))
      return
    }
    revisions.value = res.data
    // 默认看「上一个版本」，没有历史时看最新一条。
    const target = res.data.length > 1 ? res.data[1] : res.data[0]
    if (target) await selectRevision(target.id)
  } finally {
    loading.value = false
  }
}

function confirmRestore(): void {
  if (!revisionId.value) return
  const id = revisionId.value
  dialog.warning({
    title: t('history.restore'),
    content: t('history.restoreConfirm'),
    positiveText: t('common.confirm'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      restoring.value = true
      try {
        const res = await restoreRevision(props.type, props.id, id)
        if (res.code !== ErrCode.Success) {
          message.error(res.msg || t('history.restoreFailed'))
          return
        }
        message.success(t('history.restored'))
        open.value = false
        emit('restored')
      } finally {
        restoring.value = false
      }
    },
  })
}

watch(
  () => props.show,
  (v) => { if (v) void load() },
)
</script>

<template>
  <n-modal v-model:show="open" preset="card" :title="t('history.title')" class="history-modal" style="width: 860px">
    <n-spin :show="loading">
      <div class="history-body">
        <aside class="history-list">
          <button
            v-for="r in revisions"
            :key="r.id"
            class="history-item"
            :class="{ 'history-item--active': r.id === revisionId }"
            type="button"
            @click="selectRevision(r.id)"
          >
            <span class="history-time">{{ fmtTime(r.create_time) }}</span>
            <span class="history-meta">{{ actionLabel(r.action) }} · #{{ r.id }}</span>
          </button>
          <p v-if="!revisions.length" class="history-empty">{{ t('history.empty') }}</p>
        </aside>
        <section class="history-diff">
          <n-spin :show="diffLoading">
            <template v-if="revisionDiff">
              <div class="history-diff-head">
                {{ t('history.compare', { time: fmtTime(revisionDiff.revision.create_time) }) }}
              </div>
              <div v-if="revisionDiff.fields.length" class="history-table-wrap">
                <table class="history-table">
                  <thead>
                    <tr>
                      <th>{{ t('history.field') }}</th>
                      <th>{{ t('history.before') }}</th>
                      <th>{{ t('history.after') }}</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="f in revisionDiff.fields" :key="f.field">
                      <td class="history-field">{{ f.label }}</td>
                      <td class="history-value">{{ diffText(f.before) }}</td>
                      <td class="history-value history-value--after">{{ diffText(f.after) }}</td>
                    </tr>
                  </tbody>
                </table>
              </div>
              <p v-else class="history-empty">{{ t('history.noDiff') }}</p>
            </template>
            <p v-else-if="!diffLoading" class="history-empty">{{ t('history.select') }}</p>
          </n-spin>
        </section>
      </div>
    </n-spin>
    <template #footer>
      <div class="row-actions">
        <n-button size="small" @click="open = false">{{ t('common.close') }}</n-button>
        <n-button
          size="small"
          type="primary"
          :disabled="!revisionId"
          :loading="restoring"
          data-testid="history.restore"
          @click="confirmRestore"
        >
          {{ t('history.restore') }}
        </n-button>
      </div>
    </template>
  </n-modal>
</template>

<style scoped>
.history-body {
  display: flex;
  gap: 16px;
  min-height: 320px;
}

.history-list {
  flex: 0 0 220px;
  max-height: 56vh;
  overflow: auto;
  border-right: 1px solid var(--app-border);
  padding-right: 8px;
}

.history-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
  width: 100%;
  padding: 8px 10px;
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: inherit;
  text-align: left;
  cursor: pointer;
}

.history-item:hover,
.history-item--active {
  background: var(--app-bg);
}

.history-time {
  font-size: 13px;
}

.history-meta {
  color: var(--app-muted);
  font-size: 12px;
}

.history-diff {
  flex: 1 1 auto;
  min-width: 0;
}

.history-diff-head {
  margin-bottom: 10px;
  color: var(--app-muted);
  font-size: 12px;
}

.history-table-wrap {
  max-height: 48vh;
  overflow: auto;
}

.history-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 12px;
}

.history-table th,
.history-table td {
  padding: 6px 8px;
  border-bottom: 1px solid var(--app-border);
  text-align: left;
  vertical-align: top;
}

.history-field {
  width: 90px;
  color: var(--app-muted);
  white-space: nowrap;
}

.history-value {
  white-space: pre-wrap;
  word-break: break-word;
}

.history-value--after {
  color: var(--app-accent);
}

.history-empty {
  margin: 12px 0;
  color: var(--app-muted);
  font-size: 13px;
}

@media (max-width: 768px) {
  .history-body {
    flex-direction: column;
  }

  .history-list {
    flex: 0 0 auto;
    max-height: 120px;
    border-right: 0;
    border-bottom: 1px solid var(--app-border);
    padding-right: 0;
    padding-bottom: 8px;
  }
}
</style>
