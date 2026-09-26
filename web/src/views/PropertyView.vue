<script setup lang="ts">
// 公共属性（管理员）：
// ① 标签字典（全局标签复用）② 应用级「全局变量」——跨项目共享，参与变量解析的最低优先级，可被项目变量覆盖。
// 两个列表接口都一次返回全量，分页在前端切片。
import { deleteProperty, listProperties, saveProperty } from '@/api/misc'
import { createVariable, deleteVariable, listVariables, updateVariable } from '@/api/variable'
import TablePager from '@/components/TablePager.vue'
import { useUserStore } from '@/stores/user'
import { ErrCode } from '@/types/api'
import type { Variable } from '@/types/entities'
import {
    NButton, NCard, NCheckbox, NEmpty, NForm, NFormItem, NInput, NModal, NSpin, NTable, NTabPane, NTabs,
    useDialog, useMessage,
} from 'naive-ui'
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'

/** 全局变量（scope=3）不受项目限制，列表查询的 project_id 传 0。 */
const GlobalScope = 3
/** 变量名规则与后端一致。 */
const NAME_RE = /^[A-Za-z_][A-Za-z0-9_]{0,63}$/

interface Property {
  id: number
  tag: string
  tag_name: string
  description: string
}

const message = useMessage()
const dialog = useDialog()
const user = useUserStore()
const isAdmin = computed(() => user.isAdmin)
const { t } = useI18n()

const tab = ref<'tags' | 'vars'>('tags')

// ---- 标签字典 ----
const rows = ref<Property[]>([])
const total = ref(0)
const pageNum = ref(1)
const pageSize = ref(20)
const show = ref(false)
const form = reactive({ tag: '', tag_name: '', description: '' })

const paged = computed(() => rows.value.slice((pageNum.value - 1) * pageSize.value, pageNum.value * pageSize.value))

async function reload(): Promise<void> {
  const res = await listProperties()
  if (res.code === ErrCode.Success && res.data) {
    rows.value = res.data as unknown as Property[]
    total.value = rows.value.length
  }
}

/** 删除/保存后如当前页超界，回退到最后一页。 */
function clampPage(): void {
  const maxPage = Math.max(1, Math.ceil(rows.value.length / pageSize.value))
  if (pageNum.value > maxPage) pageNum.value = maxPage
}

async function save(): Promise<void> {
  if (!form.tag.trim() || !form.tag_name.trim()) {
    message.warning(t('property.tagRequired'))
    return
  }
  const res = await saveProperty({ ...form })
  message[res.code === ErrCode.Success ? 'success' : 'error'](res.code === ErrCode.Success ? t('common.saved') : res.msg)
  show.value = false
  Object.assign(form, { tag: '', tag_name: '', description: '' })
  await reload()
  clampPage()
}

async function remove(id: number): Promise<void> {
  const res = await deleteProperty(id)
  if (res.code !== ErrCode.Success) message.error(res.msg)
  await reload()
  clampPage()
}

// ---- 全局变量 ----
const vars = ref<Variable[]>([])
const varTotal = ref(0)
const varLoading = ref(false)
const varPageNum = ref(1)
const varPageSize = ref(20)
const varShow = ref(false)
const varEditId = ref(0)
/** 编辑中全局变量的载入时间（乐观锁期望值）。 */
const varStamp = ref('')
const varForm = reactive({ name: '', value: '', description: '', is_secret: false, enabled: true })

const varPaged = computed(() =>
  vars.value.slice((varPageNum.value - 1) * varPageSize.value, varPageNum.value * varPageSize.value),
)

async function reloadVars(): Promise<void> {
  varLoading.value = true
  try {
    const res = await listVariables({ project_id: 0, scope: GlobalScope })
    vars.value = res.code === ErrCode.Success && res.data ? res.data : []
    varTotal.value = vars.value.length
  } finally {
    varLoading.value = false
  }
}

function clampVarPage(): void {
  const maxPage = Math.max(1, Math.ceil(vars.value.length / varPageSize.value))
  if (varPageNum.value > maxPage) varPageNum.value = maxPage
}

function openVarCreate(): void {
  varEditId.value = 0
  varStamp.value = ''
  Object.assign(varForm, { name: '', value: '', description: '', is_secret: false, enabled: true })
  varShow.value = true
}

