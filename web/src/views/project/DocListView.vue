<script setup lang="ts">
// 文档工作台：左侧树（分组 + 文档叶子 + 树上搜索），右侧仅承载
// 文档详情（?doc=<id>，可 ?edit=1 编辑）、回收站（?trash=1）；无选中时显示占位提示创建。
import { createDoc, listDocs, purgeDocsBatch, restoreDocsBatch } from '@/api/content'
import DocDetailView from '@/views/project/DocDetailView.vue'
import EmptyHint from '@/components/EmptyHint.vue'
import SectionTree from '@/components/SectionTree.vue'
import TablePager from '@/components/TablePager.vue'
import { useNumParam } from '@/composables/params'
import { useI18n } from 'vue-i18n'
import { useUserNames } from '@/composables/userNames'
import { useProjectStore } from '@/stores/project'
import { ErrCode } from '@/types/api'
import type { DocItem } from '@/types/entities'
import { fmtTime } from '@/utils/format'
import { DocumentTextOutline, SearchOutline } from '@vicons/ionicons5'
import {
    NButton, NCard, NCheckbox, NEmpty, NForm, NFormItem, NIcon, NInput, NModal, NSpin, NTable, useDialog, useMessage,
} from 'naive-ui'
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

const route = useRoute()
const router = useRouter()
const message = useMessage()
const { t } = useI18n()
const dialog = useDialog()
const project = useProjectStore()
const { ensure: ensureUsers } = useUserNames()

const projectId = useNumParam('projectId')
const trash = computed(() => route.query.trash === '1')
/** 右侧视图：回收站 / 详情 / 空态占位。 */
const docId = computed(() => Number(route.query.doc) || 0)
/** B19：当前选中的文档分组（?group=<id>；0/缺省 = 不过滤）。 */
const groupId = computed(() => Number(route.query.group) || 0)
const groupTitle = computed(() => {
  const id = groupId.value
  if (!id) return ''
  return project.docGroups.find((g) => g.id === id)?.title ?? ''
})

const treeRef = ref<InstanceType<typeof SectionTree> | null>(null)

// ---- 回收站列表（?trash=1） ----
const rows = ref<DocItem[]>([])
const total = ref(0)
const pageNum = ref(1)
const pageSize = ref(20)
const loading = ref(false)
const keyword = ref('')

// ---- 新建文档 ----
const showCreate = ref(false)
const form = reactive({ title: '' })

const canCreate = computed(() => project.can(2))
const canDelete = computed(() => project.can(8))

