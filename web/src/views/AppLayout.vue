<script setup lang="ts">
// 应用外壳：顶栏（品牌 + 通知铃铛 + 账户浮层）+ 横向 tab 导航（含项目切换与回收站）
// + 面包屑（左侧圆形返回按钮）。tab/面包屑/回收站逻辑见 composables/projectNav.ts。
import NotificationBell from '@/components/NotificationBell.vue'
import RepoMark from '@/components/RepoMark.vue'
import UserAccountPanel from '@/components/UserAccountPanel.vue'
import { useProjectNav } from '@/composables/projectNav'
import { useProjectStore } from '@/stores/project'
import { useUserStore } from '@/stores/user'
import { ArrowBackOutline, TrashOutline } from '@vicons/ionicons5'
import {
    NAvatar,
    NBreadcrumb,
    NBreadcrumbItem,
    NButton,
    NIcon,
    NPopover,
    NSelect,
    NSpace,
    useMessage,
} from 'naive-ui'
import { computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

const route = useRoute()
const router = useRouter()
const message = useMessage()
const user = useUserStore()
const project = useProjectStore()

const { projectId, inProject, tabs, activeTab, trashActive, onTab, switchProject, crumbs } = useProjectNav()

const projectOptions = computed(() => project.projects.map((p) => ({ label: p.title, value: p.id })))
const initials = computed(() => user.nickName.slice(0, 1).toUpperCase() || 'U')

/** 首页（项目列表）无需返回按钮与面包屑。 */
const isHome = computed(() => route.name === 'projects' && !inProject.value)

/** 接口/文档工作台自带宽窄留白，取消页面内边距以获得全出血两栏。 */
const flushBody = computed(() => route.name === 'projectApis' || route.name === 'projectDocs')

// 进入项目页时补齐项目详情与分组（供分组树使用）；下拉数据首屏拉一次。
watch(
  projectId,
  (id) => {
    if (id) void project.load(id)
  },
  { immediate: true },
)

onMounted(() => {
  void project.loadProjects()
})

// 回到项目列表时刷新下拉，保证新建的项目可切换。
watch(
  () => route.name,
  (name) => {
    if (name === 'projects') void project.loadProjects(true)
  },
)

watch(
  () => [route.name, route.meta.title, project.title] as const,
  () => {
    document.title = [route.meta.title, project.title, 'API-DOC-GO'].filter(Boolean).join(' · ')
  },
  { immediate: true },
)

/** 有站内历史则真返回，否则回项目列表。 */
const hasHistory = computed(() => Boolean((window.history.state as { back?: string } | null)?.back))

function goBack(): void {
  if (hasHistory.value) {
    router.back()
    return
  }
  void router.push({ name: 'projects' })
}

const backLabel = computed(() => (hasHistory.value ? '返回上一页' : '返回项目列表'))

function goHome(): void {
  void router.push({ name: 'projects' })
}

function logout(): void {
  user.clear()
  project.clear()
  message.info('已退出登录')
  void router.push('/login')
}
</script>

<template>
  <div class="app">
    <header class="hdr">
      <div class="brand" title="API-DOC-GO" @click="goHome">
        <span class="brand-mark"><repo-mark /></span>
        <span class="brand-text">API-DOC-GO</span>
      </div>
      <div v-if="inProject" class="hdr-center">
        <n-select
          :value="projectId"
          :options="projectOptions"
          size="small"
          filterable
          class="switch"
          title="切换当前项目"
          @update:value="switchProject"
        />
      </div>
      <n-space align="center" :size="18" class="hdr-right">
        <notification-bell />
        <n-popover trigger="hover" placement="bottom-end" :width="380">
          <template #trigger>
            <span class="account">
              <n-avatar round :size="28" class="avatar">{{ initials }}</n-avatar>
              <span class="who">hello, {{ user.nickName }}</span>
            </span>
          </template>
          <user-account-panel @logout="logout" />
        </n-popover>
      </n-space>
    </header>

    <nav class="tabbar" aria-label="主导航">
      <div class="tabs">
        <button
          v-for="t in tabs"
          :key="t.key"
          class="tab"
          :class="{ 'tab--active': activeTab === t.key }"
          :aria-current="activeTab === t.key ? 'page' : undefined"
          @click="onTab(t.key)"
        >
          <n-icon :component="t.icon" :size="16" />
          <span>{{ t.label }}</span>
        </button>
      </div>
      <div v-if="inProject" class="tabbar-right">
        <button
          class="tab tab--trash"
          :class="{ 'tab--active': trashActive }"
          :aria-current="trashActive ? 'page' : undefined"
          @click="onTab('trash')"
        >
          <n-icon :component="TrashOutline" :size="16" />
          <span>回收站</span>
        </button>
      </div>
    </nav>

    <div class="content">
      <div class="content-inner">
        <div v-if="!isHome" class="crumb-bar">
          <n-button circle size="small" class="back-btn" :title="backLabel" :aria-label="backLabel" @click="goBack">
            <template #icon><n-icon :component="ArrowBackOutline" /></template>
          </n-button>
          <n-breadcrumb>
            <n-breadcrumb-item
              v-for="(c, i) in crumbs"
              :key="i"
              :clickable="Boolean(c.to)"
              @click="c.to && void router.push(c.to)"
            >
              {{ c.label }}
            </n-breadcrumb-item>
          </n-breadcrumb>
          <!-- 列表页经 Teleport 把导入/导出/搜索等操作注入到面包屑行右侧 -->
          <div id="crumb-actions" class="crumb-actions" />
        </div>
        <div class="page-body" :class="{ 'page-body--flush': flushBody }">
          <router-view />
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.app {
  display: flex;
  flex-direction: column;
  height: 100vh;
}

.hdr {
  position: relative;
  flex: 0 0 var(--app-header-h);
  height: var(--app-header-h);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 20px;
  background: #fff;
  border-bottom: 1px solid var(--app-border);
}

.hdr-right {
  flex: 0 0 auto;
}

.hdr-center {
  position: absolute;
  left: 50%;
  transform: translateX(-50%);
}

.brand {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  user-select: none;
}

.brand-mark {
  display: inline-flex;
  width: 19px;
  height: 19px;
  color: var(--app-accent);
}

.brand-text {
  font-size: 16px;
  font-weight: 600;
  letter-spacing: 0.3px;
}

.account {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
}

.avatar {
  background: var(--app-accent);
  color: #fff;
}

.who {
  color: var(--app-text);
  font-size: 13px;
}

/* ---- 横向 tab 栏 ---- */
.tabbar {
  position: relative;
  flex: 0 0 auto;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  padding: 0 16px 0 20px;
  background: #fff;
  border-bottom: 1px solid var(--app-border);
}

.tabs {
  display: flex;
  align-items: center;
  gap: 2px;
  overflow-x: auto;
  overflow-y: hidden;
}

.tabbar-right {
  position: absolute;
  right: 16px;
  display: flex;
  align-items: center;
  gap: 12px;
  flex: 0 0 auto;
}

.tab {
  position: relative;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 40px;
  padding: 0 12px;
  border: 0;
  background: none;
  color: var(--app-text);
  font-size: 14px;
  cursor: pointer;
  white-space: nowrap;
}

.tab:hover {
  color: var(--app-accent);
}

.tab--active {
  color: var(--app-accent);
  font-weight: 600;
}

.tab--active::after {
  content: '';
  position: absolute;
  left: 10px;
  right: 10px;
  bottom: -1px;
  height: 2px;
  background: var(--app-accent);
}

.tab--trash {
  color: var(--app-muted);
}

.switch {
  width: 180px;
}

/* ---- 内容区 ---- */
.content {
  flex: 1 1 auto;
  min-height: 0;
  overflow: auto;
  background: var(--app-bg);
}

.content-inner {
  min-height: 100%;
  display: flex;
  flex-direction: column;
}

.crumb-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 9px 20px;
  border-bottom: 1px solid var(--app-border);
  background: #fff;
}

.crumb-actions {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 12px;
}

.crumb-actions:empty {
  display: none;
}

.back-btn {
  flex: 0 0 auto;
}

.page-body {
  flex: 1 1 auto;
  display: flex;
  flex-direction: column;
  min-height: 0;
  padding: 20px;
}

.page-body--flush {
  padding: 0;
}
</style>
