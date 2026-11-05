// 请求头 / 参数行的 enabled 往返：缺省=启用、只有 false 落键、保存前清洗空行。
// 这三条是与客户端同步契约的接缝，回归时最先坏在这里。
import { describe, expect, it } from 'vitest'

import type { ApiHeader, ApiParam } from '@/types/entities'
import { cleanParams, headersToParams, paramsToHeaders, rowEnabled } from '@/utils/api-params'

describe('api-params enabled', () => {
  it('缺省视为启用，且不写 enabled 键（老数据保存后字节稳定）', () => {
    const rows = headersToParams([{ name: 'X-Token', value: 't' }])
    expect(rows).toEqual([
      { fieldName: 'X-Token', type: '', required: false, description: '', example: 't' },
    ])
    expect(rowEnabled(rows[0])).toBe(true)
    expect(paramsToHeaders(rows)).toEqual([{ name: 'X-Token', value: 't', description: '' }])
  })

  it('停用位往返保留，只在 false 时写键', () => {
    const headers: ApiHeader[] = [{ name: 'X-Off', value: '0', enabled: false }]
    const rows = headersToParams(headers)
    expect(rows[0]).toMatchObject({ fieldName: 'X-Off', enabled: false })
    expect(paramsToHeaders(rows)[0]).toMatchObject({ name: 'X-Off', enabled: false })
  })

  it('cleanParams 去掉占位空行、修剪名称并保留停用位', () => {
    const rows: ApiParam[] = [
      { fieldName: ' page ', type: 'integer', required: true, description: '', example: '1' },
      { fieldName: 'off', type: 'string', required: false, description: '', example: '', enabled: false },
      { fieldName: '   ', type: 'string', required: false, description: '', example: '' },
    ]
    expect(cleanParams(rows)).toEqual([
      { fieldName: 'page', type: 'integer', required: true, description: '', example: '1' },
      { fieldName: 'off', type: 'string', required: false, description: '', example: '', enabled: false },
    ])
  })
})
