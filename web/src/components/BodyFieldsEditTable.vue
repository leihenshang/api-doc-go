<script setup lang="ts">
// 请求体字段表（编辑态）：与客户端 `BodyFieldsTable` 同构 —— 5 列全可编辑
// （字段名 / 类型 / 必填 / 示例值 / 说明）+ 行尾删除 + 底部「新增行」。
//
// 真源是 `http_request_body_fields`（客户端「生成数据表格」派生，或 Web 手工增行）。
// 本组件不持有状态：所有修改通过 v-model 写回父级的 bodyFields，由父级随整体保存提交。
import { NButton, NCheckbox, NIcon, NInput, NSelect } from 'naive-ui'
import { AddOutline, TrashOutline } from '@vicons/ionicons5'
import { useI18n } from 'vue-i18n'

export interface BodyFieldRow {
  name?: string
  type?: string
  required?: boolean
  example?: string
  description?: string
}

const rows = defineModel<BodyFieldRow[]>({ required: true })
const { t } = useI18n()

const TYPE_OPTIONS = ['string', 'integer', 'number', 'boolean', 'array', 'object', 'null']

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
      <span class="bfe-hint meta">{{ t('detail.bodyFieldsHint') }}</span>
    </div>

    <div class="bfe-table">
      <div class="bfe-tr bfe-th">
        <span>{{ t('detail.colName') }}</span>
        <span>{{ t('detail.colType') }}</span>
        <span class="c">{{ t('detail.colRequired') }}</span>
        <span>{{ t('detail.colExample') }}</span>
        <span>{{ t('detail.colDesc') }}</span>
        <span />
      </div>

      <div v-for="(f, i) in rows" :key="i" class="bfe-tr" data-testid="detail.bodyFieldRow" :data-path="f.name">
        <n-input v-model:value="f.name" size="small" :placeholder="t('detail.colName')" data-testid="detail.bodyFieldName" />
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
        <span class="c">
          <n-checkbox v-model:checked="f.required" data-testid="detail.bodyFieldRequired" />
        </span>
        <n-input v-model:value="f.example" size="small" :placeholder="t('detail.colExample')" data-testid="detail.bodyFieldExample" />
        <n-input v-model:value="f.description" size="small" :placeholder="t('detail.colDesc')" data-testid="detail.bodyFieldDesc" />
        <n-button size="tiny" quaternary type="error" :title="t('common.delete')" data-testid="detail.bodyFieldRemove" @click="removeRow(i)">
          <template #icon><n-icon :component="TrashOutline" /></template>
        </n-button>
      </div>

      <div v-if="!rows.length" class="bfe-empty meta">
        {{ t('detail.bodyFieldsEmpty') }}
      </div>
    </div>

    <n-button size="tiny" secondary class="bfe-add" data-testid="detail.bodyFieldAdd" @click="addRow">
      <template #icon><n-icon :component="AddOutline" /></template>
      {{ t('detail.addRow') }}
    </n-button>
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
  align-items: baseline;
  gap: 8px;
}

.bfe-title {
  font-size: 12px;
  font-weight: 600;
}

.bfe-hint {
  font-size: 12px;
}

.bfe-table {
  border: 1px solid var(--app-border);
  border-radius: 6px;
  overflow: hidden;
}

.bfe-tr {
  display: grid;
  grid-template-columns: 1.2fr 1fr 56px 1.2fr 1.4fr 34px;
  gap: 8px;
  align-items: center;
  padding: 6px 10px;
  border-bottom: 1px solid var(--app-border);
}

.bfe-tr:last-child {
  border-bottom: none;
}

.bfe-tr:nth-child(even):not(.bfe-th) {
  background: var(--app-surface-2, transparent);
}

.bfe-th {
  font-size: 12px;
  color: var(--app-muted);
  background: var(--app-surface-3, transparent);
}

.c {
  display: flex;
  justify-content: center;
}

.bfe-empty {
  padding: 14px;
  text-align: center;
  font-size: 12px;
}

.bfe-add {
  align-self: flex-start;
}
</style>
