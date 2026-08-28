<template>
  <main class="auth-shell">
    <div class="auth-shell__background" aria-hidden="true">
      <img v-if="activeBackground" class="auth-shell__background-image" :src="activeBackground" alt=""/>
    </div>
    <AuthPreferenceControls/>
    <div class="auth-shell__glow auth-shell__glow--one"></div>
    <div class="auth-shell__glow auth-shell__glow--two"></div>
    <section class="auth-story">
      <div class="auth-story__brand">
        <template v-if="showStarryLogo"><DeploymentAsset class="auth-story__logo" variant="logo" width="148" :light-url="branding.brand_logo_light_url" :dark-url="branding.brand_logo_dark_url" alt=""/><span class="auth-story__brand-copy"><strong>{{ loginBrandLines[0] }}</strong><small>{{ loginBrandLines[1] }}</small></span></template>
        <KessokuBrand v-else/>
      </div>
      <div class="auth-story__content">
        <span class="auth-story__eyebrow">{{ T('SecureManagementConsole') }}</span>
        <h1>{{ branding.login_heading || T('WelcomeToKessoku') }}</h1>
        <p>{{ branding.login_copy || T('AuthIntroduction') }}</p>
		<BrandCustomContent v-if="branding.login_custom_html" class="auth-story__custom" :html="branding.login_custom_html" :css="branding.login_custom_css"/>
        <div class="auth-story__features">
          <div><el-icon><Monitor/></el-icon><span>{{ T('UnifiedDeviceManagement') }}</span></div>
          <div><el-icon><Connection/></el-icon><span>{{ T('ServerHealthAtAGlance') }}</span></div>
          <div><el-icon><Lock/></el-icon><span>{{ T('ExistingPermissionModel') }}</span></div>
        </div>
      </div>
      <BrandFooter class="auth-story__footer" :html="branding.footer_html"/>
    </section>
    <section class="auth-entry">
      <div class="auth-entry__mobile-brand">
        <template v-if="showStarryLogo"><DeploymentAsset class="auth-story__logo" variant="logo" width="132" :light-url="branding.brand_logo_light_url" :dark-url="branding.brand_logo_dark_url" alt=""/><span class="auth-story__brand-copy"><strong>{{ loginBrandLines[0] }}</strong><small>{{ loginBrandLines[1] }}</small></span></template>
        <KessokuBrand v-else/>
      </div>
      <slot/>
    </section>
  </main>
</template>

<script setup>
import { computed } from 'vue'
import { useDark } from '@vueuse/core'
import { Connection, Lock, Monitor } from '@element-plus/icons-vue'
import AuthPreferenceControls from '@/components/auth/AuthPreferenceControls.vue'
import BrandFooter from '@/components/brand/BrandFooter.vue'
import BrandCustomContent from '@/components/brand/BrandCustomContent.vue'
import DeploymentAsset from '@/components/brand/DeploymentAsset.vue'
import KessokuBrand from '@/components/brand/KessokuBrand.vue'
import { T } from '@/utils/i18n'
import { useAppStore } from '@/store/app'
defineProps({ showStarryLogo: Boolean })
const appStore = useAppStore()
const isDark = useDark({ storageKey: 'kessoku-theme' })
const branding = computed(() => appStore.setting.branding)
const activeBackground = computed(() => isDark.value
  ? branding.value.login_background_dark_url
  : branding.value.login_background_light_url)
const loginBrandLines = computed(() => {
  const value = (branding.value.login_kicker || 'RustDesk API\nKESSOKU').trim()
  const explicit = value.split(/\r?\n/).map(part => part.trim()).filter(Boolean)
  if (explicit.length > 1) return [explicit[0], explicit.slice(1).join(' ')]
  const legacy = value.match(/^(.*?)(?:\s+)(KESSOKU)$/i)
  return legacy ? [legacy[1], legacy[2]] : [value, 'KESSOKU']
})
</script>

