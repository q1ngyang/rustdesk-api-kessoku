<template>
  <div class="about-page">
    <section class="about-hero">
      <div class="about-hero__logo"><StarryLogo width="min(360px, 76vw)"/></div>
      <span class="about-hero__eyebrow">RustDesk API KESSOKU</span>
      <h2>{{ T('About') }}</h2>
      <p>{{ T('AboutDescription') }}</p>
    </section>

    <section class="about-grid">
      <article class="about-card about-card--identity">
        <StarryIcon :size="48"/>
        <div><small>{{ T('ProjectIdentity') }}</small><strong>RustDesk API KESSOKU</strong><p>{{ T('ProjectIdentityDescription') }}</p></div>
      </article>
      <article class="about-card">
        <span class="about-card__icon about-card__icon--yellow"><el-icon><Monitor/></el-icon></span>
        <div><small>{{ T('ResponsiveInterface') }}</small><strong>{{ T('DesktopTabletMobile') }}</strong><p>{{ T('ResponsiveInterfaceDescription') }}</p></div>
      </article>
      <article class="about-card">
        <span class="about-card__icon about-card__icon--pink"><el-icon><Moon/></el-icon></span>
        <div><small>{{ T('ThemeAdaptation') }}</small><strong>{{ T('LightAndDark') }}</strong><p>{{ T('ThemeAdaptationDescription') }}</p></div>
      </article>
    </section>

    <footer class="about-footer"><span>Version</span><strong>3.0.1</strong></footer>
  </div>
</template>

<script setup>
import { Monitor, Moon } from '@element-plus/icons-vue'
import StarryIcon from '@/components/brand/StarryIcon.vue'
import StarryLogo from '@/components/brand/StarryLogo.vue'
import { T } from '@/utils/i18n'
</script>

<style scoped lang="scss">
.about-page { width: min(1040px, 100%); margin: 0 auto; }
.about-hero { position: relative; overflow: hidden; padding: clamp(30px, 6vw, 70px); border: 1px solid var(--border-subtle); border-radius: 28px; background: var(--surface-1); box-shadow: var(--shadow-md); text-align: center; }
.about-hero::before { position: absolute; top: -110px; left: 50%; width: 470px; height: 280px; border-radius: 50%; background: radial-gradient(circle, color-mix(in srgb, var(--primary) 15%, transparent), transparent 68%); content: ''; transform: translateX(-50%); }
.about-hero__logo { position: relative; display: flex; justify-content: center; margin: 0 auto 30px; }
.about-hero__eyebrow { position: relative; color: var(--primary); font-size: 11px; font-weight: 800; letter-spacing: .14em; }
.about-hero h2 { position: relative; margin: 12px 0 9px; color: var(--text-primary); font-size: clamp(30px, 5vw, 50px); letter-spacing: -.045em; }
.about-hero p { position: relative; max-width: 650px; margin: 0 auto; color: var(--text-secondary); font-size: 14px; line-height: 1.8; }
.about-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 14px; margin-top: 14px; }
.about-card { display: flex; min-width: 0; align-items: flex-start; gap: 15px; padding: 22px; border: 1px solid var(--border-subtle); border-radius: 20px; background: var(--surface-1); box-shadow: var(--shadow-sm); }
.about-card > div { min-width: 0; }.about-card small { color: var(--text-tertiary); font-size: 10px; font-weight: 750; letter-spacing: .05em; text-transform: uppercase; }.about-card strong { display: block; margin-top: 5px; color: var(--text-primary); font-size: 14px; }.about-card p { margin: 7px 0 0; color: var(--text-secondary); font-size: 11px; line-height: 1.65; }
.about-card__icon { display: grid; width: 48px; height: 48px; flex: 0 0 auto; place-items: center; border-radius: 15px; color: var(--primary); font-size: 20px; }.about-card__icon--yellow { background: color-mix(in srgb, var(--kessoku-yellow) 18%, var(--surface-1)); color: #a77a05; }.about-card__icon--pink { background: color-mix(in srgb, var(--kessoku-pink) 17%, var(--surface-1)); color: #bb5b82; }
.about-footer { display: flex; justify-content: center; gap: 8px; padding: 22px; color: var(--text-tertiary); font-size: 11px; }.about-footer strong { color: var(--text-secondary); }
@media (max-width: 900px) { .about-grid { grid-template-columns: 1fr; }.about-card { align-items: center; } }
@media (max-width: 520px) { .about-hero { padding: 30px 18px; border-radius: 22px; }.about-hero__logo { margin-bottom: 22px; }.about-card { padding: 18px; border-radius: 17px; } }
</style>
