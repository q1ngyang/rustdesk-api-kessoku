<template>
  <AuthLayout>
    <section class="register-card" aria-labelledby="register-title">
      <header><span>RustDesk API Kessoku</span><h2 id="register-title">{{ T('CreateAccount') }}</h2><p>{{ T('CreateAccountDescription') }}</p></header>
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top" @submit.prevent="submit">
        <el-form-item :label="T('Username')" prop="username"><el-input v-model.trim="form.username" autocomplete="username" size="large" :placeholder="T('EnterUsername')"><template #prefix><el-icon><User/></el-icon></template></el-input></el-form-item>
        <el-form-item :label="T('Email')" prop="email"><el-input v-model.trim="form.email" autocomplete="email" size="large" :placeholder="T('EnterEmail')"><template #prefix><el-icon><Message/></el-icon></template></el-input></el-form-item>
        <el-form-item :label="T('Password')" prop="password"><el-input v-model="form.password" autocomplete="new-password" type="password" show-password size="large" :placeholder="T('EnterPassword')"><template #prefix><el-icon><Lock/></el-icon></template></el-input></el-form-item>
        <el-form-item :label="T('ConfirmPassword')" prop="confirm_password"><el-input v-model="form.confirm_password" autocomplete="new-password" type="password" show-password size="large" @keyup.enter="submit"><template #prefix><el-icon><Key/></el-icon></template></el-input></el-form-item>
        <el-button class="register-card__submit" type="primary" size="large" native-type="submit" :loading="loading" @click="submit">{{ T('CreateAccount') }}</el-button>
        <el-button class="register-card__login" text @click="toLogin">{{ T('AlreadyHaveAccount') }} · {{ T('Login') }}</el-button>
      </el-form>
    </section>
  </AuthLayout>
</template>

<script setup>
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Key, Lock, Message, User } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import AuthLayout from '@/components/auth/AuthLayout.vue'
import { register } from '@/api/user'
import { useUserStore } from '@/store/user'
import { useAppStore } from '@/store/app'
import { T } from '@/utils/i18n'

const router = useRouter()
const userStore = useUserStore()
const formRef = ref(null)
const loading = ref(false)
const form = reactive({ username: '', email: '', password: '', confirm_password: '' })
const rules = {
  username: [{ required: true, message: T('ParamRequired', { param: T('Username') }), trigger: 'blur' }],
  password: [{ required: true, message: T('ParamRequired', { param: T('Password') }), trigger: 'blur' }],
  confirm_password: [
    { required: true, message: T('ParamRequired', { param: T('ConfirmPassword') }), trigger: 'blur' },
    { validator: (_, value, callback) => value === form.password ? callback() : callback(new Error(T('PasswordNotMatchConfirmPassword'))), trigger: 'blur' },
  ],
}
const submit = async () => {
  if (loading.value || !await formRef.value.validate().catch(() => false)) return
  loading.value = true
  try {
    const result = await register(form)
    userStore.saveUserData(result.data)
    useAppStore().loadConfig()
    ElMessage.success(T('OperationSuccess'))
    await router.push('/')
  } finally {
    loading.value = false
  }
}
const toLogin = () => router.push('/login')
</script>

<style scoped lang="scss">
.register-card { width: min(460px, 100%); padding: clamp(24px, 4vw, 38px); border: 1px solid var(--border-subtle); border-radius: 24px; background: color-mix(in srgb, var(--surface-1) 94%, transparent); box-shadow: var(--shadow-lg); backdrop-filter: blur(18px); }
.register-card header { margin-bottom: 25px; }.register-card header span { color: var(--primary); font-size: 10px; font-weight: 800; letter-spacing: .12em; text-transform: uppercase; }.register-card h2 { margin: 9px 0 7px; color: var(--text-primary); font-size: 30px; letter-spacing: -.045em; }.register-card header p { margin: 0; color: var(--text-tertiary); font-size: 13px; line-height: 1.6; }
.register-card :deep(.el-form-item) { margin-bottom: 16px; }.register-card__submit { width: 100%; height: 46px; margin: 4px 0 0; }.register-card__login { width: 100%; margin: 10px 0 0; color: var(--text-secondary); }
@media (max-width: 520px) { .register-card { padding: 22px 18px; border-radius: 20px; box-shadow: var(--shadow-md); }.register-card h2 { font-size: 27px; } }
</style>
