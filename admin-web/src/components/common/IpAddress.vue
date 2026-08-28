<template>
  <el-popover trigger="hover" placement="top" :width="290" :show-after="180" @before-enter="load">
    <template #reference><span class="ip-address" :class="{ 'ip-address--empty': !value }">{{ value || '-' }}<el-icon v-if="value"><LocationInformation/></el-icon></span></template>
    <div class="ip-card">
      <div class="ip-card__heading"><strong>{{ value }}</strong><el-tag v-if="result.private" size="small" type="info">{{ T('PrivateNetwork') }}</el-tag></div>
      <div v-if="loading" class="ip-card__loading"><el-icon class="is-loading"><Loading/></el-icon>{{ T('LookingUpIP') }}</div>
      <template v-else-if="result.available">
        <dl><dt>{{ T('Country') }}</dt><dd>{{ result.country || result.country_iso || '-' }}</dd><dt>{{ T('City') }}</dt><dd>{{ result.city || '-' }}</dd><dt>ASN</dt><dd>{{ result.asn ? `AS${result.asn}` : '-' }}</dd><dt>{{ T('NetworkProvider') }}</dt><dd>{{ result.asn_org || '-' }}</dd></dl>
      </template>
      <el-alert v-else :title="T('IPDatabaseUnavailable')" type="info" :closable="false"/>
    </div>
  </el-popover>
</template>

<script setup>
import { reactive, ref, watch } from 'vue'
import { Loading, LocationInformation } from '@element-plus/icons-vue'
import { lookupIP } from '@/api/config'
import { T } from '@/utils/i18n'
import { useAppStore } from '@/store/app'

const props = defineProps({ value: { type: String, default: '' } })
const appStore = useAppStore()
const cache = globalThis.__kessokuIPCache || (globalThis.__kessokuIPCache = new Map())
const loading = ref(false)
const loaded = ref(false)
const result = reactive({ available: false, private: false, country: '', country_iso: '', city: '', asn: 0, asn_org: '' })
const load = async () => {
  if (!props.value || loaded.value) return
  const cacheKey = `${appStore.setting.lang}|${props.value}`
  const cached = cache.get(cacheKey)
  if (cached) { Object.assign(result, cached); loaded.value = true; return }
  loading.value = true
  try {
    const data = (await lookupIP(props.value)).data || {}
    Object.assign(result, data); cache.set(cacheKey, data); loaded.value = true
  } catch { loaded.value = true } finally { loading.value = false }
}
watch(() => [props.value, appStore.setting.lang], () => {
  loaded.value = false
  Object.assign(result, { available: false, private: false, country: '', country_iso: '', city: '', asn: 0, asn_org: '' })
})
</script>

<style scoped lang="scss">
.ip-address { display: inline-flex; align-items: center; gap: 4px; color: var(--primary); cursor: help; font-variant-numeric: tabular-nums; }.ip-address .el-icon { font-size: 12px; opacity: .7; }.ip-address--empty { color: var(--text-tertiary); cursor: default; }
.ip-card__heading { display: flex; align-items: center; justify-content: space-between; gap: 8px; }.ip-card__heading strong { overflow-wrap: anywhere; color: var(--text-primary); font-family: ui-monospace, monospace; font-size: 12px; }
.ip-card__loading { display: flex; align-items: center; gap: 7px; padding: 18px 0 6px; color: var(--text-tertiary); font-size: 12px; }
dl { display: grid; gap: 8px 12px; margin: 14px 0 2px; grid-template-columns: auto minmax(0,1fr); font-size: 11px; }dt { color: var(--text-tertiary); }dd { margin: 0; overflow-wrap: anywhere; color: var(--text-secondary); text-align: right; }
</style>
