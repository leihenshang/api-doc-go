<script setup lang="ts">
// 免登录只读分享页（/share/:token）：左侧目录树 + 右侧接口/文档详情，无任何编辑入口。
// 数据来自 GET /api/share/:token（公开接口；令牌无效/过期时给出提示页）。
import { getSharePayload } from '@/api/share'
import ApiParamTable from '@/components/ApiParamTable.vue'
import JsonBlock from '@/components/JsonBlock.vue'
import MethodTag from '@/components/MethodTag.vue'
import RepoMark from '@/components/RepoMark.vue'
import { ErrCode } from '@/types/api'
import type { ApiDoc, ApiHeader, ApiParam, DocItem, ShareView } from '@/types/entities'
import { headersToParams } from '@/utils/api-params'
import { DEFAULT_DOC_ICON, isImageIcon } from '@/utils/icon'
import { NEmpty, NSpin } from 'naive-ui'
import { MdPreview } from 'md-editor-v3'
import 'md-editor-v3/lib/preview.css'
import 'md-editor-v3/lib/style.css'
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'

/** 目录树扁平行：分组标题行与叶子行按层级顺序混排。 */
type TreeRow =
    | { kind: 'group'; id: number; title: string; level: number }
    | { kind: 'api'; api: ApiDoc }
    | { kind: 'doc'; doc: DocItem }

const route = useRoute()

const loading = ref(true)
const error = ref('')
const data = ref<ShareView | null>(null)
const keyword = ref('')
const currentApi = ref<ApiDoc | null>(null)
const currentDoc = ref<DocItem | null>(null)

const token = computed(() => String(route.params.token ?? ''))

const filteredApis = computed<ApiDoc[]>(() => {
    const list = data.value?.apis ?? []
    const kw = keyword.value.trim().toLowerCase()
    return kw ? list.filter((a) => `${a.api_name} ${a.url}`.toLowerCase().includes(kw)) : list
})

const filteredDocs = computed<DocItem[]>(() => {
    const list = data.value?.docs ?? []
    const kw = keyword.value.trim().toLowerCase()
    return kw ? list.filter((d) => `${d.title} ${d.content}`.toLowerCase().includes(kw)) : list
})

/** 接口/文档按分组归集（未知分组并入 0 = 未分组）。 */
function bucket<T extends { group_id: number }>(items: T[]): Map<number, T[]> {
    const ids = new Set((data.value?.groups ?? []).map((g) => g.id))
    const map = new Map<number, T[]>()
    for (const it of items) {
        const gid = ids.has(it.group_id) ? it.group_id : 0
        const list = map.get(gid) ?? []
        list.push(it)
        map.set(gid, list)
    }
    return map
}

const apisByGroup = computed(() => bucket(filteredApis.value))
const docsByGroup = computed(() => bucket(filteredDocs.value))

/** 目录树：分组行 + 该分组下的接口/文档行（空分组不渲染；两者都没有时收成「未分组」）。 */
const rows = computed<TreeRow[]>(() => {
    const out: TreeRow[] = []
    const groups = data.value?.groups ?? []
    const children = (pid: number) => groups.filter((g) => g.p_id === pid)
    const walk = (pid: number, level: number): void => {
        for (const g of children(pid)) {
            const apis = apisByGroup.value.get(g.id) ?? []
            const docs = docsByGroup.value.get(g.id) ?? []
            const kids = children(g.id)
            if (!apis.length && !docs.length && !kids.length) continue
            out.push({ kind: 'group', id: g.id, title: g.title, level })
            for (const a of apis) out.push({ kind: 'api', api: a })
            for (const d of docs) out.push({ kind: 'doc', doc: d })
            walk(g.id, level + 1)
        }
    }
    walk(0, 1)
    const looseApis = apisByGroup.value.get(0) ?? []
    const looseDocs = docsByGroup.value.get(0) ?? []
    if (looseApis.length || looseDocs.length) {
        out.push({ kind: 'group', id: 0, title: '未分组', level: 1 })
        for (const a of looseApis) out.push({ kind: 'api', api: a })
        for (const d of looseDocs) out.push({ kind: 'doc', doc: d })
    }
    return out
})

function headersOf(a: ApiDoc): ApiParam[] {
    try {
        return headersToParams(JSON.parse(a.http_request_header || '[]') as ApiHeader[])
    } catch {
        return []
    }
}

function paramsOf(raw: string): ApiParam[] {
    try {
        return JSON.parse(raw || '[]') as ApiParam[]
    } catch {
        return []
    }
}

