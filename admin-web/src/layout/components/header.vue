<template>
  <div class="topbar">
    <button class="topbar__menu" type="button" :aria-label="T('ToggleNavigation')" @click="toggleNavigation"><el-icon><Menu/></el-icon></button>
    <div class="topbar__heading"><h1>{{ pageTitle }}</h1><span v-if="instanceTitle">{{ instanceTitle }}</span></div>
    <Setting/>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { Menu } from '@element-plus/icons-vue'
import Setting from '@/layout/components/setting/index.vue'
import { useAppStore } from '@/store/app'
import { T } from '@/utils/i18n'

const appStore = useAppStore()
const route = useRoute()
const pageTitle = computed(() => T(route.meta?.title || route.name || 'Overview'))
const instanceTitle = computed(() => appStore.setting.title !== appStore.setting.productName ? appStore.setting.title : '')
const toggleNavigation = () => {
  if (appStore.setting.viewportWidth < 768) appStore.openMobileSidebar()
  else appStore.sideCollapse()
}
</script>

<style scoped lang="scss">
.topbar { display: flex; box-sizing: border-box; height: 100%; align-items: center; gap: 12px; padding: 0 clamp(14px, 2vw, 28px); }
.topbar__menu { display: grid; width: 38px; height: 38px; flex: 0 0 auto; place-items: center; border: 0; border-radius: 12px; background: transparent; color: var(--text-secondary); cursor: pointer; transition: color var(--motion-fast), background var(--motion-fast); }
.topbar__menu:hover { background: var(--surface-3); color: var(--primary); }.topbar__menu:focus-visible { outline: 3px solid var(--focus-ring); outline-offset: 1px; }
.topbar__heading { display: flex; min-width: 0; flex-direction: column; }.topbar__heading h1 { margin: 0; color: var(--text-primary); font-size: 17px; font-weight: 760; letter-spacing: -.02em; }.topbar__heading span { margin-top: 2px; overflow: hidden; color: var(--text-tertiary); font-size: 10px; text-overflow: ellipsis; white-space: nowrap; }
@media (max-width: 520px) { .topbar { gap: 8px; padding-inline: 10px; }.topbar__heading span { display: none; }.topbar__heading h1 { max-width: 42vw; overflow: hidden; font-size: 15px; text-overflow: ellipsis; white-space: nowrap; } }
</style>
