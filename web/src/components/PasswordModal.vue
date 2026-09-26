<script setup lang="ts">
// 修改密码全局弹窗：账户面板与个人中心复用（弹层渲染在 body 下，不受浮层宽度限制）。
import { updatePwd } from '@/api/auth'
import { ErrCode } from '@/types/api'
import { NButton, NForm, NFormItem, NInput, NModal, useMessage } from 'naive-ui'
import { reactive } from 'vue'
import { useI18n } from 'vue-i18n'

const show = defineModel<boolean>('show', { required: true })
const message = useMessage()
const { t } = useI18n()
const pwd = reactive({ old_pwd: '', pwd: '', re_pwd: '' })

async function submit(): Promise<void> {
  if (!pwd.old_pwd || !pwd.pwd) {
    message.warning(t('user.pwdRequired'))
    return
  }
  if (pwd.pwd !== pwd.re_pwd) {
    message.warning(t('auth.pwdMismatch'))
    return
  }
  const res = await updatePwd({ ...pwd })
  if (res.code !== ErrCode.Success) {
    message.error(res.msg || t('user.pwdFailed'))
    return
  }
  message.success(t('user.pwdSaved'))
  Object.assign(pwd, { old_pwd: '', pwd: '', re_pwd: '' })
  show.value = false
}
</script>

<template>
  <n-modal v-model:show="show" preset="card" :title="t('user.passwordTitle')" style="width: 420px">
    <n-form label-placement="left" label-width="76">
      <n-form-item :label="t('user.oldPwd')">
        <n-input v-model:value="pwd.old_pwd" type="password" show-password-on="click" :placeholder="t('user.oldPwdPlaceholder')" />
      </n-form-item>
      <n-form-item :label="t('user.newPwd')">
        <n-input v-model:value="pwd.pwd" type="password" show-password-on="click" :placeholder="t('user.newPwdPlaceholder')" />
      </n-form-item>
      <n-form-item :label="t('auth.confirmPwd')">
        <n-input
          v-model:value="pwd.re_pwd"
          type="password"
          show-password-on="click"
          :placeholder="t('user.confirmNewPwd')"
          @keyup.enter="submit"
        />
      </n-form-item>
    </n-form>
    <template #footer>
      <div class="foot">
        <n-button size="small" @click="show = false">{{ t('common.cancel') }}</n-button>
        <n-button type="primary" size="small" @click="submit">{{ t('user.submitPwd') }}</n-button>
      </div>
    </template>
  </n-modal>
</template>

<style scoped>
.foot {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
</style>
