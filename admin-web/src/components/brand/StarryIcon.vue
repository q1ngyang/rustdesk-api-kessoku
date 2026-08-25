<template><ThemeBrandAsset variant="icon" :width="size" :alt="alt"/></template>

<script setup>
import ThemeBrandAsset from '@/components/brand/ThemeBrandAsset.vue'
defineProps({
  size: { type: [Number, String], default: 36 },
  alt: { type: String, default: 'KESSOKU' },
})
</script>