function selectApi(a: ApiDoc): void {
    currentApi.value = a
    currentDoc.value = null
}

function selectDoc(d: DocItem): void {
    currentDoc.value = d
    currentApi.value = null
}

onMounted(async () => {
    try {
        const res = await getSharePayload(token.value)
        if (res.code !== ErrCode.Success || !res.data) {
            error.value = res.msg || '分享链接无效'
            return
        }
        data.value = res.data
        const first = res.data.apis[0]
        if (first) selectApi(first)
        else if (res.data.docs[0]) selectDoc(res.data.docs[0])
    } finally {
        loading.value = false
    }
})
</script>

<template>
    <div class="share">
        <header class="sh-hd">
            <div class="sh-brand">
                <span class="mark"><repo-mark /></span>
                <span class="name">API-DOC-GO</span>
                <span class="badge">只读分享</span>
            </div>
            <div v-if="data" class="sh-title">
                <b class="ellipsis">{{ data.project.title }}</b>
                <span v-if="data.project.version" class="meta">{{ data.project.version }}</span>
                <span v-if="data.remark" class="meta">{{ data.remark }}</span>
            </div>
            <input v-if="data" v-model="keyword" class="sh-search" type="search" placeholder="搜索接口 / 文档" />
        </header>

        <n-spin :show="loading" class="sh-spin">
            <n-empty v-if="error" size="large" :description="error" class="sh-empty" />
            <div v-else-if="data" class="sh-body">
                <aside class="sh-tree">
                    <template v-for="(row, i) in rows" :key="i">
                        <div
                            v-if="row.kind === 'group'"
                            class="sh-group"
                            :style="{ paddingLeft: 8 + (row.level - 1) * 12 + 'px' }"
                        >
                            <span class="gname ellipsis" :title="row.title">{{ row.title }}</span>
                        </div>
                        <button
                            v-else-if="row.kind === 'api'"
                            class="sh-leaf"
                            :class="{ 'sh-leaf--active': currentApi?.id === row.api.id }"
                            :title="row.api.api_name"
                            @click="selectApi(row.api)"
                        >
                            <method-tag :method="row.api.http_method_type" />
                            <span class="ellipsis">{{ row.api.api_name }}</span>
                        </button>
                        <button
                            v-else
                            class="sh-leaf"
                            :class="{ 'sh-leaf--active': currentDoc?.id === row.doc.id }"
                            :title="row.doc.title"
                            @click="selectDoc(row.doc)"
                        >
                            <img v-if="isImageIcon(row.doc.icon)" class="doc-icon-img" :src="row.doc.icon" alt="" />
                            <span v-else class="doc-icon">{{ row.doc.icon || DEFAULT_DOC_ICON }}</span>
                            <span class="ellipsis">{{ row.doc.title }}</span>
                        </button>
                    </template>
                    <p v-if="!rows.length" class="meta sh-none">无匹配内容</p>
                </aside>

                <main class="sh-main">
                    <template v-if="currentApi">
                        <div class="api-hd">
                            <method-tag :method="currentApi.http_method_type" />
                            <h1 class="api-name">{{ currentApi.api_name }}</h1>
                            <span class="meta">{{ currentApi.protocol_type || 'HTTP' }}</span>
                        </div>
                        <div class="api-url row-mono">{{ currentApi.url }}</div>

                        <section class="sec">
                            <api-param-table :model-value="headersOf(currentApi)" kind="header" />
                        </section>
                        <section class="sec">
                            <api-param-table :model-value="paramsOf(currentApi.http_request_params)" kind="request" />
                        </section>
                        <section class="sec">
                            <api-param-table
                                :model-value="paramsOf(currentApi.http_return_params)"
                                kind="response"
                                :title-extra="currentApi.http_return_type"
                            />
                        </section>
                        <section class="sec">
                            <div class="sec-hd">响应样例</div>
                            <json-block :raw="currentApi.http_return_sample" empty="未配置响应样例" />
                        </section>
                        <section v-if="currentApi.description" class="sec">
                            <div class="sec-hd">文档</div>
                            <md-preview :model-value="currentApi.description" preview-theme="github" />
                        </section>
                    </template>

                    <template v-else-if="currentDoc">
                        <div class="api-hd">
                            <span class="doc-icon doc-icon--lg">{{ currentDoc.icon || '📄' }}</span>
                            <h1 class="api-name">{{ currentDoc.title }}</h1>
                        </div>
                        <section class="sec">
                            <md-preview :model-value="currentDoc.content" preview-theme="github" />
                        </section>
                    </template>

                    <n-empty v-else description="从左侧选择一个接口或文档" class="sh-empty" />
                </main>
            </div>
        </n-spin>
    </div>
