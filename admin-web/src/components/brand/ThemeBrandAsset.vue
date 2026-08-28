<template>
  <span
    class="theme-brand-asset"
    :class="`theme-brand-asset--${variant}`"
    :style="{ width: normalizedWidth }"
    :role="alt ? 'img' : undefined"
    :aria-label="alt || undefined"
    :aria-hidden="alt ? undefined : 'true'"
  >
    <img class="theme-brand-asset__image" :src="activeSource" alt="" aria-hidden="true"/>
  </span>
</template>

<script setup>
import { computed } from 'vue'
import { useDark } from '@vueuse/core'
import iconDark from '@/assets/brand/starrylinks-icon-dark.svg'
import iconLight from '@/assets/brand/starrylinks-icon-light.svg'
import logoDark from '@/assets/brand/starrylinks-logo-dark.svg'
import logoLight from '@/assets/brand/starrylinks-logo-light.svg'

const props = defineProps({
  variant: { type: String, default: 'icon', validator: value => ['icon', 'logo'].includes(value) },
  width: { type: [Number, String], default: 36 },
  alt: { type: String, default: 'StarryLinks' },
})
const isDark = useDark({ storageKey: 'kessoku-theme' })
const normalizedWidth = computed(() => {
  if (typeof props.width === 'number') return `${props.width}px`
  return /^\d+(\.\d+)?$/.test(props.width.trim()) ? `${props.width}px` : props.width
})
const sources = computed(() => props.variant === 'logo'
  ? { light: logoLight, dark: logoDark }
  : { light: iconLight, dark: iconDark })
const activeSource = computed(() => isDark.value ? sources.value.dark : sources.value.light)
</script>

<style scoped>
.theme-brand-asset { display: inline-flex; max-width: 100%; flex: 0 0 auto; align-items: center; line-height: 0; }
.theme-brand-asset__image { display: block; width: 100%; height: auto; }
.theme-brand-asset--icon { aspect-ratio: 1; }
.theme-brand-asset--icon .theme-brand-asset__image { aspect-ratio: 1; }
.theme-brand-asset--logo { aspect-ratio: 4; }
</style>
