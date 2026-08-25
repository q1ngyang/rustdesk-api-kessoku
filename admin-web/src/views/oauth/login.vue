<template>
  <AuthActionLayout>
    <section class="oauth-action">
      <span class="oauth-action__icon"><el-icon><Connection/></el-icon></span>
      <span class="oauth-action__eyebrow">{{ T('AuthorizationRequest') }}</span>
      <h1>{{ T('OauthLogining') }}</h1>
      <p>{{ T('OauthCloseNote') }}</p>
      <dl class="oauth-action__details">
        <div><dt>{{ T('Device') }}</dt><dd>{{ oauthInfo.device_name || '—' }}</dd></div>
        <div><dt>ID</dt><dd>{{ oauthInfo.id || '—' }}</dd></div>
      </dl>
      <el-alert v-if="resStatus" :title="T('AuthorizationCompleted')" type="success" :closable="false" show-icon/>
      <div class="oauth-action__buttons">
        <el-button v-if="!resStatus" type="primary" size="large" @click="toConfirm">{{ T('ConfirmOauth') }}</el-button>
        <el-button size="large" @click="out">{{ T('Close') }}</el-button>
      </div>
    </section>
  </AuthActionLayout>
</template>

<script setup>
import { ref } from 'vue'
import { useRoute } from 'vue-router'
import { Connection } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import AuthActionLayout from '@/components/auth/AuthActionLayout.vue'
import { info, confirm } from '@/api/oauth'
import { T } from '@/utils/i18n'

const oauthInfo = ref({})
const route = useRoute()
const code = route.params?.code
const resStatus = ref(0)

const getInfo = async () => {
  const result = await info({ code }).catch(() => false)
  if (result) oauthInfo.value = result.data
}
const toConfirm = async () => {
  const result = await confirm({ code }).catch(() => false)
  if (!result) return
  resStatus.value = 1
  ElMessage.success(T('OperationSuccessAndCloseAfter3Seconds'))
  setTimeout(() => out(), 3000)
}
const out = () => window.close()
getInfo()
</script>

<style scoped lang="scss">
.oauth-action { text-align: center; }.oauth-action__icon { display: grid; width: 52px; height: 52px; margin: 0 auto 17px; place-items: center; border-radius: 16px; background: var(--primary-soft); color: var(--primary); font-size: 23px; }.oauth-action__eyebrow { color: var(--primary); font-size: 10px; font-weight: 800; letter-spacing: .1em; text-transform: uppercase; }.oauth-action h1 { margin: 8px 0 7px; color: var(--text-primary); font-size: 25px; letter-spacing: -.035em; }.oauth-action > p { margin: 0; color: var(--text-tertiary); font-size: 12px; line-height: 1.65; }
.oauth-action__details { display: grid; gap: 1px; overflow: hidden; margin: 24px 0 18px; border: 1px solid var(--border-subtle); border-radius: 14px; background: var(--border-subtle); text-align: left; }.oauth-action__details div { display: flex; min-width: 0; align-items: center; justify-content: space-between; gap: 18px; padding: 13px 15px; background: var(--surface-2); }.oauth-action__details dt { color: var(--text-tertiary); font-size: 11px; }.oauth-action__details dd { overflow: hidden; margin: 0; color: var(--text-primary); font-size: 12px; font-weight: 700; text-overflow: ellipsis; white-space: nowrap; }.oauth-action__buttons { display: grid; gap: 9px; margin-top: 18px; }.oauth-action__buttons .el-button { width: 100%; margin: 0; }
</style>