</template>

<style scoped>
.share {
    display: flex;
    flex-direction: column;
    height: 100vh;
    background: #fff;
}

.sh-hd {
    flex: 0 0 auto;
    display: flex;
    align-items: center;
    gap: 16px;
    padding: 10px 16px;
    border-bottom: 1px solid var(--app-border);
}

.sh-brand {
    display: inline-flex;
    align-items: center;
    gap: 8px;
}

.mark {
    display: inline-flex;
    width: 18px;
    height: 18px;
    color: var(--app-accent);
}

.name {
    font-size: 15px;
    font-weight: 700;
}

.badge {
    padding: 0 6px;
    border-radius: 3px;
    background: #eef7f1;
    color: var(--app-accent);
    font-size: 12px;
}

.sh-title {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    min-width: 0;
}

.sh-search {
    margin-left: auto;
    width: 220px;
    padding: 5px 10px;
    border: 1px solid var(--app-border);
    border-radius: 4px;
    font-size: 13px;
}

.sh-spin {
    flex: 1 1 auto;
    min-height: 0;
    display: flex;
    flex-direction: column;
}

/* naive-ui spin 的内容层需显式撑满，内部两栏才拿得到高度 */
.sh-spin :deep(.n-spin-content) {
    flex: 1 1 auto;
    min-height: 0;
    display: flex;
    flex-direction: column;
}

.sh-body {
    flex: 1 1 auto;
    min-height: 0;
    display: flex;
}

.sh-tree {
    flex: 0 0 268px;
    width: 268px;
    min-height: 0;
    overflow: auto;
    padding: 8px 0;
    border-right: 1px solid var(--app-border);
    background: #fbfbfc;
}

.sh-group {
    display: flex;
    align-items: center;
    gap: 6px;
    height: 26px;
    padding-right: 8px;
    color: var(--app-muted);
    font-size: 12px;
    font-weight: 600;
}

.sh-leaf {
    display: flex;
    align-items: center;
    gap: 6px;
    width: 100%;
    height: 30px;
    padding: 0 10px 0 22px;
    border: 0;
    background: none;
    font-size: 13px;
    text-align: left;
    cursor: pointer;
}

.sh-leaf:hover {
    background: #f2f3f5;
}

.sh-leaf--active {
    background: rgba(24, 160, 88, 0.1);
    color: var(--app-accent);
}

.sh-none {
    padding: 10px 14px;
}

.sh-main {
    flex: 1 1 auto;
    min-width: 0;
    min-height: 0;
    overflow: auto;
    padding: 16px 20px 40px;
}

.api-hd {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-bottom: 10px;
}

.api-name {
    margin: 0;
    font-size: 17px;
}

.api-url {
    margin-bottom: 16px;
    padding: 8px 10px;
    border: 1px solid var(--app-border);
    border-radius: 4px;
    background: #fafbfc;
    font-size: 13px;
    word-break: break-all;
}

.sec + .sec {
    margin-top: 20px;
    padding-top: 18px;
    border-top: 1px solid var(--app-border);
}

.sec-hd {
    margin-bottom: 10px;
    font-size: 13px;
    font-weight: 600;
}

.doc-icon {
    flex: 0 0 auto;
}

.doc-icon--lg {
    font-size: 20px;
}

.doc-icon-img {
    flex: 0 0 auto;
    width: 16px;
    height: 16px;
    border-radius: 3px;
    object-fit: cover;
}

.doc-icon-img--lg {
    width: 22px;
    height: 22px;
}

.sh-empty {
    margin: 60px auto 0;
}

/* 窄屏：顶栏换行、两栏改上下 */
@media (max-width: 768px) {
    .sh-hd {
        flex-wrap: wrap;
        gap: 8px;
    }

    .sh-search {
        width: 100%;
        margin-left: 0;
    }

    .sh-body {
        flex-direction: column;
    }

    .sh-tree {
        flex: 0 0 auto;
        width: 100%;
        max-height: 40vh;
        border-right: 0;
        border-bottom: 1px solid var(--app-border);
    }

    .sh-main {
        padding: 12px 14px 32px;
    }
}
</style>
