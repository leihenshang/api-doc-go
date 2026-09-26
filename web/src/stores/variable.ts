// 变量展示状态：当前环境（按项目记忆在 localStorage）+ 解析表与徽标缓存，供接口详情页与左树复用。
// 缓存键都带 projectId:envId，切换环境自然换键，无需手动失效；变量/环境变更后调 invalidate()。
import { listVariableEnvs, resolveVariables, summarizeVariables } from '@/api/variable'
import { ErrCode } from '@/types/api'
import type { ResolveItem, VariableEnv, VarSummary } from '@/types/entities'
import { defineStore } from 'pinia'

/** 单次徽标批量查询上限（与后端 `maxSummaryApiIDs` 一致），超出自动分批。 */
const SUMMARY_BATCH = 200

const envStorageKey = (projectId: number): string => `apidoc.env.${projectId}`
const scopeKey = (projectId: number, envId: number): string => `${projectId}:${envId}`

export const useVariableStore = defineStore('variable', {
    state: () => ({
        /** projectId → 环境列表。 */
        envs: {} as Record<number, VariableEnv[]>,
        /** projectId → 当前环境 id（0 = 未选择，仅通用与全局变量参与解析）。 */
        currentEnv: {} as Record<number, number>,
        /** `projectId:envId:apiId` → 解析表。 */
        resolved: {} as Record<string, ResolveItem[]>,
        /** `projectId:envId:apiId` → 徽标统计。 */
        summary: {} as Record<string, VarSummary>,
    }),
    getters: {
        envList: (state) => (projectId: number): VariableEnv[] => state.envs[projectId] ?? [],
        /** 当前环境对象（用于展示颜色与默认标记）。 */
        envObj: (state) => (projectId: number): VariableEnv | undefined => {
            const id = state.currentEnv[projectId] ?? 0
            return (state.envs[projectId] ?? []).find((e) => e.id === id)
        },
        /** 某接口的解析表（变量名 → 生效项），供 VarText 就地渲染。 */
        mapFor: (state) => (projectId: number, apiId: number): Record<string, ResolveItem> => {
            const key = `${scopeKey(projectId, state.currentEnv[projectId] ?? 0)}:${apiId}`
            const map: Record<string, ResolveItem> = {}
            for (const it of state.resolved[key] ?? []) map[it.name] = it
            return map
        },
        /** 某接口的变量徽标统计。 */
        summaryFor: (state) => (projectId: number, apiId: number): VarSummary | undefined =>
            state.summary[`${scopeKey(projectId, state.currentEnv[projectId] ?? 0)}:${apiId}`],
    },
    actions: {
        /** 加载项目环境：当前环境取「本地记忆 → 默认环境 → 第一个」；被删除的环境自动回落默认。 */
        async loadEnvs(projectId: number, force = false): Promise<VariableEnv[]> {
            if (!projectId) return []
            if (!force && this.envs[projectId]?.length) return this.envs[projectId]
            const res = await listVariableEnvs(projectId)
            const list = res.code === ErrCode.Success && res.data ? res.data : []
            this.envs[projectId] = list
            const saved = Number(localStorage.getItem(envStorageKey(projectId)) || 0)
            const fallback = list.find((e) => e.is_default === 1)?.id ?? list[0]?.id ?? 0
            this.currentEnv[projectId] = saved > 0 && list.some((e) => e.id === saved) ? saved : fallback
            return list
        },
        /** 切换当前环境：写内存 + localStorage，解析表与徽标按新环境键重新拉取。 */
        setCurrentEnv(projectId: number, envId: number): void {
            this.currentEnv[projectId] = envId
            try {
                localStorage.setItem(envStorageKey(projectId), String(envId))
            } catch {
                // 隐私模式等写入失败时忽略：仅本次会话生效。
            }
        },
        /** 拉取某接口的解析表（带缓存）。 */
        async resolve(projectId: number, apiId: number, force = false): Promise<ResolveItem[]> {
            if (!projectId || !apiId) return []
            await this.loadEnvs(projectId)
            const envId = this.currentEnv[projectId] ?? 0
            const key = `${scopeKey(projectId, envId)}:${apiId}`
            if (!force && this.resolved[key]) return this.resolved[key]
            const res = await resolveVariables(projectId, apiId, envId)
            const items = res.code === ErrCode.Success && res.data ? res.data : []
            this.resolved[key] = items
            return items
        },
        /** 批量拉取接口的变量徽标（自动分批）。 */
        async summaryOf(projectId: number, apiIds: number[], force = false): Promise<void> {
            if (!projectId || !apiIds.length) return
            await this.loadEnvs(projectId)
            const envId = this.currentEnv[projectId] ?? 0
            const missing = apiIds.filter((id) => force || !this.summary[`${scopeKey(projectId, envId)}:${id}`])
            for (let i = 0; i < missing.length; i += SUMMARY_BATCH) {
                const batch = missing.slice(i, i + SUMMARY_BATCH)
                const res = await summarizeVariables(projectId, batch, envId)
                const data = res.code === ErrCode.Success && res.data ? res.data : {}
                for (const [apiId, item] of Object.entries(data)) {
                    this.summary[`${scopeKey(projectId, envId)}:${Number(apiId)}`] = item
                }
            }
        },
        /** 变量/环境变更后清掉该项目的解析表、徽标与环境缓存。 */
        invalidate(projectId: number): void {
            const prefix = `${projectId}:`
            for (const key of Object.keys(this.resolved)) {
                if (key.startsWith(prefix)) delete this.resolved[key]
            }
            for (const key of Object.keys(this.summary)) {
                if (key.startsWith(prefix)) delete this.summary[key]
            }
            this.envs[projectId] = []
        },
    },
})
