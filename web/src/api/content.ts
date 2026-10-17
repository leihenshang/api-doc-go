// 分组 / 接口 / 文档 / 字段映射接口。
import type { ApiResponse } from '@/types/api';
import type { ApiDoc, DocItem, FieldMapping, Group, Page } from '@/types/entities';
import { del, get, post, put } from './http';

// ---- 分组 ----
export function createGroup(payload: { project_id: number; title: string; p_id?: number; type?: number; priority?: number }): Promise<ApiResponse<Group>> {
  return post<Group>('/group', payload)
}
export function listGroups(projectId: number, type?: number): Promise<ApiResponse<Group[]>> {
  return get<Group[]>('/group', { project_id: projectId, type })
}

/** B19：文档分组树 —— 含文档的分组（不分 type）+ 未分组文档数。 */
export interface DocGroupItem {
  id: number
  p_id: number
  title: string
  type: number
  priority: number
  doc_count: number
}
export function listDocGroups(projectId: number): Promise<ApiResponse<{ groups: DocGroupItem[]; ungrouped: number }>> {
  return get<{ groups: DocGroupItem[]; ungrouped: number }>('/doc/groups', { project_id: projectId })
}
export function updateGroup(payload: { id: number; title?: string; p_id?: number; priority?: number }): Promise<ApiResponse<Group>> {
  return put<Group>('/group', payload)
}
export function deleteGroup(id: number): Promise<ApiResponse<null>> {
  return del<null>('/group', { id })
}

// ---- 接口 ----
export function createApi(payload: Record<string, unknown>): Promise<ApiResponse<ApiDoc>> {
  return post<ApiDoc>('/api', payload)
}
export function listApis(params: { project_id: number; group_id?: number; deleted?: number; keyword?: string; page_num?: number; page_size?: number }): Promise<ApiResponse<Page<ApiDoc>>> {
  return get<Page<ApiDoc>>('/api', params)
}
export function getApi(id: number): Promise<ApiResponse<ApiDoc>> {
  return get<ApiDoc>('/api/detail', { id })
}
export function updateApi(
  payload: Record<string, unknown> & { id: number; expected_update_time?: string },
): Promise<ApiResponse<ApiDoc>> {
  return put<ApiDoc>('/api', payload)
}
export function deleteApi(id: number): Promise<ApiResponse<null>> {
  return del<null>('/api', { id })
}

// ---- 接口响应示例（多示例） ----
export interface ApiReturnSample {
  id: number
  api_id: number
  name: string
  sample_text: string
  status: number
  content_type: string
  seq: number
}
export function saveApiSample(payload: {
  id?: number; api_id: number; name: string; sample_text?: string; status?: number; content_type?: string; seq?: number,
}): Promise<ApiResponse<ApiReturnSample>> {
  return post<ApiReturnSample>('/api/sample', payload)
}
export function listApiSamples(apiId: number): Promise<ApiResponse<ApiReturnSample[]>> {
  return get<ApiReturnSample[]>('/api/samples', { api_id: apiId })
}
export function deleteApiSample(id: number): Promise<ApiResponse<null>> {
  return del<null>('/api/sample', { id })
}
/** 导入结果：imported 成功条数、skipped 已存在跳过条数、failed 解析失败条数与明细。 */
export interface ImportResult {
  imported: number
  skipped: number
  failed: number
  failures?: string[]
}

export function importOpenApi(projectId: number, doc: unknown, groupId = 0, strategy = 'append'): Promise<ApiResponse<ImportResult>> {
  return post<ImportResult>(`/api/import?project_id=${projectId}&group_id=${groupId}&strategy=${strategy}`, doc)
}

/** 导入 Bruno 集合目录：files 为集合内 yml 文本文件（相对集合根目录路径 + 内容）。 */
export function importBruno(
  projectId: number,
  files: { path: string; content: string }[],
  strategy = 'append',
): Promise<ApiResponse<ImportResult>> {
  return post<ImportResult>(`/api/import-bruno?project_id=${projectId}&strategy=${strategy}`, { files })
}
/** 恢复结果：restored 为恢复条目数，groups 为随之恢复的所属分组数。 */
export interface RestoreResult {
  restored: number
  groups: number
}

export function restoreApi(id: number): Promise<ApiResponse<RestoreResult>> {
  return post<RestoreResult>('/api/restore', { id })
}

/** 回收站批量还原：服务端会把已删除的所属分组（含祖先链）一并恢复。 */
export function restoreApisBatch(ids: number[]): Promise<ApiResponse<RestoreResult>> {
  return post<RestoreResult>('/api/restore-batch', { ids })
}

