// 顶部横向 tab 与面包屑：把原左侧菜单改造成「API-DOC-GO」下方的一排 tab。
// 项目内 tab = 概况/API接口/项目文档/字段映射/Bug/数据字典/操作日志，右侧另有「回收站」。
// 分组与回收站经路由 query（group / trash）表达，列表页据此取数；分组树在页面内左侧展示。
import { useProjectStore } from '@/stores/project'
import { useUserStore } from '@/stores/user'
import {
  BookOutline,
  BugOutline,
  CubeOutline,
  GitCompareOutline,
  GridOutline,
  HomeOutline,
  ListOutline,
  OptionsOutline,
  PeopleOutline,
  PulseOutline
} from '@vicons/ionicons5'
import type { Component } from 'vue'
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'

export type Crumb = { label: string; to?: { name: string; params?: Record<string, number> } }

/** 横向 tab 项：key 为路由名（回收站用保留字 'trash'）。 */
export type TabItem = { key: string; label: string; icon: Component }

const API_TABS: TabItem[] = [
  { key: 'projectOverview', label: '项目概况', icon: HomeOutline },
  { key: 'projectApis', label: 'API接口', icon: PulseOutline },
  { key: 'projectDocs', label: '项目文档', icon: BookOutline },
  { key: 'projectFields', label: '字段映射', icon: GitCompareOutline },
  { key: 'projectBugs', label: 'Bug 管理', icon: BugOutline },
  { key: 'projectDict', label: '数据字典', icon: CubeOutline },
  { key: 'projectLogs', label: '操作日志', icon: ListOutline },
]

export function useProjectNav() {
  const route = useRoute()
  const router = useRouter()
  const user = useUserStore()
  const project = useProjectStore()

  const projectId = computed(() => Number(route.params.projectId) || 0)
  const inProject = computed(() => projectId.value > 0)

  /** 当前所属分区（用于回收站跳转与高亮）。 */
  const sectionKind = computed<'apis' | 'docs' | ''>(() => {
    if (route.name === 'projectApis') return 'apis'
    if (route.name === 'projectDocs') return 'docs'
    return ''
  })

  const trashActive = computed(
    () => (route.name === 'projectApis' || route.name === 'projectDocs') && route.query.trash === '1',
  )

  const globalTabs = computed<TabItem[]>(() => [
    { key: 'projects', label: '项目列表', icon: GridOutline },
    { key: 'users', label: '用户管理', icon: PeopleOutline },
    ...(user.isAdmin ? [{ key: 'properties', label: '公共属性', icon: OptionsOutline } as TabItem] : []),
  ])

  const tabs = computed<TabItem[]>(() =>
    inProject.value ? API_TABS : globalTabs.value,
  )

  /** 高亮项：回收站态下高亮 trash。 */
  const activeTab = computed(() => {
    if (trashActive.value) return 'trash'
    return String(route.name ?? '')
  })

  /** tab 点击：trash 走当前分区回收站；其余按路由名跳转并清空 query（回到「全部」）。 */
  function onTab(key: string): void {
    if (key === 'trash') {
      const name = sectionKind.value === 'docs' ? 'projectDocs' : 'projectApis'
      void router.push({ name, params: { projectId: projectId.value }, query: { trash: '1' } })
      return
    }
    if (key === String(route.name) && !route.query.group && !route.query.trash) return
    const params = key.startsWith('project') ? { projectId: projectId.value } : undefined
    void router.push({ name: key, params })
  }

  /** 切换当前项目：默认进入 API接口（分组树为其左侧面板）。 */
  function switchProject(id: number): void {
    if (!id || id === projectId.value) return
    void router.push({ name: 'projectApis', params: { projectId: id } })
  }

  const groupTitle = (kind: 'apis' | 'docs', id: number): string | undefined => {
    const list = kind === 'apis' ? project.apiGroups : project.docGroups
    return list.find((g) => g.id === id)?.title
  }

  const crumbs = computed<Crumb[]>(() => {
    const list: Crumb[] = [{ label: '首页', to: { name: 'projects' } }]
    if (!inProject.value) {
      if (route.meta.title && route.meta.title !== '项目列表') list.push({ label: route.meta.title })
      return list
    }
    list.push({
      label: project.title || '项目详情',
      to: { name: 'projectOverview', params: { projectId: projectId.value } },
    })
    const kind = sectionKind.value
    if (kind) {
      list.push({
        label: kind === 'apis' ? 'API接口' : '项目文档',
        to: { name: kind === 'apis' ? 'projectApis' : 'projectDocs', params: { projectId: projectId.value } },
      })
      if (trashActive.value) list.push({ label: '回收站' })
      else if (kind === 'apis' && route.query.api) list.push({ label: '接口详情' })
      else if (kind === 'docs' && route.query.doc) list.push({ label: '文档详情' })
      else {
        const name = groupTitle(kind, Number(route.query.group) || 0)
        if (name) list.push({ label: name })
      }
    } else if (route.meta.title && route.name !== 'projectOverview') {
      list.push({ label: route.meta.title })
    }
    return list
  })

  return { projectId, inProject, tabs, activeTab, trashActive, onTab, switchProject, crumbs }
}
