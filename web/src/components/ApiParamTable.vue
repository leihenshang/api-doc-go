<script setup lang="ts">
// 接口参数表（Postman Query 表样式）：请求头部 / 请求参数 / 返回参数 三种形态共用查看态与编辑态。
// 行数据统一为 ApiParam —— 请求头由 headersToParams 归一化（fieldName=头名、example=值）。
// 类型列为 JSON 类型下拉（可自定义），对象/数组行可「加子字段」表达嵌套（data.list[].id）。
import VarText from '@/components/VarText.vue'
import type { ApiParam } from '@/types/entities'
import { jsonToParams, mergeParams } from '@/utils/json-params'
import { AddOutline, GitBranchOutline, TrashOutline } from '@vicons/ionicons5'
import { NButton, NCheckbox, NIcon, NInput, NModal, NRadio, NRadioGroup, NSelect, useMessage } from 'naive-ui'
import { computed, nextTick, ref } from 'vue'
import { useI18n } from 'vue-i18n'

type Kind = 'request' | 'response' | 'header'

/**
 * 各形态的列配置：请求头无需类型与必填，返回参数无需必填；color 用于计数圆标。
 * 文案存 labelKey，切语言时由 cfg 重新解析。
 */
const COLUMNS: Record<Kind, { titleKey: string; nameKey: string; valueKey: string; color: string; withRequired: boolean; withType: boolean }> = {
  request: { titleKey: 'param.requestTitle', nameKey: 'param.name', valueKey: 'param.example', color: '#0cbb52', withRequired: true, withType: true },
  response: { titleKey: 'param.responseTitle', nameKey: 'param.name', valueKey: 'param.example', color: '#8e44ad', withRequired: false, withType: true },
  header: { titleKey: 'param.headerTitle', nameKey: 'param.headerName', valueKey: 'param.value', color: '#097bed', withRequired: false, withType: false },
}

/** JSON 对应的类型枚举（filterable + tag，允许填自定义类型）。 */
const TYPE_OPTIONS = ['string', 'integer', 'number', 'boolean', 'object', 'array', 'null'].map((v) => ({ label: v, value: v }))

const props = withDefaults(
  defineProps<{
    kind: Kind
    editable?: boolean
    jsonSource?: string
    titleExtra?: string
    /** 查看态「值 / 示例」列的变量上下文（提供后 {{var}} 就地高亮）。 */
    projectId?: number
    apiId?: number
  }>(),
  { editable: false, jsonSource: '', titleExtra: '', projectId: 0, apiId: 0 },
)

const rows = defineModel<ApiParam[]>({ required: true })
const emit = defineEmits<{
    (e: 'json-applied', text: string): void
    /** 空态里的「编辑」：只请求父级切到编辑态，不直接改数据（避免用户只是看一眼被塞了空行）。 */
    (e: 'edit'): void
}>()

const message = useMessage()
const { t } = useI18n()
/** 当前形态的列配置（文案按当前语言解析）。 */
const cfg = computed(() => {
  const c = COLUMNS[props.kind]
  return { ...c, title: t(c.titleKey), name: t(c.nameKey), value: t(c.valueKey) }
})
/** 已命名的行数，用于标题旁计数圆标。 */
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

function writeRows(next: ApiParam[]): void {
  rows.value = next
}

/** 对象/数组行可在其后追加子字段（用点号与 `[]` 表达嵌套层级）。 */
function canAddChild(row: ApiParam): boolean {
  return props.editable && props.kind !== 'header' && (row.type === 'object' || row.type === 'array')
}

function add(): void {
  writeRows([
    ...rows.value,
    { fieldName: '', type: props.kind === 'header' ? '' : 'string', required: false, description: '', example: '' },
  ])
  void nextTick(() => nameRefs.value[rows.value.length - 1]?.focus())
}

