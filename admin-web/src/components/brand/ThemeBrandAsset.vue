<template>
  <span
    class="theme-brand-asset"
    :class="`theme-brand-asset--${variant}`"
    :style="{ width: normalizedWidth }"
    :role="alt ? 'img' : undefined"
    :aria-label="alt || undefined"
    :aria-hidden="alt ? undefined : 'true'"
  >
    <img class="theme-brand-asset__image theme-brand-asset__image--light" :src="sources.light" alt="" aria-hidden="true"/>
    <img class="theme-brand-asset__image theme-brand-asset__image--dark" :src="sources.dark" alt="" aria-hidden="true"/>
  </span>
</template>

<script setup>
import { computed } from 'vue'
import iconDark from '@/assets/brand/starrylinks-icon-dark.svg'
import iconLight from '@/assets/brand/starrylinks-icon-light.svg'
import logoDark from '@/assets/brand/starrylinks-logo-dark.svg'
import logoLight from '@/assets/brand/starrylinks-logo-light.svg'

const props = defineProps({
  variant: { type: String, default: 'icon', validator: value => ['icon', 'logo'].includes(value) },
  width: { type: [Number, String], default: 36 },
  alt: { type: String, default: 'StarryLinks' },
})
const normalizedWidth = computed(() => typeof props.width === 'number' ? `${props.width}px` : props.width)
const sources = computed(() => props.variant === 'logo'
  ? { light: logoLight, dark: logoDark }
  : { light: iconLight, dark: iconDark })
</script>

<style scoped>
.theme-brand-asset { display: inline-flex; max-width: 100%; flex: 0 0 auto; align-items: center; line-height: 0; }
.theme-brand-asset__image { display: block; width: 100%; height: auto; }
.theme-brand-asset__image--dark { display: none; }
:global(html.dark) .theme-brand-asset__image--light { display: none; }
:global(html.dark) .theme-brand-asset__image--dark { display: block; }
.theme-brand-asset--icon { aspect-ratio: 1; }
.theme-brand-asset--icon .theme-brand-asset__image { aspect-ratio: 1; }
.theme-brand-asset--logo { aspect-ratio: 4; }
</style>
