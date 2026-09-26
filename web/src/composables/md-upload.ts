// Markdown 编辑器的图片上传钩子：把选中文件传到 /file/upload 并回填 URL。
// 项目文档、接口文档编辑共用（避免各自维护一份）。
import { uploadImg } from '@/api/misc'
import { translate } from '@/i18n'
import { ErrCode } from '@/types/api'
import { useMessage } from 'naive-ui'

export function useMdUpload(): (files: File[], callback: (urls: string[]) => void) => Promise<void> {
  const message = useMessage()

  return async function onUploadImg(files: File[], callback: (urls: string[]) => void): Promise<void> {
    const urls: string[] = []
    for (const f of files) {
      const res = await uploadImg(f)
      if (res.code === ErrCode.Success && res.data) urls.push(res.data.url)
      else message.error(translate('upload.failed', { name: f.name }))
    }
    callback(urls)
  }
}
