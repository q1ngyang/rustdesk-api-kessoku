<template>
  <AuthLayout show-starry-logo>
    <section class="auth-card" aria-labelledby="login-title">
      <header class="auth-card__header">
        <span class="auth-card__kicker">RustDesk API Kessoku</span>
        <h2 id="login-title">{{ T('WelcomeBack') }}</h2>
        <p>{{ disablePwd ? T('ChooseIdentityProvider') : T('SignInToContinue') }}</p>
      </header>

      <el-form v-if="!disablePwd" label-position="top" class="auth-form" @submit.prevent="login">
        <el-form-item :label="T('Username')">
          <el-input v-model.trim="form.username" name="username" autocomplete="username" :placeholder="T('EnterUsername')" size="large">
            <template #prefix><el-icon><User/></el-icon></template>
          </el-input>
        </el-form-item>
        <el-form-item v-if="!form.challenge" :label="T('Password')">
          <el-input v-model="form.password" name="password" autocomplete="current-password" type="password" show-password :placeholder="T('EnterPassword')" size="large" @keyup.enter="login">
            <template #prefix><el-icon><Lock/></el-icon></template>
          </el-input>
        </el-form-item>
		<el-form-item v-if="form.challenge" :label="T('AuthenticatorCode')">
		  <el-input v-model.trim="form.tfa_code" name="one-time-code" autocomplete="one-time-code" inputmode="numeric" maxlength="6" :placeholder="T('EnterAuthenticatorCode')" size="large" @keyup.enter="login">
			<template #prefix><el-icon><Key/></el-icon></template>
		  </el-input>
		</el-form-item>
        <el-form-item v-if="captchaCode && !form.challenge" :label="T('Captcha')">
          <el-input v-model.trim="form.captcha" :placeholder="T('EnterCaptcha')" size="large" @keyup.enter="login">
            <template #prefix><el-icon><Key/></el-icon></template>
            <template #append>
              <button class="captcha-button" type="button" :aria-label="T('RefreshCaptcha')" @click="loadCaptcha"><img :src="captchaCode.b64" alt="captcha"/></button>
            </template>
          </el-input>
        </el-form-item>
        <el-button class="auth-card__primary" type="primary" native-type="submit" size="large" :loading="loginLoading" @click="login">{{ T('Login') }}</el-button>
        <el-button v-if="allowRegister" class="auth-card__secondary" size="large" @click="register">{{ T('CreateAccount') }}</el-button>
      </el-form>

      <div v-if="options.length > 0 && !disablePwd" class="auth-divider"><span>{{ T('or login in with') }}</span></div>
      <div v-if="options.length" class="oidc-options">
        <el-button v-for="option in options" :key="option.name" class="oidc-button" size="large" @click="handleOIDCLogin(option.name)">
          <img :src="getProviderImage(option.name)" alt=""/><span>{{ T(option.name) }}</span><el-icon><ArrowRight/></el-icon>
        </el-button>
      </div>
      <p class="auth-card__privacy"><el-icon><Lock/></el-icon>{{ T('CredentialsStayPrivate') }}</p>
    </section>
  </AuthLayout>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowRight, Key, Lock, User } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import AuthLayout from '@/components/auth/AuthLayout.vue'
import { useUserStore } from '@/store/user'
import { T } from '@/utils/i18n'
import { browserDeviceIdentity } from '@/utils/device'
import { captcha, loginOptions } from '@/api/login'
import { getCode, removeCode } from '@/utils/auth'
import googleImage from '@/assets/google.png'
import githubImage from '@/assets/github.png'
import oidcImage from '@/assets/oidc.png'

const userStore = useUserStore()
const route = useRoute()
const router = useRouter()
const options = reactive([])
const captchaCode = ref('')
const allowRegister = ref(false)
const disablePwd = ref(false)
const loginLoading = ref(false)
const redirect = route.query?.redirect
const userAgent = navigator.userAgent
let platform = navigator.platform || 'web'
if (platform.startsWith('Mac')) platform = 'mac'
else if (platform.startsWith('Win')) platform = 'windows'
else if (platform.startsWith('Linux armv')) platform = 'android'
else if (platform.startsWith('Linux')) platform = 'linux'
let browser = 'Unknown Browser'
if (/edg/i.test(userAgent)) browser = 'Edge'
else if (/chrome|crios/i.test(userAgent)) browser = 'Chrome'
else if (/firefox|fxios/i.test(userAgent)) browser = 'Firefox'
else if (/safari/i.test(userAgent)) browser = 'Safari'

