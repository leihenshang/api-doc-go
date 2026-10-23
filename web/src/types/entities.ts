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
  /** 界面语言偏好：''=未选择（前端按浏览器语言兜底）/ 'zh-CN' / 'en-US'。 */
  lang: string
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
  /** 是否允许桌面客户端写入（S14）；false = 客户端只能拉取、推送被拒。 */
  client_write_enabled?: boolean | null
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
  priority: number
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
  // 客户端富字段（同步过来，Web 只读展示；均为原始 JSON/文本串）
  http_request_body_type?: string
  http_request_body?: string
  http_request_body_form?: string
  http_request_body_descs?: string
  http_auth?: string
  request_settings?: string
  http_script?: string
  script_vars?: string
  script_asserts?: string
  http_return_content_type?: string
}

export interface DocItem extends BaseEntity {
  user_id: number
  project_id: number
  group_id: number
  title: string
  content: string
  /** 文档图标：上传图片 URL 或 emoji（空则用默认图标）。 */
  icon: string
  view_count: number
  like_count: number
  is_top: number
  priority: number
}

/** 分享链接：免登录只读入口（token 可随时失效、可设有效期）。 */
export interface ShareLink extends BaseEntity {
  project_id: number
  user_id: number
  token: string
  remark: string
  expire_time: string | null
}

/** 分享页数据（无权限/日志等内部字段）。 */
export interface ShareView {
  project: { id: number; title: string; description: string; version: string }
  groups: Group[]
  apis: ApiDoc[]
  docs: DocItem[]
  remark: string
  expire_time: string | null
}

export interface Bug extends BaseEntity {
  project_id: number
  to_user_id: number
  title: string
  content: string
  comment: string
  status: number
  level: number
  /** 置顶（1 置顶 / 0 普通），列表按置顶优先排序。 */
  is_top: number
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

/** 项目级环境：项目变量的取值维度（dev/test/prod）。 */
export interface VariableEnv extends BaseEntity {
  project_id: number
  name: string
  description: string
  color: string
  /** 是否项目默认环境（1 默认 / 0 普通）。 */
  is_default: number
  priority: number
}

/** 变量（三级共用）：scope 1 请求级 / 2 项目级 / 3 全局（应用级）。 */
export interface Variable extends BaseEntity {
  scope: number
  project_id: number
  api_id: number
  env_id: number
  name: string
  /** 敏感值（is_secret=1）在列表与解析结果中为掩码。 */
  value: string
  description: string
  is_secret: number
  has_value: boolean
  enabled: number
  priority: number
}

/** 解析结果项：变量 + 生效来源。 */
export interface ResolveItem extends Variable {
  source: 'request' | 'project' | 'global'
}

/** 接口列表的变量徽标统计。 */
export interface VarSummary {
  defined: string[]
  undefined: string[]
  request_count: number
  undefined_count: number
}

/** 环境删除前预检结果（variables 为引用该环境的变量，blocked 表示禁止删除）。 */
export interface EnvRefs {
  env: { id: number; name: string; is_default: number }
  variables: Variable[]
  blocked: boolean
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

/** 在线调试的请求头 / 查询参数 / 表单行。 */
export interface ProxyKV {
  name: string
  value: string
  enabled?: boolean
}

/** 在线调试认证配置（与客户端集合 auth 的 JSON 形态对齐）。 */
export interface ProxyAuth {
  type: string
  username?: string
  password?: string
  token?: string
  key?: string
  value?: string
  in?: string
}

/** 在线调试请求体表单行（客户端 bodyForm 的超集）。 */
export interface ProxyFormRow {
  name?: string
  value?: string
  enabled?: boolean
  description?: string
  type?: string
}

/** 在线调试请求：服务端负责变量渲染、SSRF 校验与审计。 */
export interface ProxySendReq {
  project_id: number
  api_id?: number
  env_id?: number
  method: string
  url: string
  headers?: ProxyKV[]
  query?: ProxyKV[]
  body_type?: string
  body?: string
  form?: ProxyKV[]
  auth?: ProxyAuth | null
}

/** 在线调试响应头（保留重复项）。 */
export interface ProxyHeader {
  name: string
  value: string
}

/** 在线调试响应。 */
export interface ProxySendResp {
  status: number
  status_text: string
  headers: ProxyHeader[]
  body: string
  content_type: string
  duration_ms: number
  size: number
  truncated: boolean
  final_url: string
}

/** 个人访问令牌（PAT，S7）：服务端只存 hash，明文仅创建时返回一次。 */
export interface UserToken {
  id: number
  name: string
  scopes: string
  expire_at: string
  last_used: string
  revoked: number
  create_time: string
}

/** 创建 PAT 的返回：token 为明文（仅此一次可见）。 */
export interface CreateTokenResp {
  token: string
  info: UserToken
}

/** 列表分页结果（与后端 page.Result 对齐）。 */
export interface Page<T> {
  items: T[]
  total: number
  pageNum: number
  pageSize: number
  pageTotal: number
}