<style scoped lang="scss">
.auth-shell {
  position: relative;
  display: grid;
  min-height: 100dvh;
  overflow: hidden;
  grid-template-columns: minmax(420px, .93fr) minmax(460px, 1.07fr);
  background: var(--app-bg);
}
.auth-shell__background { position: absolute; inset: 0; z-index: 0; overflow: hidden; pointer-events: none; }
.auth-shell__background::after { position: absolute; inset: 0; background: color-mix(in srgb, var(--app-bg) 78%, transparent); content: ''; }
.auth-shell__background-image { width: 100%; height: 100%; object-fit: cover; }
.auth-shell::before {
  position: absolute;
  inset: 0;
  pointer-events: none;
  background-image: linear-gradient(rgba(98, 113, 143, .04) 1px, transparent 1px), linear-gradient(90deg, rgba(98, 113, 143, .04) 1px, transparent 1px);
  background-size: 32px 32px;
  content: '';
}
.auth-shell__glow { position: absolute; border-radius: 50%; pointer-events: none; }
.auth-shell__glow--one { top: -240px; right: 26%; width: 520px; height: 520px; background: radial-gradient(circle, rgba(103, 119, 239, .15), transparent 68%); }
.auth-shell__glow--two { bottom: -250px; left: -80px; width: 480px; height: 480px; background: radial-gradient(circle, rgba(237, 145, 182, .12), transparent 68%); }
.auth-story {
  position: relative;
  z-index: 1;
  display: flex;
  min-height: 100dvh;
  flex-direction: column;
  padding: clamp(28px, 5vw, 64px);
  border-right: 1px solid var(--border-subtle);
  background: color-mix(in srgb, var(--surface-1) 72%, transparent);
  backdrop-filter: blur(12px);
}
.auth-story__brand { display: flex; min-height: 52px; align-items: center; gap: 16px; }
.auth-story__logo { max-width: min(148px, 42%); }
.auth-story__brand-copy { display: flex; min-width: 0; flex-direction: column; line-height: 1.1; }
.auth-story__brand-copy strong { overflow: hidden; color: var(--text-primary); font-size: 17px; font-weight: 780; letter-spacing: -.025em; text-overflow: ellipsis; white-space: nowrap; }
.auth-story__brand-copy small { margin-top: 5px; overflow: hidden; color: var(--text-tertiary); font-size: 12px; font-weight: 700; letter-spacing: .14em; text-overflow: ellipsis; text-transform: uppercase; white-space: nowrap; }
.auth-story__content { width: min(560px, 100%); margin: clamp(42px, 10vh, 104px) 0 auto; padding: 24px 0 44px; }
.auth-story__eyebrow { display: inline-flex; padding: 7px 11px; border: 1px solid var(--primary-border); border-radius: 999px; background: var(--primary-soft); color: var(--primary); font-size: 11px; font-weight: 760; letter-spacing: .04em; }
.auth-story h1 { max-width: 560px; margin: 22px 0 16px; overflow-wrap: anywhere; color: var(--text-primary); font-size: clamp(40px, 5vw, 68px); line-height: 1.08; letter-spacing: -.055em; text-wrap: balance; }
.auth-story p { max-width: 530px; margin: 0; color: var(--text-secondary); font-size: 15px; line-height: 1.8; }
.auth-story__custom { max-width: 530px; margin-top: 20px; }
.auth-story__features { display: grid; gap: 10px; margin-top: 34px; }
.auth-story__features div { display: flex; align-items: center; gap: 11px; color: var(--text-secondary); font-size: 13px; font-weight: 620; }
.auth-story__features .el-icon { display: grid; width: 32px; height: 32px; place-items: center; border-radius: 10px; background: var(--surface-1); color: var(--primary); box-shadow: var(--shadow-sm); }
.auth-story__features div:nth-child(2) .el-icon { background: color-mix(in srgb, var(--kessoku-yellow) 17%, var(--surface-1)); color: #a97d0c; }
.auth-story__features div:nth-child(3) .el-icon { background: color-mix(in srgb, var(--kessoku-pink) 15%, var(--surface-1)); color: #bd5d84; }
.auth-story__footer { justify-content: flex-start; min-height: 28px; font-size: 11px; font-weight: 680; }
.auth-story__footer :deep(a) { display: inline-flex; align-items: center; gap: 12px; color: var(--text-tertiary); text-decoration: none; transition: color var(--motion-fast); }
.auth-story__footer :deep(a:hover) { color: var(--text-primary); }
.auth-story__footer :deep(a span + span) { display: inline-flex; align-items: center; gap: 6px; padding-left: 12px; border-left: 1px solid var(--border-subtle); }
.auth-story__footer :deep(a span + span::before) { width: 15px; height: 15px; background: currentColor; content: ''; mask: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24'%3E%3Cpath d='M12 .7a11.5 11.5 0 0 0-3.64 22.41c.58.1.79-.25.79-.56v-2.23c-3.22.7-3.9-1.37-3.9-1.37-.52-1.34-1.28-1.7-1.28-1.7-1.05-.71.08-.7.08-.7 1.16.08 1.77 1.19 1.77 1.19 1.03 1.77 2.7 1.26 3.36.96.1-.75.4-1.26.73-1.55-2.57-.29-5.27-1.28-5.27-5.68 0-1.26.45-2.28 1.19-3.09-.12-.29-.52-1.47.11-3.05 0 0 .97-.31 3.16 1.18A10.9 10.9 0 0 1 12 6.12c.98 0 1.96.13 2.88.39 2.19-1.49 3.15-1.18 3.15-1.18.63 1.58.23 2.76.11 3.05.74.81 1.19 1.83 1.19 3.09 0 4.41-2.7 5.38-5.28 5.67.42.36.79 1.07.79 2.16v3.25c0 .31.21.67.8.56A11.5 11.5 0 0 0 12 .7Z'/%3E%3C/svg%3E") center / contain no-repeat; }
.auth-entry { position: relative; z-index: 1; display: grid; min-width: 0; place-items: center; padding: 28px; }
.auth-entry__mobile-brand { display: none; }
@media (max-width: 900px) {
  .auth-shell { display: block; }
  .auth-story { display: none; }
  .auth-entry { min-height: 100dvh; padding: max(22px, env(safe-area-inset-top)) 18px max(22px, env(safe-area-inset-bottom)); }
  .auth-entry__mobile-brand { display: flex; width: min(430px, 100%); margin: 0 auto 24px; align-items: center; gap: 13px; }
  .auth-entry__mobile-brand .auth-story__logo { max-width: 132px; }
  .auth-entry__mobile-brand .auth-story__brand-copy strong { font-size: 14px; }
  .auth-entry__mobile-brand .auth-story__brand-copy small { font-size: 10px; }
}
@media (max-width: 520px) {
  .auth-entry { display: flex; justify-content: center; flex-direction: column; padding-inline: 14px; }
  .auth-entry__mobile-brand { margin-bottom: 16px; }
}
</style>
