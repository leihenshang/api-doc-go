<script setup lang="ts">
// 接口参数表（Postman Query 表样式）：请求头部 / 请求参数 / 返回参数 三种形态共用查看态与编辑态。
// 行数据统一为 ApiParam —— 请求头由 headersToParams 归一化（fieldName=头名、example=值）。
import type { ApiParam } from '@/types/entities'
import { jsonToParams, mergeParams } from '@/utils/json-params'
import { AddOutline, TrashOutline } from '@vicons/ionicons5'
import { NButton, NCheckbox, NIcon, NInput, NModal, NRadio, NRadioGroup, useMessage } from 'naive-ui'
import { computed, nextTick, ref } from 'vue'

type Kind = 'request' | 'response' | 'header'

/** 各形态的列配置：请求头无需类型与必填，返回参数无需必填。 */
const COLUMNS: Record<Kind, { title: string; name: string; value: string; withRequired: boolean; withType: boolean }> = {
  request: { title: 'Query 参数', name: '参数名', value: '示例', withRequired: true, withType: true },
  response: { title: '返回参数', name: '参数名', value: '示例', withRequired: false, withType: true },
  header: { title: '请求头部', name: '请求头名', value: '值', withRequired: false, withType: false },
}

const props = withDefaults(defineProps<{ kind: Kind; editable?: boolean; jsonSource?: string }>(), {
  editable: false,
  jsonSource: '',
})

const rows = defineModel<ApiParam[]>({ required: true })
const emit = defineEmits<{ (e: 'json-applied', text: string): void }>()

const message = useMessage()
const cfg = computed(() => COLUMNS[props.kind])
/** 已命名的行数，用于标题旁计数。 */
const filled = computed(() => rows.value.filter((r) => r.fieldName.trim()).length)
const showJson = ref(false)
const jsonText = ref('')
const jsonMode = ref<'merge' | 'replace'>('merge')
const parsed = computed(() => jsonToParams(jsonText.value))
const previewRows = computed(() => parsed.value.rows.slice(0, 8))

/** 新建行的输入框引用，用于添加后自动聚焦。 */
interface Focusable {
  focus: () => void
}
const nameRefs = ref<(Focusable | null)[]>([])

function bindNameInput(el: unknown, i: number): void {
  nameRefs.value[i] = el as Focusable | null
}

function add(): void {
  rows.value = [
    ...rows.value,
    { fieldName: '', type: props.kind === 'header' ? '' : 'string', required: false, description: '', example: '' },
  ]
  void nextTick(() => nameRefs.value[rows.value.length - 1]?.focus())
}

function remove(i: number): void {
  rows.value = rows.value.filter((_, idx) => idx !== i)
}

function setRequired(row: ApiParam, checked: boolean): void {
  row.required = checked
}

async function copyJson(): Promise<void> {
  const text = JSON.stringify(rows.value, null, 2)
  try {
    await navigator.clipboard.writeText(text)
    message.success('已复制 JSON')
  } catch {
    message.warning(text)
  }
}

function openJson(): void {
  jsonText.value = props.jsonSource || ''
  jsonMode.value = 'merge'
  showJson.value = true
}

function applyJson(): void {
  const { rows: generated, error } = parsed.value
  if (error) {
    message.error(error)
    return
  }
  if (!generated.length) {
    message.warning('未解析出字段，请确认 JSON 结构')
    return
  }
  rows.value = mergeParams(rows.value, generated, jsonMode.value === 'replace')
  emit('json-applied', jsonText.value)
  message.success(`已生成 ${generated.length} 个字段`)
  showJson.value = false
}
</script>

