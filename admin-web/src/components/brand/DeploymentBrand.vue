<template>
  <div class="deployment-brand" :class="{ 'deployment-brand--compact': compact }">
    <DeploymentAsset
      variant="icon"
      :width="compact ? 36 : 38"
      :light-url="branding.brand_icon_light_url"
      :dark-url="branding.brand_icon_dark_url"
      alt=""
    />
    <span v-if="!compact" class="deployment-brand__copy"><strong>{{ title }}</strong><small>{{ subtitle }}</small></span>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import DeploymentAsset from '@/components/brand/DeploymentAsset.vue'
import { useAppStore } from '@/store/app'

const props = defineProps({ compact: Boolean })
const appStore = useAppStore()
const branding = computed(() => appStore.setting.branding)
const title = computed(() => appStore.setting.branding.admin_title || 'RustDesk API')
const subtitle = computed(() => appStore.setting.branding.admin_subtitle || 'KESSOKU')
</script>

<style scoped>
.deployment-brand { display: flex; min-width: 0; align-items: center; gap: 10px; }
.deployment-brand__copy { display: flex; min-width: 0; flex-direction: column; line-height: 1.15; }
.deployment-brand strong,.deployment-brand small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.deployment-brand strong { color: var(--text-primary); font-size: 14px; font-weight: 760; letter-spacing: -.02em; }
.deployment-brand small { margin-top: 4px; color: var(--text-tertiary); font-size: 11px; font-weight: 650; letter-spacing: .12em; text-transform: uppercase; }
</style>
