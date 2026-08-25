<template>
  <nav class="mobile-nav" aria-label="Mobile navigation">
    <button v-for="item in items" :key="item.name" class="mobile-nav__item" :class="{ 'is-active': route.name === item.name }" type="button" @click="go(item)">
      <el-icon><component :is="item.icon"/></el-icon><span>{{ T(item.label) }}</span>
    </button>
    <button class="mobile-nav__item" type="button" @click="appStore.openMobileSidebar()"><el-icon><Grid/></el-icon><span>{{ T('More') }}</span></button>
  </nav>
</template>

<script setup>
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Collection, Grid, Monitor, Operation, User, UserFilled } from '@element-plus/icons-vue'
import { useRouteStore } from '@/store/router'
import { useAppStore } from '@/store/app'
import { useUserStore } from '@/store/user'
import { T } from '@/utils/i18n'

const route = useRoute()
const router = useRouter()
const routeStore = useRouteStore()
const appStore = useAppStore()
const userStore = useUserStore()
const personalItems = [
  { name: 'MyInfo', label: 'Overview', icon: User },
  { name: 'MyPeer', label: 'Devices', icon: Monitor },
  { name: 'MyAddressBookList', label: 'AddressBooks', icon: Collection },
]
const adminItems = [
  { name: 'MyInfo', label: 'Overview', icon: User },
  { name: 'Peer', label: 'Devices', icon: Monitor },
  { name: 'UserList', label: 'UserManage', icon: UserFilled },
  { name: 'ServerControl', label: 'ServerControl', icon: Operation },
]
const preferred = computed(() => ['admin', 'super_admin'].includes(userStore.role) ? adminItems : personalItems)
const availableNames = computed(() => {
  const names = []
  const walk = routes => routes.forEach(item => { if (item.name) names.push(item.name); if (item.children) walk(item.children) })
  walk(routeStore.routes)
  return names
})
const items = computed(() => preferred.value.filter(item => availableNames.value.includes(item.name)).slice(0, 3))
const go = item => router.push({ name: item.name })
</script>

<style scoped lang="scss">
.mobile-nav { position: fixed; right: 10px; bottom: calc(10px + env(safe-area-inset-bottom)); left: 10px; z-index: 26; display: flex; min-height: 61px; align-items: center; justify-content: space-around; padding: 4px 6px; border: 1px solid color-mix(in srgb, var(--border-strong) 70%, transparent); border-radius: 20px; background: color-mix(in srgb, var(--surface-1) 91%, transparent); box-shadow: var(--shadow-lg); backdrop-filter: blur(20px); }
.mobile-nav__item { display: flex; min-width: 58px; min-height: 48px; align-items: center; justify-content: center; gap: 4px; border: 0; border-radius: 14px; background: transparent; color: var(--text-tertiary); cursor: pointer; flex-direction: column; }
.mobile-nav__item .el-icon { font-size: 19px; }.mobile-nav__item span { font-size: 10px; font-weight: 700; }.mobile-nav__item.is-active { background: var(--primary-soft); color: var(--primary); }
</style>
