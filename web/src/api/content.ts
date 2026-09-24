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
export function updateApi(payload: Record<string, unknown> & { id: number }): Promise<ApiResponse<ApiDoc>> {
  return put<ApiDoc>('/api', payload)
}
export function deleteApi(id: number): Promise<ApiResponse<null>> {
  return del<null>('/api', { id })
}
export function importOpenApi(projectId: number, doc: unknown, groupId = 0): Promise<ApiResponse<{ imported: number }>> {
  return post<{ imported: number }>(`/api/import?project_id=${projectId}&group_id=${groupId}`, doc)
}
export function restoreApi(id: number): Promise<ApiResponse<null>> {
  return post<null>('/api/restore', { id })
}
export function exportOpenApi(projectId: number, groupId = 0): Promise<ApiResponse<Record<string, unknown>>> {
  return get<Record<string, unknown>>('/api/export', { project_id: projectId, group_id: groupId })
}
export function getCurl(id: number): Promise<ApiResponse<{ curl: string }>> {
  return get<{ curl: string }>('/api/curl', { id })
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
export function updateDoc(payload: { id: number; title?: string; content?: string; group_id?: number; is_top?: number }): Promise<ApiResponse<DocItem>> {
  return put<DocItem>('/doc', payload)
}
export function deleteDoc(id: number): Promise<ApiResponse<null>> {
  return del<null>('/doc', { id })
}
export function likeDoc(id: number): Promise<ApiResponse<null>> {
  return post<null>(`/doc/like?id=${id}`)
}
export function restoreDoc(id: number): Promise<ApiResponse<null>> {
  return post<null>('/doc/restore', { id })
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