const deviceIdentity = browserDeviceIdentity()
const form = reactive({ username: '', password: '', platform, device_id: deviceIdentity.device_id, uuid: deviceIdentity.uuid, captcha: '', captcha_id: '', challenge: '', tfa_code: '' })
const providerImages = { google: googleImage, github: githubImage, oidc: oidcImage, default: oidcImage }
const getProviderImage = provider => providerImages[provider.toLowerCase()] || providerImages.default

const login = async () => {
  if (loginLoading.value) return
  if (!form.username || (!form.challenge && !form.password)) {
    ElMessage.warning(T('EnterCredentials'))
    return
  }
  loginLoading.value = true
  try {
	if (form.challenge && !/^\d{6}$/.test(form.tfa_code)) {
	  ElMessage.warning(T('EnterAuthenticatorCode'))
	  return
	}
    const result = await userStore.login(form)
	if (result?.requires_two_factor) {
	  form.challenge = result.challenge
	  form.password = ''
	  ElMessage.info(T('TwoFactorRequired'))
	  return
	}
    ElMessage.success(T('LoginSuccess'))
    await router.push({ path: redirect || '/', replace: true })
  } catch (error) {
    if (error?.code === 110) await loadCaptcha()
  } finally {
    loginLoading.value = false
  }
}
const loadCaptcha = async () => {
  const result = await captcha().catch(() => false)
  if (!result?.data?.captcha) return
  captchaCode.value = result.data.captcha
  form.captcha_id = result.data.captcha.id
}
const handleOIDCLogin = provider => userStore.oidc(provider, platform, browser)
const register = () => router.push('/register')
const loadLoginOptions = async () => {
  const result = await loginOptions().catch(() => false)
  if (!result?.data) return
  options.splice(0, options.length, ...(result.data.ops || []).map(name => ({ name })))
  disablePwd.value = Boolean(result.data.disable_pwd)
  allowRegister.value = Boolean(result.data.register)
  if (result.data.need_captcha) await loadCaptcha()
  if (result.data.auto_oidc && options[0]) handleOIDCLogin(options[0].name)
}
onMounted(async () => {
  const code = getCode()
  if (!code) return loadLoginOptions()
  const user = await userStore.query(code)
  if (user) {
    removeCode()
    ElMessage.success(T('LoginSuccess'))
    await router.push({ path: redirect || '/', replace: true })
  }
})
</script>

<style scoped lang="scss">
.auth-card { width: min(430px, 100%); padding: clamp(24px, 4vw, 38px); border: 1px solid var(--border-subtle); border-radius: 24px; background: color-mix(in srgb, var(--surface-1) 94%, transparent); box-shadow: var(--shadow-lg); backdrop-filter: blur(18px); }
.auth-card__header { margin-bottom: 28px; }.auth-card__kicker { color: var(--primary); font-size: 10px; font-weight: 800; letter-spacing: .12em; text-transform: uppercase; }.auth-card h2 { margin: 9px 0 7px; color: var(--text-primary); font-size: 30px; letter-spacing: -.045em; }.auth-card__header p { margin: 0; color: var(--text-tertiary); font-size: 13px; line-height: 1.6; }
.auth-form :deep(.el-form-item__label) { padding-bottom: 7px; }.auth-card__primary,.auth-card__secondary { width: 100%; height: 46px; margin: 4px 0 0; }.auth-card__secondary { margin-top: 10px; }
.captcha-button { display: block; height: 38px; padding: 0; overflow: hidden; border: 0; background: #fff; cursor: pointer; }.captcha-button img { display: block; width: 126px; height: 38px; object-fit: contain; }:deep(.el-input-group__append) { padding: 0; overflow: hidden; }
.auth-divider { display: flex; align-items: center; gap: 12px; margin: 23px 0 17px; color: var(--text-tertiary); font-size: 10px; }.auth-divider::before,.auth-divider::after { height: 1px; flex: 1; background: var(--border-subtle); content: ''; }
.oidc-options { display: grid; gap: 9px; }.oidc-button { width: 100%; height: 44px; margin: 0; justify-content: flex-start; border-color: var(--border-subtle); background: var(--surface-2); color: var(--text-secondary); }.oidc-button img { width: 20px; height: 20px; margin-right: 4px; object-fit: contain; }.oidc-button span { flex: 1; text-align: left; }
.auth-card__privacy { display: flex; align-items: center; justify-content: center; gap: 5px; margin: 20px 0 0; color: var(--text-tertiary); font-size: 10px; line-height: 1.5; text-align: center; }
@media (max-width: 520px) { .auth-card { padding: 22px 18px; border-radius: 20px; box-shadow: var(--shadow-md); }.auth-card h2 { font-size: 27px; } }
</style>