function addChild(i: number): void {
  const parent = rows.value[i]
  const prefix = parent.type === 'array' ? `${parent.fieldName}[]` : parent.fieldName
  const next = [...rows.value]
  next.splice(i + 1, 0, { fieldName: `${prefix}.`, type: 'string', required: false, description: '', example: '' })
  writeRows(next)
  void nextTick(() => nameRefs.value[i + 1]?.focus())
}

function remove(i: number): void {
  writeRows(rows.value.filter((_, idx) => idx !== i))
}

function setRequired(row: ApiParam, checked: boolean): void {
  row.required = checked
}

async function copyJson(): Promise<void> {
  const text = JSON.stringify(rows.value, null, 2)
  try {
    await navigator.clipboard.writeText(text)
    message.success(t('param.jsonCopied'))
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
    message.warning(t('param.jsonInvalid'))
    return
  }
  writeRows(mergeParams(rows.value, generated, jsonMode.value === 'replace'))
  emit('json-applied', jsonText.value)
  message.success(t('param.jsonGenerated', { n: generated.length }))
  showJson.value = false
}
</script>

<template>
  <section class="pt">
    <div class="pt-bar">
      <span class="pt-title">
        {{ cfg.title }}
        <span
          v-if="filled"
          class="pt-count"
          :style="{ color: cfg.color, borderColor: cfg.color, background: `${cfg.color}1a` }"
        >{{ filled }}</span>
        <span v-if="titleExtra" class="pt-extra">{{ titleExtra }}</span>
      </span>
      <div class="pt-ops">
        <n-button v-if="editable" text size="tiny" @click="openJson">{{ t('param.jsonToTable') }}</n-button>
        <n-button v-if="editable" text size="tiny" @click="add">
          <template #icon><n-icon :component="AddOutline" /></template>
          {{ t('param.addRow') }}
        </n-button>
        <n-button v-if="rows.length" text size="tiny" @click="copyJson">{{ t('param.copyJson') }}</n-button>
      </div>
    </div>

    <div v-if="!rows.length && !editable" class="pt-empty">
      <span class="meta">{{ t('param.empty', { title: cfg.title }) }}</span>
      <n-button text size="tiny" type="primary" data-testid="param.emptyEdit" @click="emit('edit')">
        {{ t('common.edit') }}
      </n-button>
    </div>

    <div v-else class="pt-wrap">
      <table class="pt-table">
        <thead>
          <tr>
            <th v-if="cfg.withRequired" class="c-chk" />
            <th class="c-name">{{ cfg.name }}</th>
            <th v-if="cfg.withType" class="c-type">{{ t('param.type') }}</th>
            <th class="c-value">{{ cfg.value }}</th>
            <th class="c-desc">{{ t('param.desc') }}</th>
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
              <n-select
                v-if="editable"
                v-model:value="row.type"
                :options="TYPE_OPTIONS"
                size="tiny"
                filterable
                tag
                :placeholder="t('param.type')"
              />
              <span v-else class="muted">{{ row.type || '—' }}</span>
            </td>
            <td class="c-value">
              <n-input v-if="editable" v-model:value="row.example" size="tiny" />
              <span v-else class="row-mono muted">
                <var-text
                  v-if="row.example"
                  :text="row.example"
                  :project-id="props.projectId"
                  :api-id="props.apiId"
                />
                <template v-else>—</template>
              </span>
            </td>
            <td class="c-desc">
              <n-input v-if="editable" v-model:value="row.description" size="tiny" />
              <span v-else>{{ row.description || '—' }}</span>
            </td>
            <td v-if="editable" class="c-del">
              <span class="pt-row-ops">
                <n-button
                  v-if="canAddChild(row)"
                  text
                  size="tiny"
                  class="pt-child"
                  :title="t('param.addChild')"
                  @click="addChild(i)"
                >
                  <template #icon><n-icon :component="GitBranchOutline" /></template>
                </n-button>
                <n-button text size="tiny" class="pt-del" :title="t('param.delRow')" @click="remove(i)">
                  <template #icon><n-icon :component="TrashOutline" /></template>
                </n-button>
              </span>
            </td>
          </tr>
          <tr v-if="editable" class="pt-add" @click="add">
            <td v-if="cfg.withRequired" class="c-chk" />
            <td class="c-name"><span class="ph">{{ cfg.name }}</span></td>
            <td v-if="cfg.withType" class="c-type"><span class="ph">string</span></td>
            <td class="c-value"><span class="ph">{{ cfg.value }}</span></td>
            <td class="c-desc"><span class="ph">{{ t('param.desc') }}</span></td>
            <td class="c-del" />
          </tr>
        </tbody>
      </table>
      <p v-if="editable && cfg.withType" class="pt-hint meta">
        {{ t('param.typeTipBefore') }}<code>data.list[].id</code>{{ t('param.typeTipAfter') }}
      </p>
    </div>

    <n-modal v-model:show="showJson" preset="card" :title="t('param.jsonTitle', { title: cfg.title })" style="width: 620px">
      <p class="modal-hd meta">{{ t('param.jsonHint') }}</p>
      <n-input
        v-model:value="jsonText"
        type="textarea"
        :rows="10"
        placeholder='{"code":200,"data":{"total":1,"list":[{"id":21,"name":"demo"}]}}'
      />
      <div class="modal-ft">
        <n-radio-group v-model:value="jsonMode" size="small">
          <n-radio value="merge">{{ t('param.mergeMode') }}</n-radio>
          <n-radio value="replace">{{ t('param.replaceMode') }}</n-radio>
        </n-radio-group>
        <span v-if="parsed.error" class="err">{{ parsed.error }}</span>
        <span v-else-if="parsed.rows.length" class="meta">{{ t('param.jsonParsed', { n: parsed.rows.length }) }}</span>
      </div>
      <ul v-if="previewRows.length" class="preview">
        <li v-for="r in previewRows" :key="r.fieldName">
          <span class="row-mono">{{ r.fieldName }}</span>
          <span class="meta">{{ r.type }}{{ r.example ? ` · ${r.example}` : '' }}</span>
        </li>
        <li v-if="parsed.rows.length > previewRows.length" class="meta">
          {{ t('param.jsonMore', { n: parsed.rows.length - previewRows.length }) }}
        </li>
      </ul>
      <template #footer>
        <div class="foot">
          <n-button size="small" @click="showJson = false">{{ t('common.cancel') }}</n-button>
          <n-button type="primary" size="small" @click="applyJson">{{ t('param.generate') }}</n-button>
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
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  font-weight: 600;
}

