<template>
  <el-menu class="menus" :collapse="isCollapse" :default-active="activeIndex" :collapse-transition="false" router @select="$emit('navigate')">
    <menu-item v-for="routeItem in routes" :key="routeItem.name" :route="routeItem" @navigate="$emit('navigate')"/>
  </el-menu>
</template>

<script setup>
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useRouteStore } from '@/store/router'
import { useAppStore } from '@/store/app'
import MenuItem from '@/layout/components/menu/item.vue'

defineEmits(['navigate'])
const route = useRoute()
const app = useAppStore()
const routeStore = useRouteStore()
const routes = computed(() => routeStore.routes)
const activeIndex = computed(() => route.name)
const isCollapse = computed(() => app.setting.sideIsCollapse && app.setting.viewportWidth >= 768)
</script>

<style scoped lang="scss">
.menus { border-right: 0; background: transparent; }
.menus:not(.el-menu--collapse) { width: 100%; }
</style>