/** 回收站批量彻底删除（不可撤销）。 */
export function purgeApisBatch(ids: number[]): Promise<ApiResponse<{ purged: number }>> {
  return post<{ purged: number }>('/api/purge-batch', { ids })
}
export function exportOpenApi(projectId: number, groupId = 0): Promise<ApiResponse<Record<string, unknown>>> {
  return get<Record<string, unknown>>('/api/export', { project_id: projectId, group_id: groupId })
}
/** 导出 Postman Collection v2.1（分组 → 文件夹）。 */
export function exportPostman(projectId: number, groupId = 0): Promise<ApiResponse<Record<string, unknown>>> {
  return get<Record<string, unknown>>('/api/export-postman', { project_id: projectId, group_id: groupId })
}
/** 导入 Postman 集合（JSON 文本）。 */
export function importPostman(projectId: number, doc: unknown, strategy = 'append'): Promise<ApiResponse<ImportResult>> {
  return post<ImportResult>(`/api/import-postman?project_id=${projectId}&strategy=${strategy}`, doc)
}
export function getCurl(id: number, envId = 0, resolve = false): Promise<ApiResponse<{ curl: string }>> {
  return get<{ curl: string }>('/api/curl', { id, env_id: envId, resolve: resolve ? 1 : 0 })
}
/** 请求代码片段：lang = curl | fetch | axios | go | python；resolve=1 时由服务端用当前环境变量值替换 {{占位符}}。 */
export function getCode(
  apiId: number,
  lang: string,
  envId = 0,
  resolve = false,
): Promise<ApiResponse<{ lang: string; code: string; resolved: boolean }>> {
  return get<{ lang: string; code: string; resolved: boolean }>('/api/code', {
    api_id: apiId,
    lang,
    env_id: envId,
    resolve: resolve ? 1 : 0,
  })
  // 说明：与 getCurl 一致走接口模块前缀，实际路径为 /api/api/code。
}
/** 全局搜索：跨项目搜接口 / 文档 / 项目（普通用户仅限自己参与的项目）。 */
export interface SearchItem {
  kind: string
  id: number
  project_id: number
  project_name: string
  title: string
  subtitle: string
  method: string
  update_time: string
}
export interface SearchResult {
  keyword: string
  projects: SearchItem[]
  apis: SearchItem[]
  docs: SearchItem[]
}
export function searchAll(keyword: string, limit = 8): Promise<ApiResponse<SearchResult>> {
  return get<SearchResult>('/search', { keyword, limit })
}

// ---- 文档 ----
export function createDoc(payload: { project_id: number; group_id?: number; title: string; content?: string }): Promise<ApiResponse<DocItem>> {
  return post<DocItem>('/doc', payload)
}
export function listDocs(params: { project_id: number; group_id?: number; keyword?: string; deleted?: number; page_num?: number; page_size?: number }): Promise<ApiResponse<Page<DocItem>>> {
  return get<Page<DocItem>>('/doc', params)
}
export function getDoc(id: number): Promise<ApiResponse<DocItem>> {
  return get<DocItem>('/doc/detail', { id })
}
export function updateDoc(payload: {
  id: number
  title?: string
  content?: string
  /** 文档图标：emoji 或上传后的图片 URL；空串表示清除。 */
  icon?: string
  group_id?: number
  is_top?: number
  /** 载入时的 update_time：传入则服务端做乐观锁校验（冲突返回 14）。 */
  expected_update_time?: string
}): Promise<ApiResponse<DocItem>> {
  return put<DocItem>('/doc', payload)
}
export function deleteDoc(id: number): Promise<ApiResponse<null>> {
  return del<null>('/doc', { id })
}
export function likeDoc(id: number): Promise<ApiResponse<null>> {
  return post<null>(`/doc/like?id=${id}`)
}
export function restoreDoc(id: number): Promise<ApiResponse<RestoreResult>> {
  return post<RestoreResult>('/doc/restore', { id })
}

/** 回收站批量还原文档（顺带恢复已删除的所属分组）。 */
export function restoreDocsBatch(ids: number[]): Promise<ApiResponse<RestoreResult>> {
  return post<RestoreResult>('/doc/restore-batch', { ids })
}

/** 回收站批量彻底删除文档（不可撤销）。 */
export function purgeDocsBatch(ids: number[]): Promise<ApiResponse<{ purged: number }>> {
  return post<{ purged: number }>('/doc/purge-batch', { ids })
}

// ---- 拖拽排序 ----
/** 排序项：parent_id 为分组树父级（分组=p_id，接口/文档=group_id），priority 越小越靠前。 */
export interface SortItem {
  id: number
  parent_id: number
  priority: number
}
export function sortNodes(payload: { project_id: number; kind: 'group' | 'api' | 'doc'; items: SortItem[] }): Promise<ApiResponse<null>> {
  return post<null>('/sort', payload)
}

// ---- 字段映射 ----
export function createFieldMapping(payload: { project_id: number; field: string; type: string; description?: string }): Promise<ApiResponse<FieldMapping>> {
  return post<FieldMapping>('/field-mapping', payload)
}
export function listFieldMappings(projectId: number): Promise<ApiResponse<FieldMapping[]>> {
  return get<FieldMapping[]>('/field-mapping', { project_id: projectId })
}
export function updateFieldMapping(payload: { id: number; project_id: number; field: string; type: string; description?: string }): Promise<ApiResponse<FieldMapping>> {
  return put<FieldMapping>('/field-mapping', payload)
}
export function deleteFieldMapping(id: number): Promise<ApiResponse<null>> {
  return del<null>('/field-mapping', { id })
}
