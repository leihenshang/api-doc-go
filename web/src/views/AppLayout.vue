<script setup lang="ts">
// 应用外壳：顶栏（品牌 + 项目切换 + 全局搜索 + 通知铃铛 + 用户菜单）+ 横向 tab 导航
//（最左「主页」入口 / 中间页签 / 右侧页面操作与回收站）。tab/回收站逻辑见 composables/projectNav.ts。
// 文案走 i18n（语言切换入口在 UserMenu）；挂载时按用户表 lang 对齐界面语言。
import GlobalSearch from '@/components/GlobalSearch.vue'
import NotificationBell from '@/components/NotificationBell.vue'
import RepoMark from '@/components/RepoMark.vue'
import UserMenu from '@/components/UserMenu.vue'
import { useLogout } from '@/composables/logout'
import { syncLocaleFromUser } from '@/composables/locale'
import { useProjectNav } from '@/composables/projectNav'
import { me } from '@/api/auth'
import { useProjectStore } from '@/stores/project'
import { useUserStore } from '@/stores/user'
import { ErrCode } from '@/types/api'
import { HomeOutline, TrashOutline } from '@vicons/ionicons5'
import {
    NButton,
    NIcon,
    NSelect,
    NSpace,
} from 'naive-ui'
import { computed, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'

const route = useRoute()
const router = useRouter()
const project = useProjectStore()
const user = useUserStore()
const { t } = useI18n()

const { projectId, inProject, tabs, activeTab, trashActive, onTab, switchProject } = useProjectNav()

const projectOptions = computed(() => project.projects.map((p) => ({ label: p.title, value: p.id })))

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
  void syncUserLocale()
})

/** 刷新/登录后按用户表偏好对齐界面语言（跨设备一致；未登录或未设置则保持本地选择）。 */
async function syncUserLocale(): Promise<void> {
  if (!user.isLogged) return
  try {
    const res = await me()
    if (res.code === ErrCode.Success && res.data) syncLocaleFromUser(res.data.lang)
  } catch {
    // 语言同步失败不阻塞界面（保持本地选择）。
  }
}

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

/** 「主页」= 项目列表；无论历史栈如何都直接回首页（不做浏览器级返回）。 */
function goHome(): void {
  void router.push({ name: 'projects' })
}

const logout = useLogout()
</script>

<template>
  <div class="app">
    <header class="hdr">
      <div class="hdr-left">
        <div class="brand" title="API-DOC-GO" @click="goHome">
          <span class="brand-mark"><repo-mark /></span>
          <span class="brand-text">API-DOC-GO</span>
        </div>
        <n-select
          v-if="inProject"
          :value="projectId"
          :options="projectOptions"
          size="medium"
          filterable
          class="switch"
          :title="t('nav.switchProject')"
          @update:value="switchProject"
        />
      </div>
      <global-search />
      <n-space align="center" :size="18" class="hdr-right">
        <notification-bell />
        <user-menu @logout="logout" />
      </n-space>
    </header>

    <nav class="tabbar" :aria-label="t('nav.mainNav')">
      <div class="tabbar-left">
        <n-button
          v-if="!isHome"
          circle
          size="small"
          class="back-btn"
          :title="t('nav.home')"
          :aria-label="t('nav.home')"
          @click="goHome"
        >
          <template #icon><n-icon :component="HomeOutline" /></template>
        </n-button>
      </div>
      <div class="tabs">
        <button
          v-for="tab in tabs"
          :key="tab.key"
          class="tab"
          :class="{ 'tab--active': activeTab === tab.key }"
          :aria-current="activeTab === tab.key ? 'page' : undefined"
          @click="onTab(tab.key)"
        >
          <n-icon :component="tab.icon" :size="16" />
          <span>{{ tab.label }}</span>
        </button>
      </div>
      <div class="tabbar-right">
        <!-- 列表页经 Teleport 把导入/导出/搜索等操作注入到横向菜单右侧 -->
        <div id="crumb-actions" class="page-actions" />
        <button
          v-if="inProject"
          class="tab tab--trash"
          :class="{ 'tab--active': trashActive }"
          :aria-current="trashActive ? 'page' : undefined"
          @click="onTab('trash')"
        >
          <n-icon :component="TrashOutline" :size="16" />
          <span>{{ t('nav.trash') }}</span>
        </button>
      </div>
    </nav>

    <div class="content">
      <div class="content-inner" :class="{ 'content-inner--flush': flushBody }">
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

.hdr-left {
  display: flex;
  align-items: center;
  gap: 4px;
  min-width: 0;
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

/* ---- 横向 tab 栏 ---- */
.tabbar {
  flex: 0 0 auto;
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 0 16px;
  background: #fff;
  border-bottom: 1px solid var(--app-border);
}

.tabbar-left,
.tabbar-right {
  display: flex;
  align-items: center;
  gap: 10px;
  flex: 0 0 auto;
}

.tabs {
  flex: 1 1 auto;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 2px;
  overflow-x: auto;
  overflow-y: hidden;
}

/* 列表页 Teleport 注入的页面操作（回收站搜索等） */
.page-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.page-actions:empty {
  display: none;
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

/* 项目切换：紧跟品牌标题，加粗 + 外框（更显眼），下拉箭头贴近文字 */
.switch {
  width: 142px;
  margin-left: 4px;
}

.switch :deep(.n-base-selection) {
  /* 收窄右侧为箭头预留的宽度，让箭头贴近项目名 */
  --n-padding-single-right: 18px;
  --n-padding-single-left: 10px;
  border-radius: 5px;
  background: #fff;
}

.switch :deep(.n-base-selection-label),
.switch :deep(.n-base-selection-input) {
  font-weight: 700;
  font-size: 14px;
}

.switch :deep(.n-base-selection-overlay__wrapper) {
  white-space: nowrap;
  text-overflow: ellipsis;
  overflow: hidden;
}

/* naive-ui 的箭头绝对定位在右侧 10px，收紧到 4px */
.switch :deep(.n-base-suffix) {
  right: 4px;
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

/* 工作台（接口/文档页）需要「外壳不滚、内部各栏自己滚」：显式定高，否则内容会把外层撑高 */
.content-inner--flush {
  height: 100%;
  min-height: 0;
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

/* ---- 移动端：顶栏换行、页签横向滑动 ---- */
@media (max-width: 768px) {
  .hdr {
    flex: 0 0 auto;
    height: auto;
    min-height: var(--app-header-h);
    flex-wrap: wrap;
    gap: 8px;
    padding: 8px 12px;
  }

  .hdr-left {
    flex: 1 1 auto;
  }

  .switch {
    width: 118px;
  }

  .tabbar {
    gap: 8px;
    padding: 0 8px 0 12px;
  }

  .tabbar-left,
  .tabbar-right {
    gap: 6px;
  }

  .tabs {
    justify-content: flex-start;
    -webkit-overflow-scrolling: touch;
  }

  .tab {
    height: 38px;
    padding: 0 8px;
    font-size: 13px;
  }
}
</style>
