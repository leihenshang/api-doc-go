// OpenAPI 文本解析：先 JSON，再 YAML；兼容 Bruno 导出里反引号包裹的标量。
import { parse as parseYaml } from 'yaml'

export function parseOpenAPIText(text: string): unknown {
  try {
    return JSON.parse(text)
  } catch {
    try {
      return parseYaml(text)
    } catch {
      return parseYaml(text.replace(/^(\s*(?:- )?[\w-]+:\s*)`([^`\n]*)`(\s*)$/gm, "$1'$2'$3"))
    }
  }
}
