<script setup lang="ts">
// 契约校验（RM-08）：上传/读取 OpenAPI 3.x，与当前项目接口做只读 diff。
// 报告只读展示，不修改文档；破坏性变更标红，供导入前确认。
import { contractDiff } from '@/api/content'
import { ErrCode } from '@/types/api'
import type { ContractDiffResult } from '@/types/entities'
import { parseOpenAPIText } from '@/utils/openapi'
import { NButton, NModal, NSpin, NTag, NUpload, useMessage, type UploadFileInfo } from 'naive-ui'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

const props = defineProps<{
  show: boolean
  projectId: number
}>()
const emit = defineEmits<{ (e: 'update:show', value: boolean): void }>()

const { t } = useI18n()
const message = useMessage()

const open = computed({
  get: () => props.show,
  set: (v: boolean) => emit('update:show', v),
})

const fileName = ref('')
const spec = ref<unknown>(null)
const checking = ref(false)
const result = ref<ContractDiffResult | null>(null)

watch(
  () => props.show,
  (v) => {
    if (!v) return
    fileName.value = ''
    spec.value = null
    result.value = null
  },
)

async function onFile(list: UploadFileInfo[]): Promise<void> {
  const file = list[0]?.file
  if (!file) return
  fileName.value = file.name
  result.value = null
  try {
    spec.value = parseOpenAPIText(await file.text())
  } catch {
    spec.value = null
    message.error(t('contract.parseFailed'))
  }
}

async function check(): Promise<void> {
  if (!spec.value) {
    message.warning(t('contract.selectFile'))
    return
  }
  checking.value = true
  try {
    const res = await contractDiff(props.projectId, spec.value)
    if (res.code !== ErrCode.Success || !res.data) {
      message.error(res.msg || t('contract.checkFailed'))
      return
    }
    result.value = res.data
  } finally {
    checking.value = false
  }
}

function kindLabel(kind: string): string {
  const map: Record<string, string> = {
    param_added: t('contract.kindAdded'),
    param_removed: t('contract.kindRemoved'),
    required_changed: t('contract.kindRequired'),
    type_changed: t('contract.kindType'),
  }
  return map[kind] || kind
}
</script>

<template>
  <n-modal v-model:show="open" preset="card" :title="t('contract.title')" style="width: 820px">
    <div class="contract-hd">
      <n-upload :max="1" :default-upload="false" accept=".json,.yaml,.yml" @update:file-list="onFile">
        <n-button size="small">{{ t('contract.chooseFile') }}</n-button>
      </n-upload>
      <span class="contract-file">{{ fileName || t('contract.noFile') }}</span>
      <n-button size="small" type="primary" :disabled="!spec" :loading="checking" data-testid="contract.check" @click="check">
        {{ t('contract.check') }}
      </n-button>
    </div>

    <n-spin :show="checking">
      <template v-if="result">
        <div class="contract-summary">
          <n-tag v-if="result.breaking_count > 0" type="error" round>
            {{ t('contract.breaking', { n: result.breaking_count }) }}
          </n-tag>
          <n-tag v-else type="success" round>{{ t('contract.noBreaking') }}</n-tag>
          <span class="contract-meta">{{ t('contract.added', { n: result.added.length }) }}</span>
          <span class="contract-meta">{{ t('contract.removed', { n: result.removed.length }) }}</span>
          <span class="contract-meta">{{ t('contract.changed', { n: result.changes.length }) }}</span>
        </div>

        <section v-if="result.removed.length" class="contract-sec">
          <h4 class="contract-sub">{{ t('contract.removedEndpoints') }}</h4>
          <ul class="contract-list">
            <li v-for="e in result.removed" :key="'r-' + e.method + e.path">
              <span class="contract-method">{{ e.method }}</span> <span class="contract-path">{{ e.path }}</span>
            </li>
          </ul>
        </section>

        <section v-if="result.added.length" class="contract-sec">
          <h4 class="contract-sub">{{ t('contract.addedEndpoints') }}</h4>
          <ul class="contract-list">
            <li v-for="e in result.added" :key="'a-' + e.method + e.path">
              <span class="contract-method">{{ e.method }}</span> <span class="contract-path">{{ e.path }}</span>
            </li>
          </ul>
        </section>

        <section v-if="result.changes.length" class="contract-sec">
          <h4 class="contract-sub">{{ t('contract.changes') }}</h4>
          <div class="contract-table-wrap">
            <table class="contract-table">
              <thead>
                <tr>
                  <th>{{ t('contract.endpoint') }}</th>
                  <th>{{ t('contract.param') }}</th>
                  <th>{{ t('contract.kind') }}</th>
                  <th>{{ t('contract.before') }}</th>
                  <th>{{ t('contract.after') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(c, i) in result.changes" :key="i" :class="{ 'contract-breaking': c.breaking }">
                  <td>{{ c.method }} {{ c.path }}</td>
                  <td>{{ c.param }}<span v-if="c.in" class="contract-in"> · {{ c.in }}</span></td>
                  <td>
                    {{ kindLabel(c.kind) }}
                    <n-tag v-if="c.breaking" size="small" type="error" round>{{ t('contract.breakingTag') }}</n-tag>
                  </td>
                  <td>{{ c.before || '—' }}</td>
                  <td>{{ c.after || '—' }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>

        <p v-if="!result.added.length && !result.removed.length && !result.changes.length" class="contract-empty">
          {{ t('contract.same') }}
        </p>
      </template>
      <p v-else-if="!checking" class="contract-empty">{{ t('contract.hint') }}</p>
    </n-spin>

    <template #footer>
      <div class="row-actions">
        <n-button size="small" @click="open = false">{{ t('common.close') }}</n-button>
      </div>
    </template>
  </n-modal>
</template>

<style scoped>
.contract-hd {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  margin-bottom: 14px;
}

.contract-file {
  flex: 1 1 auto;
  min-width: 160px;
  color: var(--app-muted);
  font-size: 12px;
  word-break: break-all;
}

.contract-summary {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  margin-bottom: 14px;
}

.contract-meta {
  color: var(--app-muted);
  font-size: 12px;
}

.contract-sec {
  margin-bottom: 14px;
}

.contract-sub {
  margin: 0 0 6px;
  font-size: 13px;
}

.contract-list {
  margin: 0;
  padding-left: 18px;
  font-size: 12px;
  line-height: 1.8;
}

.contract-method {
  display: inline-block;
  min-width: 52px;
  color: var(--app-accent);
  font-weight: 600;
}

.contract-path {
  word-break: break-all;
}

.contract-table-wrap {
  max-height: 42vh;
  overflow: auto;
}

.contract-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 12px;
}

.contract-table th,
.contract-table td {
  padding: 6px 8px;
  border-bottom: 1px solid var(--app-border);
  text-align: left;
  vertical-align: top;
}

.contract-table th {
  position: sticky;
  top: 0;
  background: var(--app-panel);
}

.contract-breaking {
  background: rgba(208, 48, 80, 0.06);
}

.contract-in {
  color: var(--app-muted);
}

.contract-empty {
  margin: 12px 0;
  color: var(--app-muted);
  font-size: 13px;
}
</style>
