<script setup lang="ts">
// 文档工作台：左侧树（分组 + 文档叶子 + 树上搜索），右侧仅承载
// 文档详情（?doc=<id>，可 ?edit=1 编辑）、回收站（?trash=1）；无选中时显示占位提示创建。
import { createDoc, deleteDoc, listDocs, restoreDoc } from '@/api/content'
import DocDetailView from '@/views/project/DocDetailView.vue'
import EmptyHint from '@/components/EmptyHint.vue'
import SectionTree from '@/components/SectionTree.vue'
import TablePager from '@/components/TablePager.vue'
import { useNumParam } from '@/composables/params'
import { useUserNames } from '@/composables/userNames'
import { useProjectStore } from '@/stores/project'
import { ErrCode } from '@/types/api'
import type { DocItem } from '@/types/entities'
import { fmtTime } from '@/utils/format'
import { DocumentTextOutline, SearchOutline } from '@vicons/ionicons5'
import {
    NButton, NCard, NEmpty, NForm, NFormItem, NIcon, NInput, NModal, NSpin, NTable, useDialog, useMessage,
} from 'naive-ui'
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

const route = useRoute()
const router = useRouter()
const message = useMessage()
const dialog = useDialog()
const project = useProjectStore()
const { ensure: ensureUsers, nameOf } = useUserNames()

const projectId = useNumParam('projectId')
const trash = computed(() => route.query.trash === '1')
/** 右侧视图：回收站 / 详情 / 空态占位。 */
const docId = computed(() => Number(route.query.doc) || 0)

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
  try {
    const res = await listDocs({
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
      message.error(res.msg || '加载失败')
    }
  } finally {
    loading.value = false
  }
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
    message.warning('请输入文档标题')
    return
  }
  const res = await createDoc({ project_id: projectId.value, group_id: 0, title: form.title.trim() })
  if (res.code !== ErrCode.Success || !res.data) {
    message.error(res.msg || '创建失败')
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

async function restore(d: DocItem): Promise<void> {
  const res = await restoreDoc(d.id)
  if (res.code !== ErrCode.Success) message.error(res.msg || '恢复失败')
  void treeRef.value?.reload()
  await loadTrash()
}

function remove(d: DocItem): void {
  dialog.warning({
    title: '彻底删除',
    content: `确认删除回收站中的「${d.title}」？`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      const res = await deleteDoc(d.id)
      if (res.code !== ErrCode.Success) message.error(res.msg || '删除失败')
      await loadTrash()
    },
  })
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
          placeholder="搜索回收站文档"
          clearable
          class="search"
          @keyup.enter="reload(1)"
          @clear="reload(1)"
        >
          <template #suffix><n-icon :component="SearchOutline" /></template>
        </n-input>
        <n-button @click="reload(1)">搜索</n-button>
      </template>
    </teleport>

    <div class="split-main">
      <!-- 回收站 -->
      <n-card v-if="trash" :bordered="true" class="pane-card table-card" content-style="padding: 0">
        <n-spin :show="loading">
          <n-empty v-if="!rows.length && !loading" class="empty-block" description="回收站为空" />
          <div v-else class="table-scroll">
            <n-table :single-line="false" size="medium" class="wide">
              <thead>
                <tr>
                  <th style="width: 40%">标题</th>
                  <th style="width: 168px">更新时间</th>
                  <th style="width: 120px">操作</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="d in rows" :key="d.id">
                  <td>{{ d.title }}</td>
                  <td class="muted nowrap">{{ fmtTime(d.update_time) }}</td>
                  <td>
                    <div class="row-actions">
                      <n-button text type="primary" size="small" @click="restore(d)">恢复</n-button>
                      <n-button v-if="canDelete" text type="error" size="small" @click="remove(d)">删除</n-button>
                    </div>
                  </td>
                </tr>
              </tbody>
            </n-table>
          </div>
        </n-spin>
        <table-pager :total="total" :page-num="pageNum" :page-size="pageSize" @update:page-num="reload" />
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
        title="从左侧文档集合中选择一篇文档查看"
        :tips="[
          { label: '查看文档', value: '点击文档集合中的文档' },
          { label: '新建文档', value: '点击右上角「创建文档」' },
        ]"
      >
        <n-button v-if="canCreate" type="primary" @click="showCreate = true">创建文档</n-button>
      </empty-hint>
    </div>

    <n-modal v-model:show="showCreate" preset="card" title="创建文档" style="width: 420px">
      <n-form label-placement="left" label-width="56">
        <n-form-item label="标题">
          <n-input v-model:value="form.title" placeholder="文档标题" @keyup.enter="submit" />
        </n-form-item>
      </n-form>
      <template #footer>
        <div class="foot">
          <n-button size="small" @click="showCreate = false">取消</n-button>
          <n-button type="primary" size="small" @click="submit">创建并编辑</n-button>
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
</style>
