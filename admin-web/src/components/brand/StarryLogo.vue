<template><ThemeBrandAsset variant="logo" :width="width" :alt="alt"/></template>

<script setup>
import ThemeBrandAsset from '@/components/brand/ThemeBrandAsset.vue'
defineProps({
  width: { type: [Number, String], default: 240 },
  alt: { type: String, default: 'StarryLinks' },
})
</script>