/* 计数：圆圈 + 居中数字，颜色随所属模块 */
.pt-count {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 17px;
  height: 17px;
  padding: 0 4px;
  border: 1px solid;
  border-radius: 999px;
  font-size: 10px;
  font-weight: 600;
  line-height: 1;
}

/* 段标题旁的补充徽标（如返回参数旁的返回类型） */
.pt-extra {
  padding: 0 6px;
  border: 1px solid var(--app-border);
  border-radius: 3px;
  background: #f5f6f8;
  color: var(--app-muted);
  font-size: 11px;
  font-weight: 500;
  line-height: 16px;
}

.pt-ops {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 4px;
}

.pt-empty {
  /* 空态压到一行高：三个「暂无」区块各占 100px 时，新建接口的首屏几乎是空白（UI-08）。 */
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  margin: 0;
  padding: 10px 0;
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

/* 表格下方的类型/嵌套说明条 */
.pt-hint {
  margin: 0;
  padding: 6px 10px;
  border-top: 1px solid var(--app-border);
  background: #fafbfc;
}

.pt-hint code {
  padding: 0 3px;
  border-radius: 3px;
  background: #eef0f3;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
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
  width: 56px;
  padding-left: 0;
  padding-right: 6px;
  text-align: center;
}

.pt-row-ops {
  display: inline-flex;
  align-items: center;
  gap: 2px;
}

.pt-del {
  color: var(--app-muted);
}

.pt-del:hover {
  color: #eb2013;
}

.pt-child {
  color: var(--app-muted);
}

.pt-child:hover {
  color: var(--app-accent);
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
