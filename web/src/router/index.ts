// 路由与全局守卫。受保护页用 meta.requiresAuth（见开发规范 §4.3）。
// 项目内页面按「一屏一事」拆为独立路由，由 AppLayout 依据 meta 渲染侧导航与面包屑。
import { useUserStore } from '@/stores/user'
import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'

declare module 'vue-router' {
    interface RouteMeta {
        /** 页面标题，用于面包屑与浏览器标题。 */
        title?: string
        /** 需要登录。 */
        requiresAuth?: boolean
    }
}

const routes: RouteRecordRaw[] = [
    { path: '/login', name: 'login', component: () => import('@/views/LoginView.vue') },
    // 免登录只读分享页（令牌在 URL 中，不需要登录态）
    { path: '/share/:token', name: 'share', component: () => import('@/views/ShareView.vue') },
    { path: '/register', name: 'register', component: () => import('@/views/RegisterView.vue') },
    {
        path: '/',
        component: () => import('@/views/AppLayout.vue'),
        meta: { requiresAuth: true },
        children: [
            { path: '', redirect: { name: 'projects' } },
            { path: 'projects', name: 'projects', component: () => import('@/views/ProjectListView.vue'), meta: { title: '项目列表' } },

            // 项目内
            {
                path: 'projects/:projectId',
                name: 'projectOverview',
                component: () => import('@/views/project/ProjectOverviewView.vue'),
                meta: { title: '项目概况', requiresAuth: true },
            },
            {
                path: 'projects/:projectId/apis',
                name: 'projectApis',
                component: () => import('@/views/project/ApiListView.vue'),
                meta: { title: 'API接口', requiresAuth: true },
            },
            {
                path: 'projects/:projectId/docs',
                name: 'projectDocs',
                component: () => import('@/views/project/DocListView.vue'),
                meta: { title: '项目文档', requiresAuth: true },
            },
            {
                path: 'projects/:projectId/fields',
                name: 'projectFields',
                component: () => import('@/views/project/FieldMappingView.vue'),
                meta: { title: '字段映射', requiresAuth: true },
            },
            {
                path: 'projects/:projectId/bugs',
                name: 'projectBugs',
                component: () => import('@/views/project/BugView.vue'),
                meta: { title: '备忘清单', requiresAuth: true },
            },
            {
                path: 'projects/:projectId/variables',
                name: 'projectVariables',
                component: () => import('@/views/project/VariableView.vue'),
                meta: { title: '变量', requiresAuth: true },
            },

            // 全局
            { path: 'notifications', name: 'notifications', component: () => import('@/views/NotificationView.vue'), meta: { title: '我的通知' } },
            { path: 'users', name: 'users', component: () => import('@/views/user/UserManagerView.vue'), meta: { title: '用户管理', requiresAdmin: true } },
            { path: 'properties', name: 'properties', component: () => import('@/views/PropertyView.vue'), meta: { title: '公共属性', requiresAdmin: true } },
            { path: 'me', name: 'myCenter', component: () => import('@/views/user/MyCenterView.vue'), meta: { title: '个人中心' } },
        ],
    },
    { path: '/:pathMatch(.*)*', redirect: { name: 'projects' } },
]

const router = createRouter({
    history: createWebHistory(),
    routes,
})

// 导航失败（守卫抛错、组件加载失败）默认是静默的，会表现为「URL 变了但页面空白」。
router.onError((err, to) => {
  console.error('[router]', to?.fullPath ?? '', err)
})

router.beforeEach((to) => {
    const user = useUserStore()
    const needsAuth = to.matched.some((r) => r.meta.requiresAuth)
    if (needsAuth && !user.isLogged) {
        return { name: 'login' }
    }
    // 管理员页面（用户管理 / 公共属性）必须挡在路由层：只隐藏页签挡不住手输 URL
    const needsAdmin = to.matched.some((r) => r.meta.requiresAdmin)
    if (needsAdmin && !user.isAdmin) {
        return { name: 'projects' }
    }
    return true
})

export default router
