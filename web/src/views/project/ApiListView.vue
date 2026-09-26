<script setup lang="ts">
// 接口工作台：左侧树（目录 + 接口叶子 + 树上搜索，新建/删除都在树上），右侧仅承载
// 接口详情（?api=<id>，可 ?edit=1 编辑）、新建（?new=1，树上入口）、回收站（?trash=1）。
// 导入 / 导出在「项目概况」页。
import { createApi, listApis, purgeApisBatch, restoreApisBatch } from '@/api/content'
import ApiDetailView from '@/views/project/ApiDetailView.vue'
import ApiFieldsEditor from '@/components/ApiFieldsEditor.vue'
import EmptyHint from '@/components/EmptyHint.vue'
import MethodTag from '@/components/MethodTag.vue'
import SectionTree from '@/components/SectionTree.vue'
import TablePager from '@/components/TablePager.vue'
import { useNumParam } from '@/composables/params'
import { useI18n } from 'vue-i18n'
import { useProjectStore } from '@/stores/project'
import { ErrCode } from '@/types/api'
import type { ApiDoc, ApiParam } from '@/types/entities'
import { paramsToHeaders } from '@/utils/api-params'
import { fmtTime } from '@/utils/format'
import { ChevronForwardOutline, SearchOutline, SendOutline } from '@vicons/ionicons5'
import {
    NButton, NCard, NCheckbox, NEmpty, NForm, NFormItem, NIcon, NInput, NSelect, NSpin, NTable, useDialog, useMessage,
} from 'naive-ui'
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

const route = useRoute()
const router = useRouter()
const message = useMessage()
const { t } = useI18n()
const dialog = useDialog()
const project = useProjectStore()

const projectId = useNumParam('projectId')
const trash = computed(() => route.query.trash === '1')
/** 右侧视图：新建 / 回收站 / 详情 / 空态占位。 */
const creating = computed(() => route.query.new === '1')
const apiId = computed(() => Number(route.query.api) || 0)

const treeRef = ref<InstanceType<typeof SectionTree> | null>(null)
/** 接口集合左栏是否收起（收起后仅留窄条展开入口）。 */
const treeCollapsed = ref(false)

// ---- 回收站列表（?trash=1） ----
const rows = ref<ApiDoc[]>([])
const total = ref(0)
const pageNum = ref(1)
const pageSize = ref(20)
const loading = ref(false)
const keyword = ref('')

// ---- 新建接口（入口在左侧树的目录「＋」菜单） ----
const saving = ref(false)
const form = reactive({
  api_name: '', http_method_type: 'GET', url: '', protocol_type: 'HTTP',
  http_return_type: 'JSON', develop_language: '', description: '', group_id: 0,
})
/** 请求头以参数行形态录入（表格改的就是这份数组），提交时转回 ApiHeader[]。 */
const headerRows = ref<ApiParam[]>([])
const reqParams = ref<ApiParam[]>([])
const resParams = ref<ApiParam[]>([])
const sample = ref('')

const methodOptions = ['GET', 'POST', 'PUT', 'DELETE', 'PATCH'].map((m) => ({ label: m, value: m }))
const groupOptions = computed(() => [
  { label: t('tree.ungrouped'), value: 0 },
  ...project.apiGroups.map((g) => ({ label: g.title, value: g.id })),
])

const canDelete = () => project.can(8)

/** 进入新建态时按 ?group 预选所属目录（树上「新建接口」会带上该参数）。 */
watch(creating, (v) => {
  if (v) form.group_id = Number(route.query.group) || 0
})

async function loadTrash(): Promise<void> {
  loading.value = true
  // 翻页/搜索后原勾选已不在当前页，直接清空避免误操作。
  selected.value = []
  try {
    const res = await listApis({
      project_id: projectId.value,
      deleted: 1,
      keyword: keyword.value.trim() || undefined,
      page_num: pageNum.value,
      page_size: pageSize.value,
    })
    if (res.code === ErrCode.Success && res.data) {
      rows.value = res.data.items
      total.value = res.data.total
    } else {
      message.error(res.msg || t('common.loadFailed'))
    }
  } finally {
    loading.value = false
  }
}

/** 回收站可选每页条数（后端上限 200）：大批量还原时一页勾选更省事。 */
const PAGE_SIZES = [10, 20, 40, 80, 100, 200]

/** 改每页条数后回到第 1 页重查（loadTrash 会清空勾选）。 */
function onPageSize(size: number): void {
  pageSize.value = size
  reload(1)
}

function reload(num = 1): void {
  pageNum.value = num
  void loadTrash()
}

watch(trash, (v) => {
  if (v) reload(1)
})

function cancelCreate(): void {
  void router.replace({ name: 'projectApis', params: { projectId: projectId.value } })
}