async function loadTrash(): Promise<void> {
  loading.value = true
  // 翻页/搜索后原勾选已不在当前页，直接清空避免误操作。
  selected.value = []
  try {
    const res = await listDocs({
      project_id: projectId.value,
      group_id: groupId.value || undefined,
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

/** 回收站可选每页条数（后端上限 200）：批量还原时一页勾选更省事。 */
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

async function submit(): Promise<void> {
  if (!form.title.trim()) {
    message.warning(t('doc.titleRequired'))
    return
  }
  // B19：新建落在当前选中的分组（未选 = 未分组），与客户端「文档在所在目录」对齐
  const res = await createDoc({ project_id: projectId.value, group_id: groupId.value, title: form.title.trim() })
  if (res.code !== ErrCode.Success || !res.data) {
    message.error(res.msg || t('common.createFailed'))
    return
  }
  showCreate.value = false
  form.title = ''
  void treeRef.value?.reload()
  void router.push({
    name: 'projectDocs',
    params: { projectId: projectId.value },
    query: { doc: String(res.data.id), edit: '1' },
  })
}

function closeDetail(): void {
  void router.replace({ name: 'projectDocs', params: { projectId: projectId.value } })
}

// ---- 回收站批量操作：勾选还原 / 彻底删除（操作列冻结）----
/** 已勾选的文档 id（仅当前页）。 */
const selected = ref<number[]>([])
const acting = ref(false)
const allChecked = computed(() => rows.value.length > 0 && selected.value.length === rows.value.length)
const someChecked = computed(() => selected.value.length > 0 && !allChecked.value)

function toggleAll(checked: boolean): void {
  selected.value = checked ? rows.value.map((d) => d.id) : []
}

function toggleOne(id: number, checked: boolean): void {
  selected.value = checked ? [...selected.value, id] : selected.value.filter((x) => x !== id)
}

/** 批量还原：服务端会把「已删除的所属分组（含祖先链）」一并恢复。 */
async function restoreSelected(): Promise<void> {
  if (!selected.value.length) return
  acting.value = true
  try {
    const res = await restoreDocsBatch([...selected.value])
    if (res.code !== ErrCode.Success || !res.data) {
      message.error(res.msg || t('doc.restoreFailed'))
      return
    }
    const extra = res.data.groups > 0 ? t('common.restoreExtra', { n: res.data.groups }) : ''
    message.success(t('doc.restored', { n: res.data.restored }) + extra)
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
    content: t('doc.purgeConfirm', { n: count }),
    positiveText: t('common.purge'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      const res = await purgeDocsBatch([...selected.value])
      if (res.code !== ErrCode.Success) {
        message.error(res.msg || t('common.deleteFailed'))
        return false
      }
      message.success(t('doc.purged', { n: count }))
      selected.value = []
      await loadTrash()
    },
  })
}

async function restoreOne(d: DocItem): Promise<void> {
  selected.value = [d.id]
  await restoreSelected()
}

function purgeOne(d: DocItem): void {
  selected.value = [d.id]
  purgeSelected()
}

/** B19：清除分组过滤（回到「全部文档」）。 */
function clearGroupFilter(): void {
  void router.push({ name: 'projectDocs', params: { projectId: projectId.value }, query: {} })
}

onMounted(() => {
  void ensureUsers()
  if (trash.value) void loadTrash()
})
</script>

<template>
  <div class="split">
    <!-- 回收站态覆盖整个内容区：隐藏左侧树 -->
    <aside v-if="!trash" class="split-tree">
      <section-tree ref="treeRef" :project-id="projectId" :type="3" section="docs" />
    </aside>
    <!-- 回收站搜索：经 Teleport 注入到面包屑行右侧 -->
    <teleport to="#crumb-actions" defer>
      <template v-if="trash">
        <n-input
          v-model:value="keyword"
          :placeholder="t('doc.searchTrashPlaceholder')"
          clearable
          class="search"
          @keyup.enter="reload(1)"
          @clear="reload(1)"
        >
          <template #suffix><n-icon :component="SearchOutline" /></template>
        </n-input>
        <n-button @click="reload(1)">{{ t('common.search') }}</n-button>
      </template>
    </teleport>

    <div class="split-main">
      <!-- B19：当前分组过滤（点左侧分组标题设置；清除后显示全部文档） -->
      <div v-if="!trash && groupId > 0" class="doc-filter" data-testid="docs.groupFilter">
        <span class="lbl">{{ t('doc.groupFilter', { name: groupTitle || t('tree.ungrouped') }) }}</span>
        <n-button size="tiny" quaternary data-testid="docs.groupClear" @click="clearGroupFilter">
          {{ t('doc.clearFilter') }}
        </n-button>
      </div>
      <!-- 回收站 -->
      <n-card v-if="trash" :bordered="true" class="pane-card table-card" content-style="padding: 0">
        <!-- 批量操作条：勾选后一键还原（服务端会一并恢复所属分组） -->
        <div class="trash-bar">
          <n-checkbox :checked="allChecked" :indeterminate="someChecked" @update:checked="toggleAll">{{ t('common.selectAllPage') }}</n-checkbox>
          <span class="meta">{{ t('common.selected', { n: selected.length, total: rows.length }) }}</span>
          <div class="row-actions">
            <n-button size="small" type="primary" :disabled="!selected.length" :loading="acting" @click="restoreSelected">
              {{ t('common.batchRestore') }}
            </n-button>
            <n-button v-if="canDelete" size="small" type="error" :disabled="!selected.length" @click="purgeSelected">
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
                  <th style="width: 40%">{{ t('doc.titleField') }}</th>
                  <th style="width: 168px">{{ t('doc.updatedAt') }}</th>
                  <th class="col-ops" style="width: 120px">{{ t('api.actions') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="d in rows" :key="d.id">
                  <td class="col-check">
                    <n-checkbox :checked="selected.includes(d.id)" @update:checked="(v: boolean) => toggleOne(d.id, v)" />
                  </td>
                  <td>{{ d.title }}</td>
                  <td class="muted nowrap">{{ fmtTime(d.update_time) }}</td>
                  <td class="col-ops">
                    <div class="row-actions">
                      <n-button text type="primary" size="small" @click="restoreOne(d)">{{ t('api.restore') }}</n-button>
                      <n-button v-if="canDelete" text type="error" size="small" @click="purgeOne(d)">{{ t('common.delete') }}</n-button>
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

      <!-- 文档详情（查看/编辑） -->
      <doc-detail-view
        v-else-if="docId"
        :key="docId"
        :doc-id="docId"
        class="pane"
        embedded
        @close="closeDetail"
        @changed="treeRef?.reload()"
      />

      <!-- 空态占位：提示创建文档 -->
      <empty-hint
        v-else
        :icon="DocumentTextOutline"
        :title="t('doc.emptyHint')"
        :tips="[
          { label: t('doc.hintView'), value: t('doc.hintViewV') },
          { label: t('doc.hintCreate'), value: t('doc.hintCreateV') },
        ]"
      >
        <n-button v-if="canCreate" type="primary" @click="showCreate = true">{{ t('doc.create') }}</n-button>
      </empty-hint>
    </div>

    <n-modal v-model:show="showCreate" preset="card" :title="t('doc.create')" style="width: 420px">
      <n-form label-placement="left" label-width="56">
        <n-form-item :label="t('doc.titleField')">
          <n-input v-model:value="form.title" :placeholder="t('doc.titlePlaceholder')" @keyup.enter="submit" />
        </n-form-item>
      </n-form>
      <template #footer>
        <div class="foot">
          <n-button size="small" @click="showCreate = false">{{ t('common.cancel') }}</n-button>
          <n-button type="primary" size="small" @click="submit">{{ t('doc.createAndEdit') }}</n-button>
        </div>
      </template>
    </n-modal>
  </div>
</template>

<style scoped>
.wide {
  min-width: 560px;
}

.search {
  width: 200px;
}

.foot {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

/* B19：分组过滤条 */
.doc-filter {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  border-bottom: 1px solid var(--app-border);
  font-size: 12px;
  color: var(--app-muted);
}

.doc-filter .lbl {
  color: var(--app-text);
}
</style>
