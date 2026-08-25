<template>
  <el-config-provider :locale="appStore.setting.locale.value">
    <div class="app-shell" :style="{ '--sidebar-width': appStore.setting.locale.sideBarWidth }" :class="{
      'app-shell--collapsed': appStore.setting.sideIsCollapse,
      'app-shell--drawer-open': appStore.setting.mobileSidebarOpen,
    }">
      <aside class="app-sidebar" aria-label="Primary navigation">
        <g-aside @navigate="closeDrawer"/>
      </aside>
      <button v-if="isMobile && appStore.setting.mobileSidebarOpen" class="app-sidebar-mask" type="button" aria-label="Close navigation" @click="closeDrawer"></button>

      <section class="app-frame">
        <header class="app-header"><g-header/></header>
        <div class="header-tags"><tags/></div>
        <main class="app-main">
          <router-view v-slot="{ Component }">
            <transition mode="out-in" name="page-fade">
              <keep-alive :include="cachedTags"><component :is="Component"/></keep-alive>
            </transition>
          </router-view>
        </main>
        <mobile-bottom-nav v-if="isMobile"/>
      </section>
    </div>
  </el-config-provider>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useAppStore } from '@/store/app'
import { useTagsStore } from '@/store/tags'
import Tags from '@/layout/components/tags/index.vue'
import GAside from '@/layout/components/aside.vue'
import GHeader from '@/layout/components/header.vue'
import MobileBottomNav from '@/layout/components/MobileBottomNav.vue'

const appStore = useAppStore()
const tagStore = useTagsStore()
const route = useRoute()
const isMobile = computed(() => appStore.setting.viewportWidth < 768)
const cachedTags = computed(() => tagStore.cached)
let previousViewportWidth = null

const updateViewport = () => {
  const width = window.innerWidth
  const enteredTablet = width >= 768 && width <= 1100 && (previousViewportWidth === null || previousViewportWidth > 1100 || previousViewportWidth < 768)
  appStore.setViewportWidth(width)
  if (enteredTablet) appStore.setting.sideIsCollapse = true
  if (!isMobile.value) appStore.closeMobileSidebar()
  previousViewportWidth = width
}
const closeDrawer = () => appStore.closeMobileSidebar()

onMounted(() => {
  updateViewport()
  window.addEventListener('resize', updateViewport, { passive: true })
})
onBeforeUnmount(() => window.removeEventListener('resize', updateViewport))
watch(() => route.fullPath, closeDrawer)
</script>

<style scoped lang="scss">
.app-shell { display: grid; min-height: 100dvh; grid-template-columns: var(--sidebar-width) minmax(0, 1fr); background: var(--app-bg); transition: grid-template-columns var(--motion-base); }
.app-shell--collapsed { grid-template-columns: var(--sidebar-collapsed-width) minmax(0, 1fr); }
.app-sidebar { position: sticky; top: 0; z-index: 30; height: 100dvh; overflow: hidden; border-right: 1px solid var(--border-subtle); background: var(--surface-1); }
.app-frame { display: flex; min-width: 0; min-height: 100dvh; flex-direction: column; }
.app-header { position: sticky; top: 0; z-index: 24; height: var(--header-height); border-bottom: 1px solid var(--border-subtle); background: color-mix(in srgb, var(--surface-1) 88%, transparent); backdrop-filter: blur(18px); }
.header-tags { position: sticky; top: var(--header-height); z-index: 20; min-height: 38px; overflow: hidden; border-bottom: 1px solid var(--border-subtle); background: color-mix(in srgb, var(--surface-1) 91%, transparent); backdrop-filter: blur(16px); }
.app-main { box-sizing: border-box; width: 100%; max-width: 1720px; flex: 1; align-self: center; padding: 24px clamp(18px, 2.2vw, 34px) 38px; }
.app-sidebar-mask { position: fixed; inset: 0; z-index: 28; border: 0; background: rgba(28, 32, 48, .35); backdrop-filter: blur(2px); }
.page-fade-enter-active,.page-fade-leave-active { transition: opacity .16s ease, transform .16s ease; }
.page-fade-enter-from { opacity: 0; transform: translateY(4px); }.page-fade-leave-to { opacity: 0; transform: translateY(-2px); }
@media (max-width: 1100px) and (min-width: 768px) {
  .app-main { padding-inline: 20px; }
}
@media (max-width: 767px) {
  .app-shell,.app-shell--collapsed { display: block; }
  .app-sidebar { position: fixed; inset: 0 auto 0 0; z-index: 32; width: min(84vw, 316px); transform: translateX(-104%); box-shadow: var(--shadow-xl); transition: transform var(--motion-base); }
  .app-shell--drawer-open .app-sidebar { transform: translateX(0); }
  .header-tags { display: none; }
  .app-main { padding: 16px 12px calc(88px + env(safe-area-inset-bottom)); }
}
@media (prefers-reduced-motion: reduce) { .app-shell,.app-sidebar,.page-fade-enter-active,.page-fade-leave-active { transition: none; } }
</style>
