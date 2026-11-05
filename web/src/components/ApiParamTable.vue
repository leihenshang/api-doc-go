<script setup lang="ts">
// 接口参数表（Postman Query 表样式）：请求头部 / 请求参数 / 返回参数 三种形态共用查看态与编辑态。
// 行数据统一为 ApiParam —— 请求头由 headersToParams 归一化（fieldName=头名、example=值）。
//
// 与客户端对齐的部分（见 doc/服务端API页对齐客户端实施清单.md §四）：
// - div + grid 列宽（不随内容跳动）、sticky 表头、行 hover、行尾常显操作列；
// - 请求参数 / 请求头有「启用」勾选（缺省启用，false 才停用），返回参数没有（客户端响应字段表本就无该位）；
// - 编辑态末行一有输入就补一行空行；空行在保存前由 cleanParams 清掉，不会写进服务端。
// 请求参数比客户端 query 表多「类型 / 必选」两列：Web 参数表承担文档语义，服务端行契约本身就有这两列。
import VarText from '@/components/VarText.vue'
import type { ApiParam } from '@/types/entities'
import { rowEnabled } from '@/utils/api-params'
import { jsonToParams, mergeParams } from '@/utils/json-params'
import { AddOutline, GitBranchOutline, TrashOutline } from '@vicons/ionicons5'
import { NButton, NCheckbox, NIcon, NInput, NModal, NRadio, NRadioGroup, NSelect, useMessage } from 'naive-ui'
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

type Kind = 'request' | 'response' | 'header'

/**
 * 各形态的列配置：请求头无类型与必选、有启用；返回参数有必选、无启用。
 * 文案存 labelKey，切语言时由 cfg 重新解析。color 不再用于计数圆标（统一中性徽标）。
 */
const COLUMNS: Record<Kind, {
  titleKey: string; nameKey: string; valueKey: string; withEnabled: boolean; withRequired: boolean; withType: boolean
}> = {
  request: {
    titleKey: 'param.requestTitle', nameKey: 'param.name', valueKey: 'param.example',
    withEnabled: true, withRequired: true, withType: true,
  },
  response: {
    titleKey: 'param.responseTitle', nameKey: 'param.name', valueKey: 'param.example',
    withEnabled: false, withRequired: true, withType: true,
  },
  header: {
    titleKey: 'param.headerTitle', nameKey: 'param.headerName', valueKey: 'param.value',
    withEnabled: true, withRequired: false, withType: false,
  },
}

/** 列宽：名称/值/说明按 fr 分享，勾选与操作列固定 px（与客户端同比例）。 */
const GRIDS: Record<Kind, string> = {
  request: '24px minmax(0,1.3fr) 108px 40px minmax(0,1fr) minmax(0,1.4fr) 56px',
  response: 'minmax(0,1.3fr) 108px 44px minmax(0,1fr) minmax(0,1.4fr) 56px',
  header: '24px minmax(0,1.1fr) minmax(0,1.4fr) minmax(0,1.4fr) 24px',
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
  return { ...c, title: t(c.titleKey), nameLabel: t(c.nameKey), valueLabel: t(c.valueKey) }
})
const gridCols = computed(() => GRIDS[props.kind])
/** 已命名的行数，用于标题旁计数徽标（空行不算）。 */
const filled = computed(() => rows.value.filter((r) => (r.fieldName ?? '').trim()).length)
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
  return props.editable && cfg.value.withType && (row.type === 'object' || row.type === 'array')
}

/** 新增行的必填默认值：返回参数按产品口径默认「是」，请求参数默认「否」。 */
function defaultRequired(): boolean {
  return props.kind === 'response'
}

/** 一行空数据（新建 / 末尾占位）。 */
function blankRow(): ApiParam {
  return {
    fieldName: '',
    type: props.kind === 'header' ? '' : 'string',
    required: defaultRequired(),
    description: '',
    example: '',
  }
}

/** 该行是否已有输入：名称 / 示例值 / 说明任一非空（只有说明也算，别把已写的备注当空行覆盖掉）。 */
function hasInput(row: ApiParam): boolean {
  return (row.fieldName ?? '').trim() !== '' ||
    (row.example ?? '').trim() !== '' ||
    (row.description ?? '').trim() !== ''
}

/**
 * 编辑态末尾始终留一行空行（对齐客户端 KeyValueTable 的 ensureTrailingBlank）：
 * 末行一有输入就补一行，省去每次手动点「添加一行」。补出来的空行不参与命名计数，
 * 保存前由 `cleanParams` 过滤，不会写进服务端或集合文件。
 */
function ensureTrailingBlank(): void {
  if (!props.editable) return
  const last = rows.value[rows.value.length - 1]
  if (last && !hasInput(last)) return
  rows.value.push(blankRow())
}

