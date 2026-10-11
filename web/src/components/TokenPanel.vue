<script setup lang="ts">
// 个人访问令牌（PAT）管理：创建 / 列表 / 查看明文 / 删除。供桌面客户端同步鉴权（S7）。
// 明文由服务端 AES-256-GCM 加密存储（S15）：眼睛查看时调 /api/user/token/reveal 解密，
// 明文只在当前会话内存缓存（换浏览器/重开仍可再次查看），不落 localStorage。
import { createToken, deleteToken, listTokens, revealToken } from '@/api/auth'
import TablePager from '@/components/TablePager.vue'
import { ErrCode } from '@/types/api'
import type { UserToken } from '@/types/entities'
import { fmtTime } from '@/utils/format'
import { EyeOffOutline, EyeOutline } from '@vicons/ionicons5'
import { NButton, NCard, NForm, NFormItem, NIcon, NInput, NInputNumber, NSelect, NTable, NTag, useDialog, useMessage } from 'naive-ui'
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
const message = useMessage()
const dialog = useDialog()

const loading = ref(false)
const creating = ref(false)
const tokens = ref<UserToken[]>([])
const form = reactive({ name: '', scopes: 'sync', days: 0 })

// 令牌列表客户端分页（接口不分页，见服务端 ListTokens）。
const pageNum = ref(1)
const pageSize = ref(10)
const paged = computed(() => tokens.value.slice((pageNum.value - 1) * pageSize.value, pageNum.value * pageSize.value))

// 明文会话缓存：id → 明文；眼睛隐藏即清除，再次查看走服务端解密。
const plainCache = ref<Record<number, string>>({})
const revealingId = ref(0)

const scopeOptions = [
  { label: t('user.tokenScopeSync'), value: 'sync' },
  { label: t('user.tokenScopeAll'), value: 'all' },
]

const plainOf = (row: UserToken): string => plainCache.value[row.id] ?? ''

/** 隐藏态展示：pat_ 前缀 + 前几位 + 省略号。 */
function maskedPat(row: UserToken): string {
  const raw = plainOf(row)
  if (raw) return raw.slice(0, 10) + '••••'
  return 'pat_••••••••'
}

/** 眼睛切换显隐；未加载明文时请求服务端解密。 */
async function toggleReveal(row: UserToken): Promise<void> {
  if (revealingId.value) return
  const id = row.id
  if (plainCache.value[id]) {
    const next = { ...plainCache.value }
    delete next[id]
    plainCache.value = next
    return
  }
  revealingId.value = id
  try {
    const res = await revealToken(id)
    if (res.code === ErrCode.Success && res.data?.token) {
      plainCache.value = { ...plainCache.value, [id]: res.data.token }
    } else {
      message.error(res.msg || t('user.tokenRevealFailed'))
    }
  } catch {
    // 网络错误/404（旧版服务）等：axios reject 不会带统一响应体，需显式提示。
    message.error(t('user.tokenRevealFailed'))
  } finally {
    revealingId.value = 0
  }
}

/** Go time.Time 未设置时序列化为 0001-01-01…，视为「不过期」。 */
const isZeroTime = (v: string): boolean => !v || v.startsWith('0001')

async function load(): Promise<void> {
  loading.value = true
  try {
    const res = await listTokens()
    if (res.code === ErrCode.Success) {
      tokens.value = res.data ?? []
      pageNum.value = 1 // 重新加载回到第一页
    }
  } finally {
    loading.value = false
  }
}

async function submit(): Promise<void> {
  if (!form.name.trim()) {
    message.warning(t('user.tokenNamePlaceholder'))
    return
  }
  creating.value = true
  try {
    const res = await createToken({
      name: form.name.trim(),
      scopes: form.scopes,
      days: form.days || 0,
    })
    if (res.code !== ErrCode.Success) {
      message.error(res.msg || t('common.saveFailed'))
      return
    }
    message.success(t('user.tokenCreateSuccess'))
    form.name = ''
    form.days = 0
    await load()
  } finally {
    creating.value = false
  }
}

function copy(text: string): void {
  void navigator.clipboard.writeText(text).then(
    () => message.success(t('common.copied')),
    () => message.error(t('common.copyFailed')),
  )
}

function remove(row: UserToken): void {
  dialog.warning({
    title: t('user.tokenDelete'),
    content: t('user.tokenDeleteConfirm'),
    positiveText: t('user.tokenDelete'),
    negativeText: t('common.cancel'),
    onPositiveClick: async () => {
      const res = await deleteToken(row.id)
      if (res.code === ErrCode.Success) {
        message.success(t('user.tokenDeleteDone'))
        await load()
      } else {
        message.error(res.msg || t('common.opFailed'))
      }
    },
  })
}

onMounted(load)
</script>

