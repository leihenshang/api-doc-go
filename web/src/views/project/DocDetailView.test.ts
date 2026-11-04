// 项目文档（doc 实体）的 markdown 渲染测试。
//
// 与被删掉的「纯 markdown-it 渲染」测试不同：这里挂载真实的 DocDetailView 组件，
// 让数据走完整链路 —— getDoc(id) → doc.content → form.content → previewContent
// → <md-preview>（md-editor-v3，页面同款渲染器）。
//
// 测的就是「类型为 doc 的独立文档」：接口详情里的 description 是另一条数据
// （api.description → ApiDetailView），本文件不涉及，且显式断言两者不互相串入。

import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'

const getDoc = vi.fn()

vi.mock('@/api/content', () => ({
    getDoc: (...a: unknown[]) => getDoc(...a),
    updateDoc: vi.fn(),
    deleteDoc: vi.fn(),
    likeDoc: vi.fn(),
}))
vi.mock('@/api/misc', () => ({ uploadImg: vi.fn() }))
vi.mock('@/composables/userNames', () => ({
    useUserNames: () => ({ ensure: async () => {}, nameOf: () => 'admin' }),
}))
vi.mock('@/stores/project', () => ({
    useProjectStore: () => ({ docGroups: [], can: () => false }),
}))
vi.mock('vue-router', () => ({
    useRoute: () => ({ params: { projectId: '6' }, query: {} }),
    useRouter: () => ({ replace: vi.fn(), back: vi.fn() }),
}))
// i18n 需要 app.use(i18n) 才能用 useI18n，组件测试里直接给个只会回显 key 的实现。
// 保留 actual 的 createI18n：src/i18n/index.ts 会被 md-upload 间接导入并创建实例。
vi.mock('vue-i18n', async (importOriginal) => {
    const actual = await importOriginal<Record<string, unknown>>()
    return {
        ...actual,
        useI18n: () => ({
            t: (k: string, p?: Record<string, unknown>) => (p ? `${k} ${JSON.stringify(p)}` : k),
        }),
    }
})
// naive-ui 的 useDialog/useMessage 依赖 provider，测试里替换成空实现；
// 其余组件（NCard/NButton/...）保持真实，避免用 stub 假造出「渲染成功」。
vi.mock('naive-ui', async (importOriginal) => {
    const actual = await importOriginal<Record<string, unknown>>()
    return {
        ...actual,
        useDialog: () => ({ warning: vi.fn(), error: vi.fn() }),
        useMessage: () => ({ error: vi.fn(), success: vi.fn(), warning: vi.fn() }),
    }
})
// md-editor-v3 的预览组件会向外网 CDN 拉取 katex/highlight 资源，测试环境必然超时；
// 这里换成**同款渲染内核 markdown-it** 的替身（md-editor-v3 内部也是它），
// 测的仍是完整链路：getDoc → doc.content → previewContent → 渲染。
vi.mock('md-editor-v3', async () => {
    const MarkdownIt = (await import('markdown-it')).default
    const md = new MarkdownIt({ html: true, linkify: true, breaks: true })
    return {
        // 不写 id：模板上的 id="doc-preview" 会作为 attr 继承到根元素。
        MdPreview: {
            props: { modelValue: { type: String, default: '' } },
            computed: {
                html(this: { modelValue: string }) {
                    return md.render(this.modelValue)
                },
            },
            template: '<div v-html="html"></div>',
        },
        MdEditor: { props: { modelValue: { type: String, default: '' } }, template: '<textarea></textarea>' },
    }
})

import DocDetailView from './DocDetailView.vue'

/** 一篇独立的 markdown 文档（覆盖常见语法）。 */
const DOC_MD = `# 渲染验证文档

这是一段确认 **加粗**、*斜体*、\`行内代码\` 与 [外链](https://example.com) 的正文。

## 二级标题

- 列表项一
- 列表项二，含 \`code\` 片段

1. 有序第一
2. 有序第二

> 引用块

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| userId | string | 用户标识 |

\`\`\`go
func Hello(name string) string { return "hi" }
\`\`\`

---

结尾段落。
`

function docEntity(content: string, title = '渲染验证文档') {
    return {
        id: 7, project_id: 6, group_id: 0, title, content, icon: '',
        user_id: 1, update_time: '2026-11-04 14:58:12', view_count: 1, like_count: 0, is_top: 0,
    }
}

async function mountDoc(content: string, title?: string) {
    // ErrCode.Success 是 200（不是 0）：用错会让 load() 走失败分支、doc 保持 null，
    // 测试就会在「页面标题为 —」的状态下空转，看着像渲染坏了。
    getDoc.mockResolvedValue({ code: 200, msg: 'success', data: docEntity(content, title) })
    const w = mount(DocDetailView, {
        props: { docId: 7, embedded: true },
        global: { stubs: { RevisionHistory: true } },
    })
    await flushPromises()
    await nextTick()
    await nextTick()
    return w
}

describe('项目文档（doc 实体）的 markdown 渲染', () => {
    beforeEach(() => {
        getDoc.mockReset()
    })

    it('数据来自 doc 实体接口，并用该 id 拉取', async () => {
        const w = await mountDoc(DOC_MD)
        expect(getDoc).toHaveBeenCalledWith(7)
        // 页面标题由 <h1 class="doc-title"> 承担
        expect(w.find('.doc-title').text()).toContain('渲染验证文档')
    })

    it('独立文档的各类 markdown 结构都渲染成对应标签', async () => {
        const w = await mountDoc(DOC_MD)
        const pv = w.find('#doc-preview')
        expect(pv.exists()).toBe(true)

        expect(pv.findAll('h2').length).toBeGreaterThan(0)
        expect(pv.findAll('ul li').length).toBeGreaterThanOrEqual(2)
        expect(pv.findAll('ol li').length).toBeGreaterThanOrEqual(2)
        expect(pv.find('blockquote').exists()).toBe(true)
        expect(pv.find('table').exists()).toBe(true)
        expect(pv.find('table th').exists()).toBe(true)
        expect(pv.find('table td').text()).toContain('userId')
        expect(pv.find('pre code').exists()).toBe(true)
        expect(pv.find('pre code').text()).toContain('func Hello')
        expect(pv.find('strong').exists()).toBe(true)
        expect(pv.find('em').exists()).toBe(true)
        expect(pv.find('a[href="https://example.com"]').exists()).toBe(true)
        expect(pv.find('hr').exists()).toBe(true)
    })

    it('正文里与标题重复的 H1 被去掉，其余内容不丢', async () => {
        const w = await mountDoc(DOC_MD)
        const pv = w.find('#doc-preview')
        // 页面已有 .doc-title 显示标题，正文不应再出现重复 H1
        expect(pv.findAll('h1').length).toBe(0)
        expect(pv.text()).toContain('这是一段确认')
        expect(pv.text()).toContain('结尾段落')
    })

    it('没有重复 H1 的文档不会被误删内容', async () => {
        const bodyOnly = '## 只有二级标题\n\n正文段落，**加粗**照常渲染。'
        const w = await mountDoc(bodyOnly, '另一篇文档')
        const pv = w.find('#doc-preview')
        expect(pv.find('h2').text()).toContain('只有二级标题')
        expect(pv.find('strong').exists()).toBe(true)
    })

    it('接口说明（api.description）不会被渲染进项目文档页', async () => {
        const w = await mountDoc(DOC_MD)
        expect(w.text()).not.toContain('这是接口说明')
        expect(w.find('#doc-preview').text()).not.toContain('接口的说明字段')
    })
})
