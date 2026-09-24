// 领域实体类型，与后端 model 的 JSON 契约对齐（手工维护）。

export interface BaseEntity {
  id: number
  create_time: string
  update_time: string
}

export interface User extends BaseEntity {
  name: string
  email: string
  nick_name: string
  type: number
  state: number
  user_face: string
  mobile_number: string
}

export interface LoginResult {
  token: string
  user: User
}

export interface Project extends BaseEntity {
  title: string
  description: string
  version: string
  type: string
}

export interface MemberView {
  user_id: number
  name: string
  nick_name: string
  is_leader: number
  permission: number
}

export interface Group extends BaseEntity {
  p_id: number
  project_id: number
  title: string
  priority: number
  type: number
}

export interface ApiDoc extends BaseEntity {
  project_id: number
  group_id: number
  group_id_second: number
  api_name: string
  protocol_type: string
  http_method_type: string
  url: string
  description: string
  http_request_header: string
  http_request_params: string
  http_return_type: string
  http_return_params: string
  http_return_sample: string
  develop_language?: string
  object_name?: string
  function_name?: string
}

export interface DocItem extends BaseEntity {
  user_id: number
  project_id: number
  group_id: number
  title: string
  content: string
  view_count: number
  like_count: number
  is_top: number
  priority: number
}

export interface Bug extends BaseEntity {
  project_id: number
  to_user_id: number
  title: string
  content: string
  comment: string
  status: number
  level: number
}

export interface BugAssign extends BaseEntity {
  bug_id: number
  from_user_id: number
  to_user_id: number
  comment: string
  status: number
}

export interface FieldMapping extends BaseEntity {
  project_id: number
  field: string
  type: string
  description: string
}

export interface Conn extends BaseEntity {
  address: string
  port: number
  username: string
  database_name: string
  project_id: number
  type: number
  description: string
}

export interface Notification extends BaseEntity {
  title: string
  content: string
  send_type: number
  receiver: number
  read: number
}

export interface OpLog extends BaseEntity {
  user_id: number
  action: string
  content: string
  object_id: number
  type: number
  project_id: number
}

/** 接口请求/返回参数行（键名与后端字段映射 applyFieldMapping 约定一致）。 */
export interface ApiParam {
  fieldName: string
  type: string
  required?: boolean
  description: string
  example?: string
}

/** 接口请求头行。 */
export interface ApiHeader {
  name: string
  value: string
  description?: string
}

/** 列表分页结果（与后端 page.Result 对齐）。 */
export interface Page<T> {
  items: T[]
  total: number
  pageNum: number
  pageSize: number
  pageTotal: number
}
