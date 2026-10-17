// JSON 参数表工具：展平规则与「生成行 + 人工填写行」的合并规则。
// 这两条规则直接决定导出的参数表长什么样，回归代价高，值得锁死。
import { describe, expect, it } from 'vitest'

import type { ApiParam } from '@/types/entities'
import { flattenJson, jsonToParams, mergeParams } from '@/utils/json-params'

describe('flattenJson', () => {
  it('对象逐层展开为点号路径', () => {
    const rows = flattenJson({ code: 200, data: { list: [{ id: 1 }] } })
    const paths = rows.map((r) => r.fieldName)
    // 数组字段本身是 data.list（type=array），元素内部路径带 [] 后缀。
    expect(paths).toEqual(['code', 'data', 'data.list', 'data.list[].id'])
  })

  it('整数与浮点区分类型，对象不填示例', () => {
    const rows = flattenJson({ n: 1, f: 1.5, s: 'x', b: true, o: { k: 1 }, arr: [1, 2] })
    const byName = new Map(rows.map((r) => [r.fieldName, r]))
    expect(byName.get('n')?.type).toBe('integer')
    expect(byName.get('f')?.type).toBe('number')
    expect(byName.get('s')?.type).toBe('string')
    expect(byName.get('b')?.type).toBe('boolean')
    expect(byName.get('o')?.example).toBe('')
    expect(byName.get('arr')?.type).toBe('array')
    expect(byName.get('arr')?.example).toBe('1')
  })

  it('顶层数组按首元素推断结构', () => {
    const paths = flattenJson([{ id: 1 }]).map((r) => r.fieldName)
    expect(paths).toEqual(['[].id'])
  })

  it('标量根值不产生任何行', () => {
    expect(flattenJson(42)).toEqual([])
  })
})

describe('jsonToParams', () => {
  it('空文本报 empty', () => {
    const res = jsonToParams('   ')
    expect(res.rows).toEqual([])
    expect(res.error).toBeTruthy()
  })

  it('非法 JSON 返回错误而不是抛出', () => {
    const res = jsonToParams('{oops')
    expect(res.rows).toEqual([])
    expect(res.error).toBeTruthy()
  })

  it('合法 JSON 正常展平', () => {
    const res = jsonToParams('{"a":{"b":1}}')
    expect(res.error).toBeUndefined()
    expect(res.rows.map((r) => r.fieldName)).toEqual(['a', 'a.b'])
  })
})

describe('mergeParams', () => {
  const gen = (names: string[]): ApiParam[] =>
    names.map((fieldName) => ({ fieldName, type: 'string', required: false, description: '', example: '' }))

  it('同名字段沿用已填写的说明与必填标记', () => {
    const existing: ApiParam[] = [
      { fieldName: 'id', type: 'integer', required: true, description: '主键', example: '1' },
    ]
    const merged = mergeParams(existing, gen(['id', 'name']))
    expect(merged).toHaveLength(2)
    expect(merged[0].description).toBe('主键')
    expect(merged[0].required).toBe(true)
    // 类型与示例以 JSON 为准
    expect(merged[0].type).toBe('string')
  })

  it('JSON 中不存在的人工行被保留在末尾', () => {
    const existing: ApiParam[] = [
      { fieldName: 'id', type: 'integer', required: false, description: '', example: '' },
      { fieldName: 'memo', type: 'string', required: false, description: '手工补充', example: '' },
    ]
    const merged = mergeParams(existing, gen(['id']))
    expect(merged.map((r) => r.fieldName)).toEqual(['id', 'memo'])
  })

  it('replace 为真时直接以生成结果覆盖', () => {
    const existing: ApiParam[] = [
      { fieldName: 'id', type: 'integer', required: true, description: '主键', example: '' },
    ]
    const merged = mergeParams(existing, gen(['name']), true)
    expect(merged.map((r) => r.fieldName)).toEqual(['name'])
  })
})
