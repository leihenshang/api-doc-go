// 混排工作台（section=all）的拖拽排序：接口与文档同在一棵树里，拖完要按
// `kind:id` 复合键算出新顺序，并以 kind=mixed + 逐条 kind 提交给服务端。
//
// 为什么不用真鼠标拖：HTML5 drag 在 happy-dom 里没有真实指针轨迹，
// 直接派发 dragstart → dragover → drop 三步就能覆盖组件里的全部判断
//（落点推算、复合键搬迁、payload 组装），比 dragTo 稳定。

import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const listApis = vi.fn()
const listDocs = vi.fn()
const sortNodes = vi.fn()

vi.mock('@/api/content', () => ({
    createGroup: vi.fn(),
    deleteApi: vi.fn(),
    deleteDoc: vi.fn(),
    deleteGroup: vi.fn(),
    listApis: (...a: unknown[]) => listApis(...a),
    listDocs: (...a: unknown[]) => listDocs(...a),
    sortNodes: (...a: unknown[]) => sortNodes(...a),
    updateGroup: vi.fn(),
}))

const GROUP = { id: 16, p_id: 0, project_id: 6, title: '公共', priority: 0, type: 1, create_time: '', update_time: '' }

vi.mock('@/stores/project', () => ({
    useProjectStore: () => ({
        allGroups: [GROUP],
        apiGroups: [GROUP],
        docGroups: [],
        docGroupCounts: {},
        ungroupedDocs: 0,
        can: () => true,
        loadGroups: vi.fn(),
    }),
}))
vi.mock('vue-router', () => ({
    useRoute: () => ({ params: { projectId: '6' }, query: {} }),
    useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
}))
vi.mock('vue-i18n', async (importOriginal) => {
    const actual = await importOriginal<Record<string, unknown>>()
    return {
        ...actual,
        useI18n: () => ({ t: (k: string) => k }),
    }
})
vi.mock('naive-ui', async (importOriginal) => {
    const actual = await importOriginal<Record<string, unknown>>()
    return {
        ...actual,
        useDialog: () => ({}),
        useMessage: () => ({ error: vi.fn(), warning: vi.fn(), success: vi.fn() }),
    }
})

import SectionTree from './SectionTree.vue'

/** 同组下的一个接口（id=69）与一篇文档（id=11）：主键刻意不同，靠 kind 区分。 */
function apiPage() {
    return { code: 200, data: { items: [{ id: 69, group_id: 16, api_name: '登录接口', http_method_type: 'POST', url: '/login' }], total: 1 } }
}
function docPage() {
    return { code: 200, data: { items: [{ id: 11, group_id: 16, title: '同步验证文档', icon: '' }], total: 1 } }
}

async function mountTree() {
    listApis.mockResolvedValue(apiPage())
    listDocs.mockResolvedValue(docPage())
    sortNodes.mockResolvedValue({ code: 200, msg: 'success', data: null })
    const w = mount(SectionTree, {
        props: { projectId: 6, type: 1, section: 'all' },
        global: { stubs: { MethodTag: true } },
    })
    await flushPromises()
    await flushPromises()
    return w
}

function dt() {
    return { setData: () => {}, effectAllowed: '', dropEffect: '' }
}

describe('混排树的拖拽排序', () => {
    beforeEach(() => {
        listApis.mockReset()
        listDocs.mockReset()
        sortNodes.mockReset()
    })

    it('接口与文档在同一棵树里按 kind 区分', async () => {
        const w = await mountTree()
        const leaves = w.findAll('.gn--leaf')
        expect(leaves.length).toBe(2)
        expect(leaves[0].text()).toContain('登录接口')
        expect(leaves[1].text()).toContain('同步验证文档')
    })

    it('把接口拖到文档后面：提交 kind=mixed，逐条带自己的 kind', async () => {
        const w = await mountTree()
        const leaves = w.findAll('.gn--leaf')
        const apiRow = leaves[0]
        const docRow = leaves[1]

        await apiRow.trigger('dragstart', { dataTransfer: dt() })
        // clientY 远大于行高（happy-dom 里 offsetHeight 为 0 时按 1 处理）→ 落在下方 = after
        await docRow.trigger('dragover', { dataTransfer: dt(), clientY: 100 })
        await docRow.trigger('drop', { dataTransfer: dt(), clientY: 100 })
        await flushPromises()

        expect(sortNodes).toHaveBeenCalledTimes(1)
        const payload = sortNodes.mock.calls[0][0] as {
            kind: string
            project_id: number
            items: { id: number; priority: number; kind?: string }[]
        }
        expect(payload.kind).toBe('mixed')
        expect(payload.project_id).toBe(6)
        // 文档保持第 0（目标行原位），接口被插到它后面变成第 1
        expect(payload.items).toEqual([
            { id: 11, parent_id: 16, priority: 0, kind: 'doc' },
            { id: 69, parent_id: 16, priority: 1, kind: 'api' },
        ])
    })

    it('把文档拖到接口前面：文档变成第 0', async () => {
        const w = await mountTree()
        const leaves = w.findAll('.gn--leaf')
        const docRow = leaves[1]
        const apiRow = leaves[0]

        await docRow.trigger('dragstart', { dataTransfer: dt() })
        // clientY=0 → 落点在行内上半部分 = before（文档插到接口之前）
        await apiRow.trigger('dragover', { dataTransfer: dt(), clientY: 0 })
        await apiRow.trigger('drop', { dataTransfer: dt(), clientY: 0 })
        await flushPromises()

        expect(sortNodes).toHaveBeenCalledTimes(1)
        const payload = sortNodes.mock.calls[0][0] as { items: { id: number; priority: number; kind?: string }[] }
        // 初始顺序是 [api, doc]；只有真的移动过才会变成 [doc, api]
        expect(payload.items).toEqual([
            { id: 11, parent_id: 16, priority: 0, kind: 'doc' },
            { id: 69, parent_id: 16, priority: 1, kind: 'api' },
        ])
    })
})