async function submit(): Promise<void> {
  if (!form.api_name.trim()) {
    message.warning(t('api.nameRequired'))
    return
  }
  let sampleVal: unknown = ''
  if (sample.value.trim()) {
    try {
      sampleVal = JSON.parse(sample.value)
    } catch {
      message.error(t('api.sampleInvalid'))
      return
    }
  }
  saving.value = true
  try {
    const res = await createApi({
      ...form,
      project_id: projectId.value,
      http_request_header: paramsToHeaders(headerRows.value),
      http_request_params: reqParams.value,
      http_return_params: resParams.value,
      http_return_sample: sampleVal,
    })
    if (res.code !== ErrCode.Success || !res.data) {
      message.error(res.msg || t('common.createFailed'))
      return
    }
    message.success(t('api.created'))
    void treeRef.value?.reload()
    void router.push({ name: 'projectApis', params: { projectId: projectId.value }, query: { api: String(res.data.id) } })
  } finally {
    saving.value = false
  }
}

// ---- 导入 / 导出：已移至「项目概况」页 ----

// ---- 回收站批量操作：勾选还原 / 彻底删除（操作列冻结，方便横向滚动时点击）----
/** 已勾选的接口 id（仅当前页）。 */
const selected = ref<number[]>([])
const acting = ref(false)
const allChecked = computed(() => rows.value.length > 0 && selected.value.length === rows.value.length)
const someChecked = computed(() => selected.value.length > 0 && !allChecked.value)

function toggleAll(checked: boolean): void {
  selected.value = checked ? rows.value.map((a) => a.id) : []
}

function toggleOne(id: number, checked: boolean): void {
  selected.value = checked ? [...selected.value, id] : selected.value.filter((x) => x !== id)
}

/** 批量还原：服务端会把「已删除的所属分组（含祖先链）」一并恢复，避免还原后落到未分组。 */
async function restoreSelected(): Promise<void> {
  if (!selected.value.length) return
  acting.value = true
  try {
    const res = await restoreApisBatch([...selected.value])
    if (res.code !== ErrCode.Success || !res.data) {
      message.error(res.msg || t('api.restoreFailed'))
      return
    }
    const extra = res.data.groups > 0 ? t('api.restoreExtra', { n: res.data.groups }) : ''
    message.success(t('api.restored', { n: res.data.restored }) + extra)
    selected.value = []
    await loadTrash()
    void treeRef.value?.reload()
  } finally {
    acting.value = false
  }
}

/** 批量彻底删除（不可撤销）。 */
function purgeSelected(): void {
  if (!selected.value.length) return
  const count = selected.value.length
  dialog.warning({
    title: t('common.purge'),
    content: t('api.purgeConfirm', { n: count }),
    positiveText: t('common.purge'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      const res = await purgeApisBatch([...selected.value])
      if (res.code !== ErrCode.Success) {
        message.error(res.msg || t('common.deleteFailed'))
        return false
      }
      message.success(t('api.purged', { n: count }))
      selected.value = []
      await loadTrash()
    },
  })
}

async function restoreOne(a: ApiDoc): Promise<void> {
  selected.value = [a.id]
  await restoreSelected()
}

function purgeOne(a: ApiDoc): void {
  selected.value = [a.id]
  purgeSelected()
}

onMounted(() => {
  if (trash.value) void loadTrash()
})
</script>

