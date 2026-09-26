<script setup lang="ts">
// 接口结构化字段编辑器：请求头部 / 请求参数 / 返回参数 / 响应样例 / 文档（放底部）。
// 新建与编辑共用；三段表格统一由 ApiParamTable 渲染，文档一节用 MdEditor 写 Markdown。
// 请求头直接以 ApiParam[] 承载（表格改的就是这份数组），提交前由调用方转回 ApiHeader[]。
import ApiParamTable from '@/components/ApiParamTable.vue'
import { useMdUpload } from '@/composables/md-upload'
import type { ApiParam } from '@/types/entities'
import { MdEditor } from 'md-editor-v3'
import 'md-editor-v3/lib/preview.css'
import 'md-editor-v3/lib/style.css'
import { NButton, NInput, NPopconfirm } from 'naive-ui'

const headerRows = defineModel<ApiParam[]>('headerRows', { required: true })
const reqParams = defineModel<ApiParam[]>('reqParams', { required: true })
const resParams = defineModel<ApiParam[]>('resParams', { required: true })
const sample = defineModel<string>('sample', { required: true })
const description = defineModel<string>('description', { required: true })

const onUploadImg = useMdUpload()

/** 响应 JSON 生成字段后，样例框为空则顺带填入，避免两处内容不一致。 */
function onSampleJson(text: string): void {
  if (!sample.value.trim()) sample.value = text.trim()
}
</script>

<template>
  <div class="editor">
    <api-param-table v-model="headerRows" kind="header" editable />

    <api-param-table v-model="reqParams" kind="request" editable />

    <section class="sec">
      <p class="meta tip">返回参数保存时自动应用项目字段映射。</p>
      <api-param-table v-model="resParams" kind="response" editable :json-source="sample" @json-applied="onSampleJson" />
    </section>

    <section class="sec">
      <div class="sec-hd">
        <h3 class="sec-title">响应样例（JSON）</h3>
        <n-popconfirm v-if="sample.trim()" @positive-click="sample = ''">
          <template #trigger>
            <n-button size="tiny" tertiary type="error">清空</n-button>
          </template>
          确认清空响应样例？
        </n-popconfirm>
      </div>
      <n-input
        v-model:value="sample"
        type="textarea"
        :rows="8"
        placeholder='{"code":200,"msg":"success","data":{"id":1,"name":"demo"}}'
      />
    </section>

    <section class="sec">
      <div class="sec-hd">
        <h3 class="sec-title">文档</h3>
        <span class="meta">该接口的说明文档，支持 Markdown</span>
      </div>
      <md-editor
        v-model="description"
        preview-theme="default"
        class="md-edit"
        @on-upload-img="onUploadImg"
      />
    </section>
  </div>
</template>

<style scoped>
.editor {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.sec {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.sec-hd {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.sec-title {
  margin: 0;
  font-size: 13px;
  font-weight: 600;
}

.tip {
  margin: 0 0 -2px;
}

.md-edit {
  width: 100%;
  height: 320px;
}
</style>
