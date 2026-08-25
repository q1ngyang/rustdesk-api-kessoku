<template>
  <div class="sidebar-inner">
    <div class="sidebar-brand"><KessokuBrand :compact="isCollapsed"/></div>
    <div v-if="!isCollapsed" class="sidebar-caption">{{ T('Workspace') }}</div>
    <el-scrollbar class="sidebar-scroll"><menus @navigate="$emit('navigate')"/></el-scrollbar>
    <div class="sidebar-profile" :class="{ 'sidebar-profile--compact': isCollapsed }">
      <span class="sidebar-profile__avatar">{{ initial }}</span>
      <span v-if="!isCollapsed" class="sidebar-profile__copy">
        <strong>{{ userStore.username || T('User') }}</strong>
        <small>{{ roleLabel }}</small>
      </span>
      <span class="sidebar-profile__presence" :title="T('SignedIn')"></span>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import Menus from '@/layout/components/menu/index.vue'
import KessokuBrand from '@/components/brand/KessokuBrand.vue'
import { useAppStore } from '@/store/app'
import { useUserStore } from '@/store/user'
import { T } from '@/utils/i18n'

defineEmits(['navigate'])
const appStore = useAppStore()
const userStore = useUserStore()
const isCollapsed = computed(() => appStore.setting.sideIsCollapse && appStore.setting.viewportWidth >= 768)
const roleLabel = computed(() => ({ admin: T('ScopedAdministrator'), super_admin: T('SuperAdministrator') }[userStore.role] || T('PersonalWorkspace')))
const initial = computed(() => (userStore.username || 'K').slice(0, 1).toUpperCase())
</script>

<style scoped lang="scss">
.sidebar-inner { display: flex; box-sizing: border-box; height: 100%; min-width: 0; flex-direction: column; padding: 14px 10px 12px; }
.sidebar-brand { display: flex; min-height: 54px; align-items: center; padding: 0 9px 12px; }
.sidebar-caption { padding: 6px 13px 8px; color: var(--text-tertiary); font-size: 10px; font-weight: 750; letter-spacing: .12em; text-transform: uppercase; }
.sidebar-scroll { min-height: 0; flex: 1; }
.sidebar-profile { position: relative; display: flex; min-width: 0; align-items: center; gap: 10px; margin-top: 10px; padding: 10px; border: 1px solid var(--border-subtle); border-radius: 15px; background: var(--surface-2); }
.sidebar-profile--compact { justify-content: center; padding-inline: 6px; }
.sidebar-profile__avatar { display: grid; width: 34px; height: 34px; flex: 0 0 auto; place-items: center; border-radius: 11px; background: var(--primary-soft); color: var(--primary); font-size: 13px; font-weight: 800; }
.sidebar-profile__copy { display: flex; min-width: 0; flex: 1; flex-direction: column; }
.sidebar-profile__copy strong,.sidebar-profile__copy small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.sidebar-profile__copy strong { color: var(--text-primary); font-size: 13px; }.sidebar-profile__copy small { margin-top: 3px; color: var(--text-tertiary); font-size: 10px; }
.sidebar-profile__presence { width: 7px; height: 7px; flex: 0 0 auto; border: 2px solid var(--surface-2); border-radius: 50%; background: var(--success); }
@media (max-width: 767px) { .sidebar-inner { padding-top: calc(14px + env(safe-area-inset-top)); } }
</style>