<template>
  <n-card :bordered="true" class="token-card">
    <template #header>
      <div class="token-hd">
        <span class="k">{{ t('user.tokenTitle') }}</span>
        <span class="hint">{{ t('user.tokenSub') }}</span>
      </div>
    </template>

    <n-form label-placement="left" label-width="72" class="create-form">
      <n-form-item :label="t('user.tokenName')">
        <div class="field">
          <n-input v-model:value="form.name" :placeholder="t('user.tokenNamePlaceholder')" />
          <span class="field-hint">{{ t('user.tokenNameHint') }}</span>
        </div>
      </n-form-item>
      <n-form-item :label="t('user.tokenScopes')">
        <div class="field">
          <n-select v-model:value="form.scopes" :options="scopeOptions" />
          <span class="field-hint">{{ t('user.tokenScopeHint') }}</span>
        </div>
      </n-form-item>
      <n-form-item :label="t('user.tokenDays')">
        <div class="field">
          <n-input-number
            v-model:value="form.days"
            :min="0"
            :show-button="false"
            :placeholder="t('user.tokenDaysHint')"
            class="days"
          />
        </div>
      </n-form-item>
      <n-form-item>
        <n-button type="primary" size="small" :loading="creating" @click="submit">{{ t('user.tokenCreate') }}</n-button>
      </n-form-item>
    </n-form>

    <n-table :single-line="false" size="small" class="token-table">
      <thead>
        <tr>
          <th>{{ t('user.token') }}</th>
          <th>{{ t('user.tokenName') }}</th>
          <th>{{ t('user.tokenScopes') }}</th>
          <th>{{ t('user.tokenExpire') }}</th>
          <th>{{ t('user.tokenStatus') }}</th>
          <th class="col-ops">{{ t('common.actions') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="row in paged" :key="row.id">
          <td class="pat-cell">
            <span class="pat-mask" :title="plainCache[row.id] || ''">
              {{ plainOf(row) ? plainOf(row) : maskedPat(row) }}
            </span>
            <n-button
              text
              size="small"
              class="icon-btn"
              :loading="revealingId === row.id"
              :title="plainOf(row) ? t('user.tokenHide') : t('user.tokenShow')"
              @click="toggleReveal(row)"
            >
              <n-icon :component="plainOf(row) ? EyeOffOutline : EyeOutline" />
            </n-button>
          </td>
          <td>{{ row.name }}</td>
          <td class="muted">{{ row.scopes || '—' }}</td>
          <td class="muted">{{ isZeroTime(row.expire_at) ? t('user.tokenExpireNever') : fmtTime(row.expire_at) }}</td>
          <td>
            <n-tag v-if="row.revoked === 1" size="small" type="error">{{ t('user.tokenRevoked') }}</n-tag>
            <n-tag v-else size="small" type="success">{{ t('user.tokenActive') }}</n-tag>
          </td>
          <td class="col-ops">
            <n-button
              size="small"
              class="op-btn"
              :disabled="!plainOf(row)"
              :title="t('common.copy')"
              @click="copy(plainOf(row))"
            >
              {{ t('common.copy') }}
            </n-button>
            <n-button size="small" type="error" class="op-btn" @click="remove(row)">
              {{ t('user.tokenDelete') }}
            </n-button>
          </td>
        </tr>
        <tr v-if="!loading && !tokens.length">
          <td colspan="6" class="muted empty">{{ t('user.tokenEmpty') }}</td>
        </tr>
      </tbody>
    </n-table>
    <table-pager
      :total="tokens.length"
      :page-num="pageNum"
      :page-size="pageSize"
      @update:page-num="pageNum = $event"
    />
  </n-card>
</template>

<style scoped>
.token-hd {
  display: flex;
  align-items: baseline;
  gap: 8px;
}

.token-hd .k {
  font-weight: 600;
}

.hint {
  color: var(--app-text-muted, #666);
  font-size: 12px;
}

.create-form {
  max-width: 460px;
  margin-bottom: 8px;
}

.field {
  display: flex;
  flex-direction: column;
  width: 100%;
}

.field-hint {
  font-size: 12px;
  color: var(--app-text-muted, #666);
  line-height: 18px;
  margin-top: 2px;
}

.days {
  width: 140px;
}

.pat-cell {
  max-width: 240px;
}

.pat-mask {
  display: inline-block;
  max-width: 170px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  vertical-align: middle;
  font-family: var(--app-font-mono, monospace);
  font-size: 12px;
}

.icon-btn,
.op-btn {
  vertical-align: middle;
}

.op-btn {
  margin-right: 8px;
}

.token-table :deep(td) {
  padding: 8px 12px;
}

.token-table .empty {
  text-align: center;
  padding: 24px 0;
}

.muted {
  color: var(--app-text-muted, #666);
}
</style>
