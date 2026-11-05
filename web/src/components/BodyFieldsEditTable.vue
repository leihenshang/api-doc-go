<script setup lang="ts">
// 请求体字段表：与客户端 `BodyFieldsTable` 同构 —— 5 列全可编辑
// （字段名 / 类型 / 必填 / 示例值 / 说明）+ 行尾删除 + 底部「新增行」。
//
// 真源是 `http_request_body_fields`（客户端「生成数据表格」派生，或 Web 手工增行）。
// 本组件不持有状态：所有修改通过 v-model 写回父级的 bodyFields，由父级随整体保存提交。
// 样式走全局 `.rg-*`（styles/row-grid.css）：grid 列宽、sticky 表头、行 hover 与客户端一致。
import { computed } from 'vue'
import { NButton, NCheckbox, NIcon, NInput, NSelect } from 'naive-ui'
import { AddOutline, SparklesOutline, TrashOutline } from '@vicons/ionicons5'
import { useI18n } from 'vue-i18n'

export interface BodyFieldRow {
  name?: string
  type?: string
  required?: boolean
  example?: string
  description?: string
}

const props = withDefaults(defineProps<{ showGenerate?: boolean }>(), { showGenerate: true })
const emit = defineEmits<{ (e: 'generate'): void }>()
const rows = defineModel<BodyFieldRow[]>({ required: true })
const { t } = useI18n()

const TYPE_OPTIONS = ['string', 'integer', 'number', 'boolean', 'array', 'object', 'null']
/** 列宽与客户端一致：名称/示例/说明按 fr，类型 108px、必填 40px、操作 32px。 */
const GRID = 'minmax(0,1.3fr) 108px 40px minmax(0,1fr) minmax(0,1.4fr) 32px'

const filled = computed(() => rows.value.filter((f) => (f.name ?? '').trim()).length)

function addRow(): void {
  rows.value = [...rows.value, { name: '', type: 'string', required: false, example: '', description: '' }]
}

function removeRow(i: number): void {
  rows.value = rows.value.filter((_, idx) => idx !== i)
}
</script>

<template>
  <div class="bfe">
    <div class="bfe-hd">
      <span class="bfe-title">{{ t('detail.bodyFieldsTitle') }}</span>
      <span v-if="filled" class="rg-num">{{ filled }}</span>
      <span class="bfe-hint meta">{{ t('detail.bodyFieldsHint') }}</span>
      <n-button
        v-if="props.showGenerate"
        size="tiny"
        class="bfe-gen"
        data-testid="detail.bodyFieldGenerate"
        @click="emit('generate')"
      >
        <template #icon><n-icon :component="SparklesOutline" /></template>
        {{ t('detail.bodyFieldsGen') }}
      </n-button>
    </div>

    <div class="rg-wrap" :style="`--rg-cols: ${GRID}`">
      <template v-if="rows.length">
        <div class="rg-tr rg-th">
          <span>{{ t('detail.colFieldName') }}</span>
          <span>{{ t('detail.colType') }}</span>
          <span class="rg-c">{{ t('detail.colRequired') }}</span>
          <span>{{ t('detail.colExample') }}</span>
          <span>{{ t('detail.colDesc') }}</span>
          <span />
        </div>

        <div v-for="(f, i) in rows" :key="i" class="rg-tr" data-testid="detail.bodyFieldRow" :data-path="f.name">
          <n-input v-model:value="f.name" size="small" :placeholder="t('detail.colFieldName')" data-testid="detail.bodyFieldName" />
          <n-select
            v-model:value="f.type"
            size="small"
            filterable
            tag
            clearable
            :options="TYPE_OPTIONS.map((v) => ({ label: v, value: v }))"
            :placeholder="t('detail.colType')"
            data-testid="detail.bodyFieldType"
          />
          <span class="rg-c">
            <n-checkbox v-model:checked="f.required" size="small" data-testid="detail.bodyFieldRequired" />
          </span>
          <n-input v-model:value="f.example" size="small" :placeholder="t('detail.colExample')" data-testid="detail.bodyFieldExample" />
          <n-input v-model:value="f.description" size="small" :placeholder="t('detail.colDesc')" data-testid="detail.bodyFieldDesc" />
          <span class="rg-ops">
            <n-button size="tiny" quaternary type="error" :title="t('common.delete')" data-testid="detail.bodyFieldRemove" @click="removeRow(i)">
              <template #icon><n-icon :component="TrashOutline" /></template>
            </n-button>
          </span>
        </div>
      </template>

      <p v-else class="rg-empty">{{ t('detail.bodyFieldsEmpty') }}</p>

      <div class="rg-foot">
        <n-button size="tiny" secondary data-testid="detail.bodyFieldAdd" @click="addRow">
          <template #icon><n-icon :component="AddOutline" /></template>
          {{ t('detail.addRow') }}
        </n-button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.bfe {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.bfe-hd {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.bfe-title {
  font-size: 12.5px;
  font-weight: 600;
}

.bfe-hint {
  font-size: 11.5px;
}

.bfe-gen {
  margin-left: auto;
}
</style>
