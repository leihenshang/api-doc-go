<script setup lang="ts">
// 接口工作台：左侧树（分组 + 接口叶子 + 树上搜索），右侧仅承载
// 接口详情（?api=<id>，可 ?edit=1 编辑）、新建（?new=1）、回收站（?trash=1）；
// 无选中时显示占位提示创建。导入/导出仍在面包屑栏右侧。
import { createApi, deleteApi, exportOpenApi, importOpenApi, listApis, restoreApi } from '@/api/content'
import ApiDetailView from '@/views/project/ApiDetailView.vue'
import ApiFieldsEditor from '@/components/ApiFieldsEditor.vue'
import EmptyHint from '@/components/EmptyHint.vue'
import MethodTag from '@/components/MethodTag.vue'
import SectionTree from '@/components/SectionTree.vue'
import TablePager from '@/components/TablePager.vue'
import { useNumParam } from '@/composables/params'
import { useProjectStore } from '@/stores/project'
import { ErrCode } from '@/types/api'
import type { ApiDoc, ApiHeader, ApiParam } from '@/types/entities'
import { fmtTime } from '@/utils/format'
import { SearchOutline, SendOutline } from '@vicons/ionicons5'
import {
    NButton, NCard, NEmpty, NForm, NFormItem, NIcon, NInput, NModal, NSelect, NSpin, NTable, NUpload, useDialog, useMessage,
} from 'naive-ui'
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { parse as parseYaml } from 'yaml'
import type { UploadFileInfo } from 'naive-ui'

const route = useRoute()
const router = useRouter()
const message = useMessage()
const dialog = useDialog()
const project = useProjectStore()

const projectId = useNumParam('projectId')
const trash = computed(() => route.query.trash === '1')
/** 右侧视图：新建 / 回收站 / 详情 / 空态占位。 */
const creating = computed(() => route.query.new === '1')
const apiId = computed(() => Number(route.query.api) || 0)

const treeRef = ref<InstanceType<typeof SectionTree> | null>(null)

// ---- 回收站列表（?trash=1） ----
const rows = ref<ApiDoc[]>([])
const total = ref(0)
const pageNum = ref(1)
const pageSize = ref(20)
const loading = ref(false)
const keyword = ref('')

// ---- 新建接口 ----
const showImport = ref(false)
const importFile = ref<File | null>(null)
const saving = ref(false)
const form = reactive({
  api_name: '', http_method_type: 'GET', url: '', protocol_type: 'HTTP',
  http_return_type: 'JSON', develop_language: '', description: '', group_id: 0,
})
const headers = ref<ApiHeader[]>([])
const reqParams = ref<ApiParam[]>([])
const resParams = ref<ApiParam[]>([])
const sample = ref('')

const methodOptions = ['GET', 'POST', 'PUT', 'DELETE', 'PATCH'].map((m) => ({ label: m, value: m }))
const groupOptions = computed(() => [
  { label: '全部接口', value: 0 },
  ...project.apiGroups.map((g) => ({ label: g.title, value: g.id })),
])

const canCreate = () => project.can(2)
const canDelete = () => project.can(8)