/** 敏感值列表只回掩码，编辑时带 with_value=1 单独取明文（管理员，后端会记操作日志）。 */
async function openVarEdit(row: Variable): Promise<void> {
  let value = row.value
  if (row.is_secret === 1) {
    const res = await listVariables({ project_id: 0, scope: GlobalScope, keyword: row.name, with_value: 1 })
    const hit = res.code === ErrCode.Success && res.data ? res.data.find((v) => v.id === row.id) : undefined
    value = hit ? hit.value : ''
  }
  varEditId.value = row.id
  // 乐观锁：保存时回传载入时间。
  varStamp.value = row.update_time
  Object.assign(varForm, {
    name: row.name,
    value,
    description: row.description,
    is_secret: row.is_secret === 1,
    enabled: row.enabled === 1,
  })
  varShow.value = true
}

async function submitVar(): Promise<void> {
  const name = varForm.name.trim()
  if (!NAME_RE.test(name)) {
    message.warning(t('varForm.nameInvalid'))
    return
  }
  const payload = {
    scope: GlobalScope,
    name,
    value: varForm.value,
    description: varForm.description,
    is_secret: varForm.is_secret ? 1 : 0,
    enabled: varForm.enabled ? 1 : 0,
  }
  const res = varEditId.value
    ? await updateVariable({ id: varEditId.value, ...payload, expected_update_time: varStamp.value })
    : await createVariable(payload)
  if (res.code !== ErrCode.Success) {
    message.error(res.msg || (varEditId.value ? t('common.saveFailed') : t('common.createFailed')))
    if ((res.msg || '').includes('已被他人修改')) await reloadVars()
    return
  }
  message.success(varEditId.value ? t('common.saved') : t('property.var.created'))
  varShow.value = false
  await reloadVars()
  clampVarPage()
}

function removeVar(row: Variable): void {
  dialog.warning({
    title: t('property.var.deleteTitle', { name: row.name }),
    content: t('property.var.deleteConfirm'),
    positiveText: t('common.delete'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      const res = await deleteVariable(row.id)
      if (res.code !== ErrCode.Success) {
        message.error(res.msg || t('common.deleteFailed'))
        return false
      }
      message.success(t('common.deleted'))
      await reloadVars()
      clampVarPage()
    },
  })
}

onMounted(() => {
  void reload()
  void reloadVars()
})
</script>

