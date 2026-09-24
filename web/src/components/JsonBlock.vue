<script setup lang="ts">
// JSON 展示块：自动美化 + 一键复制；非法 JSON 原样展示。
import { NButton, useMessage } from 'naive-ui';
import { computed } from 'vue';

const props = defineProps<{ raw: string; empty?: string }>()
const message = useMessage()

const text = computed(() => {
    if (!props.raw) return ''
    try {
        return JSON.stringify(JSON.parse(props.raw), null, 2)
    } catch {
        return props.raw
    }
})

async function copy(): Promise<void> {
    try {
        await navigator.clipboard.writeText(text.value)
        message.success('已复制到剪贴板')
    } catch {
        message.warning('浏览器剪贴板不可用，请手动选择复制')
    }
}
</script>

<template>
    <div class="code-block">
        <n-button v-if="text" class="code-copy" size="tiny" tertiary @click="copy">复制</n-button>
        <pre v-if="text" class="json">{{ text }}</pre>
        <p v-else class="muted placeholder">{{ empty || '暂无内容' }}</p>
    </div>
</template>

<style scoped>
.json {
    margin: 0;
    padding: 4px 20px;
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    font-size: 12.5px;
    line-height: 1.7;
    white-space: pre;
    color: #24292f;
}

.placeholder {
    margin: 0;
    padding: 16px 20px;
}
</style>