<template>
  <div class="split">
    <!-- 新建/回收站态覆盖整个内容区：隐藏左侧树 -->
    <aside v-if="!creating && !trash" class="split-tree" :class="{ 'split-tree--collapsed': treeCollapsed }">
      <button v-if="treeCollapsed" class="tree-rail" :title="t('api.expandTree')" @click="treeCollapsed = false">
        <n-icon :component="ChevronForwardOutline" />
      </button>
      <section-tree
        v-show="!treeCollapsed"
        ref="treeRef"
        :project-id="projectId"
        :type="1"
        section="apis"
        collapsible
        @collapse="treeCollapsed = true"
      />
    </aside>
    <!-- 回收站搜索：经 Teleport 注入到面包屑行右侧 -->
    <teleport v-if="trash" to="#crumb-actions" defer>
      <n-input
        v-model:value="keyword"
        :placeholder="t('api.searchTrashPlaceholder')"
        clearable
        class="search"
        @keyup.enter="reload(1)"
        @clear="reload(1)"
      >
        <template #suffix><n-icon :component="SearchOutline" /></template>
      </n-input>
      <n-button @click="reload(1)">{{ t('common.search') }}</n-button>
    </teleport>

    <div class="split-main">
      <!-- 新建接口：覆盖整个内容区 -->
      <n-card v-if="creating" :bordered="true" class="pane-card">
        <template #header>
          <h2 class="section-title">{{ t('api.createTitle') }}</h2>
        </template>
        <template #header-extra>
          <div class="row-actions">
            <n-button size="small" @click="cancelCreate">{{ t('common.cancel') }}</n-button>
            <n-button size="small" type="primary" :loading="saving" @click="submit">{{ t('common.save') }}</n-button>
          </div>
        </template>
        <n-form label-placement="left" label-width="72" class="form-grid">
          <n-form-item :label="t('api.name')"><n-input v-model:value="form.api_name" :placeholder="t('api.namePlaceholder')" /></n-form-item>
          <n-form-item :label="t('api.group')">
            <n-select v-model:value="form.group_id" :options="groupOptions" />
          </n-form-item>
          <n-form-item :label="t('api.method')">
            <n-select v-model:value="form.http_method_type" :options="methodOptions" />
          </n-form-item>
          <n-form-item label="URL"><n-input v-model:value="form.url" placeholder="/api/xxx" /></n-form-item>
          <n-form-item :label="t('api.protocol')"><n-input v-model:value="form.protocol_type" placeholder="HTTP" /></n-form-item>
          <n-form-item :label="t('api.returnType')"><n-input v-model:value="form.http_return_type" placeholder="JSON" /></n-form-item>
          <n-form-item :label="t('api.devLang')"><n-input v-model:value="form.develop_language" :placeholder="t('api.devLangPlaceholder')" /></n-form-item>
        </n-form>
        <api-fields-editor
          v-model:header-rows="headerRows"
          v-model:req-params="reqParams"
          v-model:res-params="resParams"
          v-model:sample="sample"
          v-model:description="form.description"
        />
      </n-card>

      <!-- 回收站 -->
      <n-card v-else-if="trash" :bordered="true" class="pane-card table-card" content-style="padding: 0">
        <!-- 批量操作条：勾选后一键还原（服务端会一并恢复所属分组） -->
        <div class="trash-bar">
          <n-checkbox :checked="allChecked" :indeterminate="someChecked" @update:checked="toggleAll">{{ t('common.selectAllPage') }}</n-checkbox>
          <span class="meta">{{ t('common.selected', { n: selected.length, total: rows.length }) }}</span>
          <div class="row-actions">
            <n-button size="small" type="primary" :disabled="!selected.length" :loading="acting" @click="restoreSelected">
              {{ t('common.batchRestore') }}
            </n-button>
            <n-button v-if="canDelete()" size="small" type="error" :disabled="!selected.length" @click="purgeSelected">
              {{ t('common.purge') }}
            </n-button>
          </div>
        </div>
        <n-spin :show="loading">
          <n-empty v-if="!rows.length && !loading" class="empty-block" :description="t('common.trashEmpty')" />
          <div v-else class="table-scroll trash-table">
            <n-table :single-line="false" size="medium" class="wide">
              <thead>
                <tr>
                  <th class="col-check" />
                  <th style="width: 24%">{{ t('api.name') }}</th>
                  <th style="width: 96px">{{ t('api.method') }}</th>
                  <th style="width: 30%">URL</th>
                  <th style="width: 168px">{{ t('api.createdAt') }}</th>
                  <th class="col-ops" style="width: 120px">{{ t('api.actions') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="a in rows" :key="a.id">
                  <td class="col-check">
                    <n-checkbox :checked="selected.includes(a.id)" @update:checked="(v: boolean) => toggleOne(a.id, v)" />
                  </td>
                  <td>{{ a.api_name || t('tree.unnamedApi') }}</td>
                  <td><method-tag :method="a.http_method_type" /></td>
                  <td class="row-mono ellipsis" :title="a.url">{{ a.url || '—' }}</td>
                  <td class="muted nowrap">{{ fmtTime(a.create_time) }}</td>
                  <td class="col-ops">
                    <div class="row-actions">
                      <n-button text type="primary" size="small" @click="restoreOne(a)">{{ t('api.restore') }}</n-button>
                      <n-button v-if="canDelete()" text type="error" size="small" @click="purgeOne(a)">{{ t('common.delete') }}</n-button>
                    </div>
                  </td>
                </tr>
              </tbody>
            </n-table>
          </div>
        </n-spin>
        <table-pager
          :total="total"
          :page-num="pageNum"
          :page-size="pageSize"
          :page-sizes="PAGE_SIZES"
          @update:page-num="reload"
          @update:page-size="onPageSize"
        />
      </n-card>

      <!-- 接口详情（查看/编辑） -->
      <api-detail-view
        v-else-if="apiId"
        :key="apiId"
        :api-id="apiId"
        @changed="treeRef?.reload()"
      />

      <!-- 空态占位：Postman 式引导 -->
      <empty-hint
        v-else
        :icon="SendOutline"
        :title="t('api.emptyHint')"
        :tips="[
          { label: t('api.hintDetail'), value: t('api.hintDetailV') },
          { label: t('api.hintCreate'), value: t('api.hintCreateV') },
          { label: t('api.hintNewDir'), value: t('api.hintNewDirV') },
          { label: t('api.hintImport'), value: t('api.hintImportV') },
        ]"
      />
    </div>
  </div>
</template>

<style scoped>
.wide {
  min-width: 760px;
}

.search {
  width: 200px;
}

.form-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  column-gap: 16px;
}

.span-2 {
  grid-column: span 2;
}
</style>
