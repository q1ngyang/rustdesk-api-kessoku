<template>
  <span
    class="deployment-asset"
    :class="`deployment-asset--${variant}`"
    :style="{ width: normalizedWidth }"
    :role="alt ? 'img' : undefined"
    :aria-label="alt || undefined"
    :aria-hidden="alt ? undefined : 'true'"
  >
    <img class="deployment-asset__image" :src="activeSource" alt="" aria-hidden="true"/>
  </span>
</template>

<script setup>
import { computed } from 'vue'
import { useDark } from '@vueuse/core'
import iconDark from '@/assets/brand/starrydesk-icon-dark.svg'
import iconLight from '@/assets/brand/starrydesk-icon-light.svg'
import logoDark from '@/assets/brand/starrydesk-logo-dark.svg'
import logoLight from '@/assets/brand/starrydesk-logo-light.svg'

const props = defineProps({
  variant: { type: String, default: 'icon', validator: value => ['icon', 'logo'].includes(value) },
  width: { type: [Number, String], default: 36 },
  lightUrl: { type: String, default: '' },
  darkUrl: { type: String, default: '' },
  alt: { type: String, default: 'Kessoku' },
})
const isDark = useDark({ storageKey: 'kessoku-theme' })
const normalizedWidth = computed(() => {
  if (typeof props.width === 'number') return `${props.width}px`
  return /^\d+(\.\d+)?$/.test(props.width.trim()) ? `${props.width}px` : props.width
})
const defaults = computed(() => props.variant === 'logo'
  ? { light: logoLight, dark: logoDark }
  : { light: iconLight, dark: iconDark })
const sources = computed(() => ({
  light: props.lightUrl || defaults.value.light,
  dark: props.darkUrl || defaults.value.dark,
}))
const activeSource = computed(() => isDark.value ? sources.value.dark : sources.value.light)
</script>

<style scoped>
.deployment-asset { display: inline-flex; max-width: 100%; flex: 0 0 auto; align-items: center; line-height: 0; }
.deployment-asset__image { display: block; width: 100%; height: auto; object-fit: contain; }
.deployment-asset--icon,.deployment-asset--icon .deployment-asset__image { aspect-ratio: 1; }
.deployment-asset--logo { aspect-ratio: 4; }
</style>
