// 文档导出：参数解析容错 + Markdown 结构。导出结果是交付物，格式回归最容易被忽视。
import { describe, expect, it } from 'vitest'

import type { Group } from '@/types/entities'
import { apiHeaders, apiParams, apiToMarkdown, groupPath, safeName } from '@/utils/export-doc'

const api = {
  id: 1,
  api_name: '用户-列表',
  group_id: 3,
  http_method_type: 'POST',
  url: 'https://api.example.com/v1/user/list',
  http_request_header: '[{"name":"X-Token","value":"abc"}]',
  http_request_params: '[{"fieldName":"page","type":"integer","required":true,"example":"1"}]',
  http_return_type: 'application/json',
  http_return_params: '[{"fieldName":"code","type":"integer","required":false,"example":"200"}]',
  http_return_sample: '{"code":200}',
}

describe('参数解析容错', () => {
  it('请求头 JSON 损坏时返回空数组而不是抛错', () => {
    expect(apiHeaders({ ...api, http_request_header: '{oops' })).toEqual([])
    expect(apiHeaders({ ...api, http_request_header: '' })).toEqual([])
  })

  it('参数 JSON 损坏时返回空数组', () => {
    expect(apiParams('{oops')).toEqual([])
    expect(apiParams('')).toEqual([])
  })

  it('合法 JSON 正常解析（请求头转成参数行，值落到 example）', () => {
    expect(apiHeaders(api)).toEqual([
      { fieldName: 'X-Token', type: '', required: false, description: '', example: 'abc' },
    ])
    expect(apiParams(api.http_request_params)).toHaveLength(1)
  })
})

describe('groupPath', () => {
  const groups: Group[] = [
    { id: 3, title: '用户', p_id: 2 } as Group,
    { id: 2, title: '平台', p_id: 0 } as Group,
  ]

  it('从顶层到当前分组', () => {
    expect(groupPath(groups, 3)).toEqual(['平台', '用户'])
  })

  it('找不到时返回空数组，父子成环不死循环', () => {
    expect(groupPath(groups, 99)).toEqual([])
    const looped = [
      { id: 1, title: 'A', p_id: 2 } as Group,
      { id: 2, title: 'B', p_id: 1 } as Group,
    ]
    expect(groupPath(looped, 1).length).toBeLessThan(25)
  })
})

describe('apiToMarkdown', () => {
  const md = apiToMarkdown(api, [])

  it('包含方法、地址与参数表', () => {
    expect(md).toContain('POST')
    expect(md).toContain('https://api.example.com/v1/user/list')
    expect(md).toContain('page')
    expect(md).toContain('code')
  })

  it('表格里的竖线与换行被转义，避免破坏 Markdown 结构', () => {
    const withPipe = apiToMarkdown({
      ...api,
      http_request_params: '[{"fieldName":"a|b","type":"string","required":false,"example":"x\\ny"}]',
    })
    expect(withPipe).toContain('a\\|b')
    expect(withPipe).not.toContain('| a|b |')
  })
})

describe('safeName', () => {
  it('过滤文件系统非法字符并保留可读部分', () => {
    const name = safeName('项目/名称: 含*非法?字符', 'fallback')
    expect(name).not.toMatch(/[/:*?"<>|]/)
    expect(name.length).toBeGreaterThan(0)
  })

  it('全空时用兜底名', () => {
    expect(safeName('   ', 'fallback')).toBe('fallback')
  })
})