<template>
  <section class="pt">
    <div class="pt-bar">
      <span class="pt-title">{{ cfg.title }}<sup v-if="filled" class="pt-count">{{ filled }}</sup></span>
      <div class="pt-ops">
        <n-button v-if="editable" text size="tiny" @click="openJson">JSON 转表格</n-button>
        <n-button v-if="editable" text size="tiny" @click="add">
          <template #icon><n-icon :component="AddOutline" /></template>
          添加一行
        </n-button>
        <n-button v-if="rows.length" text size="tiny" @click="copyJson">复制 JSON</n-button>
      </div>
    </div>

    <p v-if="!rows.length && !editable" class="pt-empty meta">暂无{{ cfg.title }}</p>

    <div v-else class="pt-wrap">
      <table class="pt-table">
        <thead>
          <tr>
            <th v-if="cfg.withRequired" class="c-chk" />
            <th class="c-name">{{ cfg.name }}</th>
            <th v-if="cfg.withType" class="c-type">类型</th>
            <th class="c-value">{{ cfg.value }}</th>
            <th class="c-desc">说明</th>
            <th v-if="editable" class="c-del" />
          </tr>
        </thead>
        <tbody>
          <tr v-for="(row, i) in rows" :key="i">
            <td v-if="cfg.withRequired" class="c-chk">
              <n-checkbox
                :checked="row.required === true"
                :disabled="!editable"
                size="small"
                @update:checked="(v) => setRequired(row, v)"
              />
            </td>
            <td class="c-name">
              <n-input
                v-if="editable"
                :ref="(el) => bindNameInput(el, i)"
                v-model:value="row.fieldName"
                size="tiny"
                :placeholder="cfg.name"
              />
              <span v-else class="row-mono">{{ row.fieldName || '—' }}</span>
            </td>
            <td v-if="cfg.withType" class="c-type">
              <n-input v-if="editable" v-model:value="row.type" size="tiny" placeholder="string" />
              <span v-else class="muted">{{ row.type || '—' }}</span>
            </td>
            <td class="c-value">
              <n-input v-if="editable" v-model:value="row.example" size="tiny" />
              <span v-else class="row-mono muted">{{ row.example || '—' }}</span>
            </td>
            <td class="c-desc">
              <n-input v-if="editable" v-model:value="row.description" size="tiny" />
              <span v-else>{{ row.description || '—' }}</span>
            </td>
            <td v-if="editable" class="c-del">
              <n-button text size="tiny" class="pt-del" title="删除该行" @click="remove(i)">
                <template #icon><n-icon :component="TrashOutline" /></template>
              </n-button>
            </td>
          </tr>
          <tr v-if="editable" class="pt-add" @click="add">
            <td v-if="cfg.withRequired" class="c-chk" />
            <td class="c-name"><span class="ph">{{ cfg.name }}</span></td>
            <td v-if="cfg.withType" class="c-type"><span class="ph">string</span></td>
            <td class="c-value"><span class="ph">{{ cfg.value }}</span></td>
            <td class="c-desc"><span class="ph">说明</span></td>
            <td class="c-del" />
          </tr>
        </tbody>
      </table>
    </div>

    <n-modal v-model:show="showJson" preset="card" :title="`JSON 转${cfg.title}`" style="width: 620px">
      <p class="modal-hd meta">粘贴 JSON 或复用响应样例，自动展开为 data.list[].id 形式的参数行。</p>
      <n-input
        v-model:value="jsonText"
        type="textarea"
        :rows="10"
        placeholder='{"code":200,"data":{"total":1,"list":[{"id":21,"name":"demo"}]}}'
      />
      <div class="modal-ft">
        <n-radio-group v-model:value="jsonMode" size="small">
          <n-radio value="merge">追加并保留已填说明</n-radio>
          <n-radio value="replace">覆盖当前表格</n-radio>
        </n-radio-group>
        <span v-if="parsed.error" class="err">{{ parsed.error }}</span>
        <span v-else-if="parsed.rows.length" class="meta">解析出 {{ parsed.rows.length }} 个字段</span>
      </div>
      <ul v-if="previewRows.length" class="preview">
        <li v-for="r in previewRows" :key="r.fieldName">
          <span class="row-mono">{{ r.fieldName }}</span>
          <span class="meta">{{ r.type }}{{ r.example ? ` · ${r.example}` : '' }}</span>
        </li>
        <li v-if="parsed.rows.length > previewRows.length" class="meta">
          …另有 {{ parsed.rows.length - previewRows.length }} 个字段
        </li>
      </ul>
      <template #footer>
        <div class="foot">
          <n-button size="small" @click="showJson = false">取消</n-button>
          <n-button type="primary" size="small" @click="applyJson">生成</n-button>
        </div>
      </template>
    </n-modal>
  </section>
</template>

<style scoped>
.pt {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.pt-bar {
  display: flex;
  align-items: center;
  gap: 12px;
}

.pt-title {
  font-size: 13px;
  font-weight: 600;
}

.pt-count {
  margin-left: 4px;
  color: var(--app-muted);
  font-size: 10px;
  font-weight: 500;
}

.pt-ops {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 4px;
}

.pt-empty {
  margin: 0;
  padding: 18px 0;
  text-align: center;
  border: 1px dashed var(--app-border);
  border-radius: 4px;
}

.pt-wrap {
  border: 1px solid var(--app-border);
  border-radius: 4px;
  overflow: hidden;
}

.pt-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
  table-layout: fixed;
}

.pt-table th,
.pt-table td {
  padding: 5px 10px;
  text-align: left;
  border-right: 1px solid var(--app-border);
  border-bottom: 1px solid var(--app-border);
  vertical-align: middle;
}

.pt-table th {
  padding: 7px 10px;
  background: #f7f8fa;
  color: var(--app-muted);
  font-size: 12px;
  font-weight: 500;
}

.pt-table th:last-child,
.pt-table td:last-child {
  border-right: 0;
}

.pt-table tbody tr:last-child td {
  border-bottom: 0;
}

.pt-table tbody tr:hover {
  background: #fafbfc;
}

.c-chk {
  width: 34px;
  padding-left: 8px;
  padding-right: 0;
  text-align: center;
}

.c-name {
  width: 26%;
}

.c-type {
  width: 108px;
}

.c-desc {
  width: 28%;
}

.c-del {
  width: 34px;
  padding-left: 0;
  padding-right: 6px;
  text-align: center;
}

.pt-del {
  color: var(--app-muted);
}

.pt-del:hover {
  color: var(--app-danger, #eb2013);
}

.pt-add {
  cursor: pointer;
}

.ph {
  color: #c2c8ce;
  font-size: 12px;
}

/* ---- JSON 转表格弹窗 ---- */
.modal-hd {
  margin: 0 0 8px;
}

.modal-ft {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-top: 12px;
}

.err {
  font-size: 12px;
  color: #d03050;
}

.preview {
  list-style: none;
  margin: 12px 0 0;
  padding: 10px 12px;
  font-size: 12px;
  background: #fafbfc;
  border: 1px solid var(--app-border);
  border-radius: 4px;
}

.preview li {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  padding: 2px 0;
}

.foot {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
</style>
