// 项目与成员接口。
import type { ApiResponse } from '@/types/api';
import type { MemberView, Page, Project } from '@/types/entities';
import { del, get, post, put } from './http';

export function listProjects(params: { page_num?: number; page_size?: number }): Promise<ApiResponse<Page<Project>>> {
  return get<Page<Project>>('/project', params)
}

export function createProject(payload: { title: string; description?: string; version?: string; type?: string }): Promise<ApiResponse<Project>> {
  return post<Project>('/project', payload)
}

export function getProject(id: number): Promise<ApiResponse<Project>> {
  return get<Project>('/project/detail', { id })
}

export function updateProject(payload: { id: number; title?: string; description?: string; version?: string; type?: string }): Promise<ApiResponse<Project>> {
  return put<Project>('/project', payload)
}

export function deleteProject(id: number): Promise<ApiResponse<null>> {
  return del<null>('/project', { id })
}

export function listMembers(projectId: number): Promise<ApiResponse<MemberView[]>> {
  return get<MemberView[]>('/project/members', { project_id: projectId })
}

export function addMember(payload: { project_id: number; user_id: number; permission?: number }): Promise<ApiResponse<null>> {
  return post<null>('/project/member', payload)
}

export function setPermission(payload: { project_id: number; user_id: number; permission: number }): Promise<ApiResponse<null>> {
  return put<null>('/project/permission', payload)
}

export function setLeader(payload: { project_id: number; user_id: number }): Promise<ApiResponse<null>> {
  return put<null>('/project/leader', payload)
}

export function quitProject(projectId: number): Promise<ApiResponse<null>> {
  return post<null>('/project/quit', { project_id: projectId })
}

export function myPermission(projectId: number): Promise<ApiResponse<{ permission: number }>> {
  return get<{ permission: number }>('/project/permission', { project_id: projectId })
}
