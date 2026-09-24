// 当前项目状态：供侧边导航、项目切换下拉与面包屑复用，避免各页重复拉取。
import { listGroups } from '@/api/content'
import { getProject, listProjects, myPermission } from '@/api/project'
import { useUserStore } from '@/stores/user'
import { ErrCode } from '@/types/api'
import type { Group, Project } from '@/types/entities'
import { defineStore } from 'pinia'

export const useProjectStore = defineStore('project', {
    state: () => ({
        current: null as Project | null,
        /** 当前登录用户在本项目的权限掩码（1 读 / 2 建 / 4 改 / 8 删 / 15 全部）。 */
        permission: 0,
        /** 项目切换下拉的数据源。 */
        projects: [] as Project[],
        /** 当前项目的接口分组（type=1）与文档分组（type=3），供侧导航子菜单使用。 */
        apiGroups: [] as Group[],
        docGroups: [] as Group[],
    }),
    getters: {
        id(state): number {
            return state.current?.id ?? 0
        },
        title(state): string {
            return state.current?.title ?? ''
        },
    },
    actions: {
        /** 判断是否具备某位权限；管理员直接放行。 */
        can(perm: number): boolean {
            return useUserStore().isAdmin || (this.permission & perm) !== 0
        },
        /** 加载指定项目详情、权限与分组。 */
        async load(projectId: number): Promise<Project | null> {
            if (!projectId) return null
            const [detail, perm] = await Promise.all([getProject(projectId), myPermission(projectId)])
            if (detail.code === ErrCode.Success && detail.data) {
                this.current = detail.data
            }
            if (perm.code === ErrCode.Success && perm.data) {
                this.permission = perm.data.permission
            }
            await this.loadGroups(projectId)
            return this.current
        },
        /** 拉取当前项目的接口与文档分组（侧导航子菜单数据）。 */
        async loadGroups(projectId: number): Promise<void> {
            if (!projectId) {
                this.apiGroups = []
                this.docGroups = []
                return
            }
            const [apis, docs] = await Promise.all([listGroups(projectId, 1), listGroups(projectId, 3)])
            this.apiGroups = apis.code === ErrCode.Success && apis.data ? apis.data : []
            this.docGroups = docs.code === ErrCode.Success && docs.data ? docs.data : []
        },
        /** 项目切换下拉数据；force 为真时忽略已有缓存。 */
        async loadProjects(force = false): Promise<void> {
            if (this.projects.length && !force) return
            const res = await listProjects({ page_size: 200 })
            this.projects = res.code === ErrCode.Success && res.data ? res.data.items : []
        },
        /** 用最新数据覆盖缓存（编辑项目后调用）。 */
        set(project: Project) {
            this.current = project
        },
        clear() {
            this.current = null
            this.permission = 0
            this.apiGroups = []
            this.docGroups = []
        },
    },
})