onMounted(ensureTrailingBlank)
watch(() => props.editable, ensureTrailingBlank)
watch(rows, ensureTrailingBlank, { deep: true })

function add(): void {
  rows.value.push(blankRow())
  void nextTick(() => nameRefs.value[rows.value.length - 1]?.focus())
}

function addChild(i: number): void {
  const parent = rows.value[i]
  const name = parent.fieldName ?? ''
  const prefix = parent.type === 'array' ? `${name}[]` : name
  const next = [...rows.value]
  next.splice(i + 1, 0, {
    fieldName: `${prefix}.`, type: 'string', required: defaultRequired(), description: '', example: '',
  })
  writeRows(next)
  void nextTick(() => nameRefs.value[i + 1]?.focus())
}

function remove(i: number): void {
  writeRows(rows.value.filter((_, idx) => idx !== i))
}

function setRequired(row: ApiParam, checked: boolean): void {
  row.required = checked
}

/** 启用位：只在显式 false 时写键（缺省 = 启用），与客户端同步契约一致。 */
function setEnabled(row: ApiParam, checked: boolean): void {
  if (checked) delete row.enabled
  else row.enabled = false
}

async function copyJson(): Promise<void> {
  // 只导出已命名的行：编辑态末尾的占位空行不写进剪贴板。
  const text = JSON.stringify(rows.value.filter((r) => (r.fieldName ?? '').trim()), null, 2)
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
        <span v-if="filled" class="rg-num" data-testid="param.count">{{ filled }}</span>
        <span v-if="titleExtra" class="pt-extra">{{ titleExtra }}</span>
      </span>
      <div class="pt-ops">
        <slot name="actions" />
        <n-button v-if="editable" text size="tiny" @click="openJson">{{ t('param.jsonToTable') }}</n-button>
        <n-button v-if="filled" text size="tiny" @click="copyJson">{{ t('param.copyJson') }}</n-button>
      </div>
    </div>

    <div class="rg-wrap" :style="`--rg-cols: ${gridCols}`">
      <template v-if="rows.length">
        <div class="rg-tr rg-th">
          <span v-if="cfg.withEnabled" class="rg-c" />
          <span>{{ cfg.nameLabel }}</span>
          <span v-if="cfg.withType">{{ t('param.type') }}</span>
          <span v-if="cfg.withRequired" class="rg-c">{{ t('detail.colRequired') }}</span>
          <span>{{ cfg.valueLabel }}</span>
          <span>{{ t('param.desc') }}</span>
          <span />
        </div>

        <div
          v-for="(row, i) in rows"
          :key="i"
          class="rg-tr"
          data-testid="param.row"
          :data-path="row.fieldName"
        >
          <span v-if="cfg.withEnabled" class="rg-c">
            <n-checkbox
              :checked="rowEnabled(row)"
              :disabled="!editable"
              size="small"
              :title="t('param.disabledHint')"
              data-testid="param.enabled"
              @update:checked="(v: boolean) => setEnabled(row, v)"
            />
          </span>
          <n-input
            v-if="editable"
            :ref="(el) => bindNameInput(el, i)"
            v-model:value="row.fieldName"
            size="tiny"
            :placeholder="cfg.nameLabel"
            data-testid="param.name"
          />
          <span v-else class="row-mono">{{ row.fieldName || '—' }}</span>

          <template v-if="cfg.withType">
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
          </template>

          <span v-if="cfg.withRequired" class="rg-c">
            <n-checkbox
              :checked="row.required === true"
              :disabled="!editable"
              size="small"
              @update:checked="(v: boolean) => setRequired(row, v)"
            />
          </span>

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

          <n-input v-if="editable" v-model:value="row.description" size="tiny" />
          <span v-else>{{ row.description || '—' }}</span>

          <span class="rg-ops">
            <template v-if="editable">
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
            </template>
          </span>
        </div>
      </template>

      <!-- 查看态空表：边框内一行灰字 + 切到编辑态（对齐客户端空态文案的观感） -->
      <p v-else class="rg-empty">
        <span>{{ t('param.empty', { title: cfg.title }) }}</span>
        <n-button text size="tiny" type="primary" data-testid="param.emptyEdit" @click="emit('edit')">
          ＋ {{ t('param.addRow') }}
        </n-button>
      </p>

      <div v-if="editable" class="rg-foot">
        <n-button text size="tiny" type="primary" data-testid="param.add" @click="add">
          <template #icon><n-icon :component="AddOutline" /></template>
          {{ t('param.addRow') }}
        </n-button>
      </div>
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
  font-size: 12.5px;
  font-weight: 600;
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
  gap: 6px;
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