async function loadTrash(): Promise<void> {
  loading.value = true
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

function openCreate(): void {
  Object.assign(form, {
    api_name: '', http_method_type: 'GET', url: '', protocol_type: 'HTTP',
    http_return_type: 'JSON', develop_language: '', description: '', group_id: 0,
  })
  headers.value = []
  reqParams.value = []
  resParams.value = []
  sample.value = ''
  void router.push({ name: 'projectApis', params: { projectId: projectId.value }, query: { new: '1' } })
}

function cancelCreate(): void {
  void router.replace({ name: 'projectApis', params: { projectId: projectId.value } })
}

function closeDetail(): void {
  void router.replace({ name: 'projectApis', params: { projectId: projectId.value } })
}

async function submit(): Promise<void> {
  if (!form.api_name.trim()) {
    message.warning('请输入接口名称')
    return
  }
  let sampleVal: unknown = ''
  if (sample.value.trim()) {
    try {
      sampleVal = JSON.parse(sample.value)
    } catch {
      message.error('响应样例需为合法 JSON')
      return
    }
  }
  saving.value = true
  try {
    const res = await createApi({
      ...form,
      project_id: projectId.value,
      http_request_header: headers.value,
      http_request_params: reqParams.value,
      http_return_params: resParams.value,
      http_return_sample: sampleVal,
    })
    if (res.code !== ErrCode.Success || !res.data) {
      message.error(res.msg || '创建失败')
      return
    }
    message.success('已创建')
    void treeRef.value?.reload()
    void router.push({ name: 'projectApis', params: { projectId: projectId.value }, query: { api: String(res.data.id) } })
  } finally {
    saving.value = false
  }
}

// ---- 导入 / 导出 ----
function onImportFiles(list: UploadFileInfo[]): void {
  importFile.value = list[0]?.file ?? null
}

/** 文本解析为 OpenAPI 对象：先按 JSON，再按 YAML（含 Bruno 反引号容错）。 */
function parseOpenApi(text: string): unknown {
  try {
    return JSON.parse(text)
  } catch {
    return parseYamlLoose(text)
  }
}

function repairBacktickScalars(text: string): string {
  return text.replace(/^(\s*(?:- )?[\w-]+:\s*)`([^`\n]*)`(\s*)$/gm, "$1'$2'$3")
}

function parseYamlLoose(text: string): unknown {
  try {
    return parseYaml(text)
  } catch {
    return parseYaml(repairBacktickScalars(text))
  }
}

async function doImport(): Promise<void> {
  if (!importFile.value) {
    message.warning('请先选择 OpenAPI 文件')
    return
  }
  let doc: unknown
  try {
    doc = parseOpenApi(await importFile.value.text())
  } catch {
    message.error('OpenAPI 文件解析失败（JSON / YAML 均无法识别）')
    return
  }
  const res = await importOpenApi(projectId.value, doc, 0)
  if (res.code !== ErrCode.Success) {
    message.error(res.msg || '导入失败')
    return
  }
  message.success(`已导入 ${res.data?.imported ?? 0} 个接口`)
  showImport.value = false
  importFile.value = null
  void treeRef.value?.reload()
}

async function doExport(): Promise<void> {
  const res = await exportOpenApi(projectId.value, 0)
  if (res.code !== ErrCode.Success || !res.data) {
    message.error(res.msg || '导出失败')
    return
  }
  const url = URL.createObjectURL(new Blob([JSON.stringify(res.data, null, 2)], { type: 'application/json' }))
  const a = document.createElement('a')
  a.href = url
  a.download = `openapi-project-${projectId.value}.json`
  a.click()
  URL.revokeObjectURL(url)
}

async function restore(a: ApiDoc): Promise<void> {
  const res = await restoreApi(a.id)
  if (res.code !== ErrCode.Success) message.error(res.msg || '恢复失败')
  void treeRef.value?.reload()
  await loadTrash()
}

function remove(a: ApiDoc): void {
  dialog.warning({
    title: '彻底删除',
    content: `确认删除回收站中的「${a.api_name}」？`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      const res = await deleteApi(a.id)
      if (res.code !== ErrCode.Success) message.error(res.msg || '删除失败')
      await loadTrash()
    },
  })
}

onMounted(() => {
  if (trash.value) void loadTrash()
})
</script>

<template>
  <div class="split">
    <!-- 新建/回收站态覆盖整个内容区：隐藏左侧树 -->
    <aside v-if="!creating && !trash" class="split-tree">
      <section-tree ref="treeRef" :project-id="projectId" :type="1" section="apis" />
    </aside>
    <!-- 导入/导出/回收站搜索：经 Teleport 注入到面包屑行右侧 -->
    <teleport to="#crumb-actions" defer>
      <template v-if="!creating">
        <n-button @click="showImport = true">导入 OpenAPI</n-button>
        <n-button @click="doExport">导出 OpenAPI</n-button>
        <template v-if="trash">
          <n-input
            v-model:value="keyword"
            placeholder="搜索回收站接口"
            clearable
            class="search"
            @keyup.enter="reload(1)"
            @clear="reload(1)"
          >
            <template #suffix><n-icon :component="SearchOutline" /></template>
          </n-input>
          <n-button @click="reload(1)">搜索</n-button>
        </template>
      </template>
    </teleport>

    <div class="split-main">
      <!-- 新建接口：覆盖整个内容区 -->
      <n-card v-if="creating" :bordered="true" class="pane-card">
        <template #header>
          <h2 class="section-title">新建接口</h2>
        </template>
        <template #header-extra>
          <div class="row-actions">
            <n-button size="small" @click="cancelCreate">取消</n-button>
            <n-button size="small" type="primary" :loading="saving" @click="submit">保存</n-button>
          </div>
        </template>
        <n-form label-placement="left" label-width="72" class="form-grid">
          <n-form-item label="名称"><n-input v-model:value="form.api_name" placeholder="接口名称" /></n-form-item>
          <n-form-item label="所属分组">
            <n-select v-model:value="form.group_id" :options="groupOptions" />
          </n-form-item>
          <n-form-item label="请求方法">
            <n-select v-model:value="form.http_method_type" :options="methodOptions" />
          </n-form-item>
          <n-form-item label="URL"><n-input v-model:value="form.url" placeholder="/api/xxx" /></n-form-item>
          <n-form-item label="请求协议"><n-input v-model:value="form.protocol_type" placeholder="HTTP" /></n-form-item>
          <n-form-item label="返回类型"><n-input v-model:value="form.http_return_type" placeholder="JSON" /></n-form-item>
          <n-form-item label="开发语言"><n-input v-model:value="form.develop_language" placeholder="如 go / php" /></n-form-item>
          <n-form-item label="说明" class="span-2">
            <n-input v-model:value="form.description" type="textarea" :rows="2" placeholder="可选" />
          </n-form-item>
        </n-form>
        <api-fields-editor
          v-model:headers="headers"
          v-model:req-params="reqParams"
          v-model:res-params="resParams"
          v-model:sample="sample"
        />
      </n-card>

      <!-- 回收站 -->
      <n-card v-else-if="trash" :bordered="true" class="pane-card table-card" content-style="padding: 0">
        <n-spin :show="loading">
          <n-empty v-if="!rows.length && !loading" class="empty-block" description="回收站为空" />
          <div v-else class="table-scroll">
            <n-table :single-line="false" size="medium" class="wide">
              <thead>
                <tr>
                  <th style="width: 24%">名称</th>
                  <th style="width: 96px">请求方法</th>
                  <th style="width: 30%">URL</th>
                  <th style="width: 168px">创建时间</th>
                  <th style="width: 120px">操作</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="a in rows" :key="a.id">
                  <td>{{ a.api_name || '未命名接口' }}</td>
                  <td><method-tag :method="a.http_method_type" /></td>
                  <td class="row-mono ellipsis" :title="a.url">{{ a.url || '—' }}</td>
                  <td class="muted nowrap">{{ fmtTime(a.create_time) }}</td>
                  <td>
                    <div class="row-actions">
                      <n-button text type="primary" size="small" @click="restore(a)">恢复</n-button>
                      <n-button v-if="canDelete()" text type="error" size="small" @click="remove(a)">删除</n-button>
                    </div>
                  </td>
                </tr>
              </tbody>
            </n-table>
          </div>
        </n-spin>
        <table-pager :total="total" :page-num="pageNum" :page-size="pageSize" @update:page-num="reload" />
      </n-card>

      <!-- 接口详情（查看/编辑） -->
      <api-detail-view
        v-else-if="apiId"
        :key="apiId"
        :api-id="apiId"
        embedded
        @close="closeDetail"
        @changed="treeRef?.reload()"
      />

      <!-- 空态占位：Postman 式引导 -->
      <empty-hint
        v-else
        :icon="SendOutline"
        title="从左侧接口集合中选择一个接口查看详情"
        :tips="[
          { label: '查看详情', value: '点击接口集合中的接口' },
          { label: '新建接口', value: '点击右上角「创建接口」' },
          { label: '批量导入', value: '「导入 OpenAPI」支持 JSON / YAML' },
        ]"
      >
        <n-button v-if="canCreate()" type="primary" @click="openCreate">创建接口</n-button>
      </empty-hint>
    </div>

    <n-modal v-model:show="showImport" preset="card" title="导入 OpenAPI（选择 JSON / YAML 文件）" style="width: 520px">
      <n-upload
        :max="1"
        :default-upload="false"
        accept=".json,.yaml,.yml"
        @update:file-list="onImportFiles"
      >
        <n-button>选择文件</n-button>
      </n-upload>
      <template #footer>
        <div class="foot">
          <n-button size="small" @click="showImport = false">取消</n-button>
          <n-button type="primary" size="small" @click="doImport">导入</n-button>
        </div>
      </template>
    </n-modal>
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

.foot {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
</style>
