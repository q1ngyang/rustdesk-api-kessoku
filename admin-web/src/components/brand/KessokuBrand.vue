<template>
  <div class="kessoku-brand" :class="{ 'kessoku-brand--compact': compact }">
    <DeploymentAsset
      variant="icon"
      :width="compact ? 34 : 38"
      :light-url="branding.brand_icon_light_url"
      :dark-url="branding.brand_icon_dark_url"
      alt=""
    />
    <div v-if="!compact" class="kessoku-brand__copy">
      <strong>RustDesk API</strong>
      <span>KESSOKU</span>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import DeploymentAsset from '@/components/brand/DeploymentAsset.vue'
import { useAppStore } from '@/store/app'
defineProps({ compact: Boolean })
const appStore = useAppStore()
const branding = computed(() => appStore.setting.branding)
</script>

<style scoped lang="scss">
.kessoku-brand { display: flex; min-width: 0; align-items: center; gap: 12px; }
.kessoku-brand__copy { display: flex; min-width: 0; flex-direction: column; line-height: 1.15; }
.kessoku-brand__copy strong { overflow: hidden; color: var(--text-primary); font-size: 14px; font-weight: 760; letter-spacing: -.02em; text-overflow: ellipsis; white-space: nowrap; }
.kessoku-brand__copy span { margin-top: 4px; color: var(--text-tertiary); font-size: 11px; font-weight: 650; letter-spacing: .12em; text-transform: uppercase; }
</style>
