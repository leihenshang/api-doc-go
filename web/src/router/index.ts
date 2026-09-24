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
                meta: { title: 'Bug 管理', requiresAuth: true },
            },
            {
                path: 'projects/:projectId/dict',
                name: 'projectDict',
                component: () => import('@/views/project/DataDictView.vue'),
                meta: { title: '数据字典', requiresAuth: true },
            },
            {
                path: 'projects/:projectId/logs',
                name: 'projectLogs',
                component: () => import('@/views/project/OpLogView.vue'),
                meta: { title: '操作日志', requiresAuth: true },
            },

            // 全局
            { path: 'notifications', name: 'notifications', component: () => import('@/views/NotificationView.vue'), meta: { title: '我的通知' } },
            { path: 'users', name: 'users', component: () => import('@/views/user/UserManagerView.vue'), meta: { title: '用户管理' } },
            { path: 'properties', name: 'properties', component: () => import('@/views/PropertyView.vue'), meta: { title: '公共属性' } },
            { path: 'me', name: 'myCenter', component: () => import('@/views/user/MyCenterView.vue'), meta: { title: '个人中心' } },
        ],
    },
    { path: '/:pathMatch(.*)*', redirect: { name: 'projects' } },
]

const router = createRouter({
    history: createWebHistory(),
    routes,
})

router.beforeEach((to) => {
    const user = useUserStore()
    const needsAuth = to.matched.some((r) => r.meta.requiresAuth)
    if (needsAuth && !user.isLogged) {
        return { name: 'login' }
    }
    return true
})

export default router
