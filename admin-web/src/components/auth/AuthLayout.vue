<template>
  <main class="auth-shell">
    <div class="auth-shell__glow auth-shell__glow--one"></div>
    <div class="auth-shell__glow auth-shell__glow--two"></div>
    <section class="auth-story">
      <div class="auth-story__brand">
        <template v-if="showStarryLogo"><StarryLogo width="260"/><span>RustDesk API KESSOKU</span></template>
        <KessokuBrand v-else/>
      </div>
      <div class="auth-story__content">
        <span class="auth-story__eyebrow">{{ T('SecureManagementConsole') }}</span>
        <h1>{{ T('WelcomeToKessoku') }}</h1>
        <p>{{ T('AuthIntroduction') }}</p>
        <div class="auth-story__features">
          <div><el-icon><Monitor/></el-icon><span>{{ T('UnifiedDeviceManagement') }}</span></div>
          <div><el-icon><Connection/></el-icon><span>{{ T('ServerHealthAtAGlance') }}</span></div>
          <div><el-icon><Lock/></el-icon><span>{{ T('ExistingPermissionModel') }}</span></div>
        </div>
      </div>
      <div class="auth-story__footer">
        <span>RustDesk API Kessoku · v3</span>
      </div>
    </section>
    <section class="auth-entry">
      <div class="auth-entry__mobile-brand">
        <template v-if="showStarryLogo"><StarryLogo width="220"/><span>RustDesk API KESSOKU</span></template>
        <KessokuBrand v-else/>
      </div>
      <slot/>
    </section>
  </main>
</template>

<script setup>
import { Connection, Lock, Monitor } from '@element-plus/icons-vue'
import KessokuBrand from '@/components/brand/KessokuBrand.vue'
import StarryLogo from '@/components/brand/StarryLogo.vue'
import { T } from '@/utils/i18n'
defineProps({ showStarryLogo: Boolean })
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
.auth-story__brand { display: flex; min-height: 50px; align-items: flex-start; flex-direction: column; gap: 8px; }
.auth-story__brand > span { color: var(--text-tertiary); font-size: 10px; font-weight: 800; letter-spacing: .13em; }
.auth-story__content { width: min(560px, 100%); margin: auto 0; padding: 56px 0; }
.auth-story__eyebrow { display: inline-flex; padding: 7px 11px; border: 1px solid var(--primary-border); border-radius: 999px; background: var(--primary-soft); color: var(--primary); font-size: 11px; font-weight: 760; letter-spacing: .04em; }
.auth-story h1 { max-width: 560px; margin: 22px 0 16px; color: var(--text-primary); font-size: clamp(40px, 5vw, 68px); line-height: 1.02; letter-spacing: -.055em; }
.auth-story p { max-width: 530px; margin: 0; color: var(--text-secondary); font-size: 15px; line-height: 1.8; }
.auth-story__features { display: grid; gap: 10px; margin-top: 34px; }
.auth-story__features div { display: flex; align-items: center; gap: 11px; color: var(--text-secondary); font-size: 13px; font-weight: 620; }
.auth-story__features .el-icon { display: grid; width: 32px; height: 32px; place-items: center; border-radius: 10px; background: var(--surface-1); color: var(--primary); box-shadow: var(--shadow-sm); }
.auth-story__features div:nth-child(2) .el-icon { background: color-mix(in srgb, var(--kessoku-yellow) 17%, var(--surface-1)); color: #a97d0c; }
.auth-story__features div:nth-child(3) .el-icon { background: color-mix(in srgb, var(--kessoku-pink) 15%, var(--surface-1)); color: #bd5d84; }
.auth-story__footer { display: flex; align-items: center; gap: 12px; color: var(--text-tertiary); font-size: 10px; font-weight: 680; letter-spacing: .06em; }
.auth-entry { position: relative; z-index: 1; display: grid; min-width: 0; place-items: center; padding: 28px; }
.auth-entry__mobile-brand { display: none; }
@media (max-width: 900px) {
  .auth-shell { display: block; }
  .auth-story { display: none; }
  .auth-entry { min-height: 100dvh; padding: max(22px, env(safe-area-inset-top)) 18px max(22px, env(safe-area-inset-bottom)); }
  .auth-entry__mobile-brand { display: flex; width: min(430px, 100%); margin: 0 auto 24px; align-items: flex-start; flex-direction: column; gap: 7px; }
  .auth-entry__mobile-brand > span { color: var(--text-tertiary); font-size: 9px; font-weight: 800; letter-spacing: .12em; }
}
@media (max-width: 520px) {
  .auth-entry { display: flex; justify-content: center; flex-direction: column; padding-inline: 14px; }
  .auth-entry__mobile-brand { margin-bottom: 16px; }
}
</style>