<template>
  <div class="page">
    <div class="page-hd">
      <div>
        <h1 class="page-title">{{ t('property.title') }}</h1>
        <p class="page-sub">{{ t('property.sub') }}</p>
      </div>
      <n-button v-if="tab === 'tags'" type="primary" @click="show = true">{{ t('property.createTag') }}</n-button>
      <n-button v-else type="primary" @click="openVarCreate">{{ t('property.createVar') }}</n-button>
    </div>

    <n-tabs v-model:value="tab" type="line" class="tabs">
      <n-tab-pane name="tags" :tab="t('property.tabTags')">
        <n-card :bordered="true" class="table-card" content-style="padding: 0">
          <n-empty v-if="!rows.length" class="empty-block" :description="t('property.tagEmpty')" />
          <n-table v-else :single-line="false" size="medium">
            <thead>
              <tr>
                <th style="width: 22%">{{ t('property.tag') }}</th>
                <th style="width: 22%">{{ t('property.tagName') }}</th>
                <th>{{ t('property.description') }}</th>
                <th style="width: 80px">{{ t('property.actions') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="p in paged" :key="p.id">
                <td class="row-mono">{{ p.tag }}</td>
                <td>{{ p.tag_name }}</td>
                <td class="muted">{{ p.description || '—' }}</td>
                <td><n-button text type="error" size="small" @click="remove(p.id)">{{ t('common.delete') }}</n-button></td>
              </tr>
            </tbody>
          </n-table>
          <table-pager :total="total" :page-num="pageNum" :page-size="pageSize" @update:page-num="pageNum = $event" />
        </n-card>
      </n-tab-pane>

      <n-tab-pane v-if="isAdmin" name="vars" :tab="t('property.tabVars')">
        <p class="tip meta">{{ t('property.var.tip') }}</p>
        <n-card :bordered="true" class="table-card" content-style="padding: 0">
          <n-spin :show="varLoading">
            <n-empty v-if="!vars.length && !varLoading" class="empty-block" :description="t('property.var.empty')" />
            <n-table v-else :single-line="false" size="medium">
              <thead>
                <tr>
                  <th style="width: 22%">{{ t('varForm.name') }}</th>
                  <th style="width: 26%">{{ t('varForm.value') }}</th>
                  <th>{{ t('property.description') }}</th>
                  <th style="width: 80px">{{ t('varForm.status') }}</th>
                  <th style="width: 130px">{{ t('property.actions') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="v in varPaged" :key="v.id">
                  <td class="row-mono">{{ v.name }}</td>
                  <td class="row-mono">
                    <span v-if="v.is_secret === 1" class="masked" :title="t('varForm.maskedTip')">{{ v.has_value ? '••••••' : t('varForm.emptySecret') }}</span>
                    <span v-else-if="v.has_value">{{ v.value }}</span>
                    <span v-else class="muted">{{ t('varForm.emptyValue') }}</span>
                  </td>
                  <td>{{ v.description || '—' }}</td>
                  <td class="muted">{{ v.enabled === 1 ? t('varForm.enabled') : t('varForm.disabled') }}</td>
                  <td>
                    <div class="row-actions">
                      <n-button text size="small" @click="openVarEdit(v)">{{ t('common.edit') }}</n-button>
                      <n-button text type="error" size="small" @click="removeVar(v)">{{ t('common.delete') }}</n-button>
                    </div>
                  </td>
                </tr>
              </tbody>
            </n-table>
          </n-spin>
          <table-pager :total="varTotal" :page-num="varPageNum" :page-size="varPageSize"
            @update:page-num="varPageNum = $event" />
        </n-card>
      </n-tab-pane>
    </n-tabs>

    <n-modal v-model:show="show" preset="card" :title="t('property.tagTitle')" style="width: 420px">
      <n-form label-placement="left" label-width="56">
        <n-form-item :label="t('property.tag')"><n-input v-model:value="form.tag" :placeholder="t('property.tagPlaceholder')" /></n-form-item>
        <n-form-item :label="t('property.tagName')"><n-input v-model:value="form.tag_name" /></n-form-item>
        <n-form-item :label="t('property.description')"><n-input v-model:value="form.description" /></n-form-item>
      </n-form>
      <template #footer>
        <div class="foot">
          <n-button size="small" @click="show = false">{{ t('common.cancel') }}</n-button>
          <n-button type="primary" size="small" @click="save">{{ t('common.save') }}</n-button>
        </div>
      </template>
    </n-modal>

    <n-modal v-model:show="varShow" preset="card" :title="varEditId ? t('property.var.editTitle') : t('property.var.createTitle')" style="width: 460px">
      <n-form label-placement="left" label-width="72">
        <n-form-item :label="t('varForm.name')">
          <n-input v-model:value="varForm.name" :placeholder="t('varForm.namePlaceholder')" />
        </n-form-item>
        <n-form-item :label="t('varForm.value')">
          <n-input v-model:value="varForm.value" type="textarea" :rows="2" :placeholder="t('varForm.valuePlaceholder')" />
        </n-form-item>
        <n-form-item :label="t('property.description')"><n-input v-model:value="varForm.description" :placeholder="t('common.optional')" /></n-form-item>
        <n-form-item :label="t('varForm.status')">
          <div class="opts">
            <n-checkbox v-model:checked="varForm.is_secret">{{ t('varForm.secret') }}</n-checkbox>
            <n-checkbox v-model:checked="varForm.enabled">{{ t('varForm.enable') }}</n-checkbox>
          </div>
        </n-form-item>
      </n-form>
      <template #footer>
        <div class="foot">
          <n-button size="small" @click="varShow = false">{{ t('common.cancel') }}</n-button>
          <n-button type="primary" size="small" @click="submitVar">{{ t('common.save') }}</n-button>
        </div>
      </template>
    </n-modal>
  </div>
</template>

<style scoped>
.tabs {
  flex: 1 1 auto;
  min-height: 0;
}

.tip {
  margin: 0 0 12px;
}

.masked {
  letter-spacing: 1px;
  color: var(--app-muted);
}

.opts {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.foot {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
</style>
