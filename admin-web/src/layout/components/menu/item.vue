<template>
  <el-sub-menu v-if="visibleChildren.length > 1" :key="route.name" :index="route.name">
    <template #title>
      <el-icon v-if="route.meta?.icon"><component :is="`el-icon-${route.meta.icon}`"/></el-icon>
      <span>{{ T(route.meta?.title) || T(route.name) }}</span>
    </template>
    <menu-item v-for="child in visibleChildren" :key="child.name" :route="child" @navigate="$emit('navigate')"/>
  </el-sub-menu>
  <el-menu-item v-else-if="!singleRoute.meta?.hide" :route="singleRoute" :index="singleRoute.name" @click="$emit('navigate')">
    <el-icon v-if="singleRoute.meta?.icon"><component :is="`el-icon-${singleRoute.meta.icon}`"/></el-icon>
    <template #title>{{ T(singleRoute.meta?.title) || T(singleRoute.name) }}</template>
  </el-menu-item>
</template>

<script setup>
import { computed } from 'vue'
import { T } from '@/utils/i18n'

const props = defineProps({ route: { type: Object, required: true } })
defineEmits(['navigate'])
const visibleChildren = computed(() => (props.route.children || []).filter(child => !child.meta?.hide))
const singleRoute = computed(() => visibleChildren.value.length === 1 ? visibleChildren.value[0] : props.route)
</script>
