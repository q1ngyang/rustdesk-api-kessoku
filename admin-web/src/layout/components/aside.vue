<template>
  <div class="sidebar-inner">
    <button class="sidebar-brand" type="button" :aria-label="T('Overview')" @click="toOverview"><DeploymentBrand :compact="isCollapsed"/></button>
    <div v-if="!isCollapsed" class="sidebar-caption">{{ T('Workspace') }}</div>
    <el-scrollbar class="sidebar-scroll"><menus @navigate="$emit('navigate')"/></el-scrollbar>
    <button class="sidebar-profile" :class="{ 'sidebar-profile--compact': isCollapsed }" type="button" :aria-label="T('Overview')" @click="toOverview">
      <span class="sidebar-profile__avatar"><img v-if="userStore.avatar" :src="userStore.avatar" alt=""/><template v-else>{{ initial }}</template></span>
      <span v-if="!isCollapsed" class="sidebar-profile__copy">
        <strong>{{ userStore.username || T('User') }}</strong>
        <small>{{ roleLabel }}</small>
      </span>
      <span class="sidebar-profile__presence" :title="T('SignedIn')"></span>
    </button>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import Menus from '@/layout/components/menu/index.vue'
import DeploymentBrand from '@/components/brand/DeploymentBrand.vue'
import { useAppStore } from '@/store/app'
import { useUserStore } from '@/store/user'
import { T } from '@/utils/i18n'

defineEmits(['navigate'])
const appStore = useAppStore()
const router = useRouter()
const userStore = useUserStore()
const isCollapsed = computed(() => appStore.setting.sideIsCollapse && appStore.setting.viewportWidth >= 768)
const roleLabel = computed(() => ({ admin: T('ScopedAdministrator'), super_admin: T('SuperAdministrator') }[userStore.role] || T('PersonalWorkspace')))
const initial = computed(() => (userStore.username || 'K').slice(0, 1).toUpperCase())
const toOverview = () => router.push({ name: 'MyInfo' })
</script>

<style scoped lang="scss">
.sidebar-inner { display: flex; box-sizing: border-box; height: 100%; min-width: 0; flex-direction: column; padding: 14px 10px 12px; }
.sidebar-brand { display: flex; min-height: 54px; align-items: center; padding: 0 9px 12px; border: 0; background: transparent; color: inherit; cursor: pointer; text-align: left; }
.sidebar-brand:focus-visible { border-radius: 12px; outline: 3px solid var(--focus-ring); }
.sidebar-caption { padding: 6px 13px 8px; color: var(--text-tertiary); font-size: 10px; font-weight: 750; letter-spacing: .12em; text-transform: uppercase; }
.sidebar-scroll { min-height: 0; flex: 1; }
.sidebar-profile { position: relative; display: flex; width: 100%; min-width: 0; align-items: center; gap: 10px; margin-top: 10px; padding: 10px; border: 1px solid var(--border-subtle); border-radius: 15px; background: var(--surface-2); color: inherit; cursor: pointer; text-align: left; }
.sidebar-profile:hover { border-color: var(--primary-border); background: var(--surface-3); }
.sidebar-profile--compact { justify-content: center; padding-inline: 6px; }
.sidebar-profile__avatar { display: grid; width: 34px; height: 34px; flex: 0 0 auto; place-items: center; border-radius: 11px; background: var(--primary-soft); color: var(--primary); font-size: 13px; font-weight: 800; }
.sidebar-profile__avatar { overflow: hidden; }.sidebar-profile__avatar img { width: 100%; height: 100%; object-fit: cover; }
.sidebar-profile__copy { display: flex; min-width: 0; flex: 1; flex-direction: column; }
.sidebar-profile__copy strong,.sidebar-profile__copy small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.sidebar-profile__copy strong { color: var(--text-primary); font-size: 13px; }.sidebar-profile__copy small { margin-top: 3px; color: var(--text-tertiary); font-size: 10px; }
.sidebar-profile__presence { width: 10px; height: 10px; flex: 0 0 auto; border: 2px solid var(--surface-2); border-radius: 50%; background: var(--success); box-shadow: 0 0 0 2px color-mix(in srgb, var(--success) 15%, transparent); }
@media (max-width: 767px) { .sidebar-inner { padding-top: calc(14px + env(safe-area-inset-top)); } }
</style>
