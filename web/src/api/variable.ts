// 变量与环境接口。
import type { ApiResponse } from '@/types/api'
import type { EnvRefs, ResolveItem, Variable, VariableEnv, VarSummary } from '@/types/entities'
import { del, get, post, put } from './http'

// ---- 项目级环境 ----
export function createVariableEnv(payload: {
  project_id: number
  name: string
  description?: string
  color?: string
  priority?: number
}): Promise<ApiResponse<VariableEnv>> {
  return post<VariableEnv>('/variable-env', payload)
}
export function listVariableEnvs(projectId: number): Promise<ApiResponse<VariableEnv[]>> {
  return get<VariableEnv[]>('/variable-env', { project_id: projectId })
}
export function updateVariableEnv(payload: {
  id: number
  project_id: number
  name: string
  description?: string
  color?: string
  priority?: number
}): Promise<ApiResponse<VariableEnv>> {
  return put<VariableEnv>('/variable-env', payload)
}
export function setDefaultVariableEnv(id: number): Promise<ApiResponse<null>> {
  return put<null>(`/variable-env/default?id=${id}`)
}
/** 删除环境：仍被变量引用或为默认环境时后端拒绝并返回明细文案。 */
export function deleteVariableEnv(id: number): Promise<ApiResponse<null>> {
  return del<null>('/variable-env', { id })
}
/** 删除前预检：返回引用该环境的变量清单与是否禁止删除。 */
export function variableEnvRefs(id: number): Promise<ApiResponse<EnvRefs>> {
  return get<EnvRefs>('/variable-env/references', { id })
}

// ---- 变量 ----
export interface VariablePayload {
  scope: number
  project_id?: number
  api_id?: number
  env_id?: number
  name: string
  /** 缺省表示不修改原值（仅更新场景有意义）。 */
  value?: string
  description?: string
  is_secret?: number
  enabled?: number
  priority?: number
}
export function createVariable(payload: VariablePayload): Promise<ApiResponse<Variable>> {
  return post<Variable>('/variable', payload)
}
export function updateVariable(
  payload: VariablePayload & { id: number; expected_update_time?: string },
): Promise<ApiResponse<Variable>> {
  return put<Variable>('/variable', payload)
}
export function deleteVariable(id: number): Promise<ApiResponse<null>> {
  return del<null>('/variable', { id })
}
/** 变量列表；with_value=1 且具备写权限时敏感值回明文（供编辑回显）。 */
export function listVariables(params: {
  project_id: number
  scope?: number
  api_id?: number
  /** 0 = 仅通用变量；不传 = 不按环境筛选。 */
  env_id?: number
  keyword?: string
  with_value?: number
}): Promise<ApiResponse<Variable[]>> {
  return get<Variable[]>('/variable', params)
}
/** 解析表：接口可见变量的生效值（敏感值掩码），同名只保留最高优先级。 */
export function resolveVariables(projectId: number, apiId: number, envId = 0): Promise<ApiResponse<ResolveItem[]>> {
  return get<ResolveItem[]>('/variable/resolve', { project_id: projectId, api_id: apiId, env_id: envId })
}
/** 批量统计接口的变量使用情况（列表徽标），单次上限 200 个接口。 */
export function summarizeVariables(projectId: number, apiIds: number[], envId = 0): Promise<ApiResponse<Record<string, VarSummary>>> {
  return get<Record<string, VarSummary>>('/variable/summary', {
    project_id: projectId,
    api_ids: apiIds.join(','),
    env_id: envId,
  })
}
