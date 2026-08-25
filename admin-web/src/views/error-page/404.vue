<template>
  <AuthActionLayout>
    <section class="not-found">
      <span class="not-found__code">404</span>
      <h1>{{ T('PageNotFound') }}</h1>
      <p>{{ T('PageNotFoundDescription') }}</p>
      <el-button type="primary" size="large" @click="router.replace('/')">{{ T('BackToWorkspace') }}</el-button>
    </section>
  </AuthActionLayout>
</template>

<script setup>
import { useRouter } from 'vue-router'
import AuthActionLayout from '@/components/auth/AuthActionLayout.vue'
import { T } from '@/utils/i18n'
const router = useRouter()
</script>

<style scoped lang="scss">
.not-found { text-align: center; }.not-found__code { display: inline-block; padding: 7px 12px; border: 1px solid var(--primary-border); border-radius: 999px; background: var(--primary-soft); color: var(--primary); font-size: 11px; font-weight: 800; letter-spacing: .12em; }.not-found h1 { margin: 18px 0 8px; color: var(--text-primary); font-size: 29px; letter-spacing: -.04em; }.not-found p { margin: 0 auto 24px; color: var(--text-tertiary); font-size: 13px; line-height: 1.7; }.not-found .el-button { width: 100%; }
</style>
