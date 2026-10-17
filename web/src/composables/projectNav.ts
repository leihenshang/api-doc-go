// 顶部横向 tab：把原左侧菜单改造成「API-DOC-GO」下方的一排 tab。
// 项目内 tab = 概况/API接口/项目文档/字段映射/变量/备忘清单，右侧另有「回收站」。
// 分组与回收站经路由 query（group / trash）表达，列表页据此取数；分组树在页面内左侧展示。
import { useUserStore } from '@/stores/user'
import {
  BookOutline,
  BugOutline,
  CodeSlashOutline,
  GitCompareOutline,
  GridOutline,
  HomeOutline,
  OptionsOutline,
  PeopleOutline,
  PulseOutline
} from '@vicons/ionicons5'
import type { Component } from 'vue'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'


/** 横向 tab 项：key 为路由名（回收站用保留字 'trash'），label 为按当前语言解析后的文案。 */
export type TabItem = { key: string; label: string; icon: Component }

/** 页签定义：存 labelKey 而非文案，切语言时由 tabs 重新解析；adminOnly 仅管理员可见。 */
type TabDef = { key: string; labelKey: string; icon: Component; adminOnly?: boolean }

const API_TABS: TabDef[] = [
  { key: 'projectOverview', labelKey: 'nav.overview', icon: HomeOutline },
  { key: 'projectApis', labelKey: 'nav.apis', icon: PulseOutline },
  { key: 'projectDocs', labelKey: 'nav.docs', icon: BookOutline },
  { key: 'projectFields', labelKey: 'nav.fields', icon: GitCompareOutline },
  { key: 'projectVariables', labelKey: 'nav.variables', icon: CodeSlashOutline },
  { key: 'projectBugs', labelKey: 'nav.bugs', icon: BugOutline },
]

/** 全局页签（非项目内）：用户管理与公共属性，两者均仅管理员可见。 */
const GLOBAL_TABS: TabDef[] = [
  { key: 'projects', labelKey: 'nav.projectList', icon: GridOutline },
  { key: 'users', labelKey: 'nav.users', icon: PeopleOutline, adminOnly: true },
  { key: 'properties', labelKey: 'nav.properties', icon: OptionsOutline, adminOnly: true },
]

export function useProjectNav() {
  const route = useRoute()
  const router = useRouter()
  const user = useUserStore()
  const { t } = useI18n()

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

  /** 按当前语言解析页签文案（切语言时 computed 自动重算）。 */
  function resolve(defs: TabDef[]): TabItem[] {
    return defs.map((d) => ({ key: d.key, label: t(d.labelKey), icon: d.icon }))
  }

  const apiTabs = computed<TabItem[]>(() => resolve(API_TABS))

  const globalTabs = computed<TabItem[]>(() =>
    resolve(GLOBAL_TABS.filter((d) => !d.adminOnly || user.isAdmin)),
  )

  const tabs = computed<TabItem[]>(() =>
    inProject.value ? apiTabs.value : globalTabs.value,
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

  return { projectId, inProject, tabs, activeTab, trashActive, onTab, switchProject }
}
