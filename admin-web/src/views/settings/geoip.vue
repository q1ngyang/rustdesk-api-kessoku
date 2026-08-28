<template>
  <div class="settings-page page-stack">
    <section class="page-hero">
      <div><h2>{{ T('IPGeolocation') }}</h2><p>{{ T('GeoIPSettingsDescription') }}</p></div>
    </section>
    <div class="settings-actions"><el-button type="primary" :loading="saving" @click="save">{{ T('Save') }}</el-button></div>

    <el-card shadow="never">
      <template #header><div class="setting-heading"><span class="setting-heading__icon"><el-icon><Location/></el-icon></span><div><strong>{{ T('IPGeolocation') }}</strong><small>{{ T('IPGeolocationDescription') }}</small></div><el-switch v-model="form.geoip_enabled"/></div></template>
      <el-alert class="country-note" :title="T('CountryDatabaseHelp')" type="info" :closable="false" show-icon/>
      <el-form label-position="top" :disabled="!form.geoip_enabled">
        <el-form-item :label="T('CityDatabaseURL')"><el-input v-model.trim="form.geoip_city_url" placeholder="https://…/GeoLite2-City.mmdb"/></el-form-item>
        <el-form-item :label="T('CountryDatabaseURL')"><el-input v-model.trim="form.geoip_country_url" placeholder="https://…/GeoLite2-Country.mmdb"/></el-form-item>
        <el-form-item :label="T('ASNDatabaseURL')"><el-input v-model.trim="form.geoip_asn_url" placeholder="https://…/GeoLite2-ASN.mmdb"/></el-form-item>
        <el-form-item :label="T('AutomaticUpdateInterval')">
          <el-input-number v-model="form.geoip_update_hours" :min="1" :max="2160"/>
          <span class="unit">{{ T('HourUnit') }}</span>
        </el-form-item>
      </el-form>
      <div class="database-status">
        <StatusValue :value="form.geoip_last_error ? 'degraded' : form.geoip_last_updated_at ? 'healthy' : 'pending'"/>
        <span>{{ form.geoip_last_updated_at ? T('LastUpdatedAt', { time: formatTime(form.geoip_last_updated_at) }) : T('DatabaseNotDownloaded') }}</span>
        <small v-if="form.geoip_last_error">{{ form.geoip_last_error }}</small>
        <el-button size="small" :loading="form.geoip_updating" :disabled="!form.geoip_enabled" @click="updateDatabase">{{ T('UpdateNow') }}</el-button>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { Location } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import StatusValue from '@/components/common/StatusValue.vue'
import { updateGeoIPDatabase } from '@/api/config'
import { T } from '@/utils/i18n'
import { loadSystemSettingForm, saveSystemSettingForm, systemSettingDefaults } from './shared'

const form = reactive({ ...systemSettingDefaults })
const saving = ref(false)
const load = () => loadSystemSettingForm(form)
let updatePollTimer = 0
let updatePollDeadline = 0
let updatePollNotifies = false

const stopUpdatePolling = () => {
  if (updatePollTimer) window.clearTimeout(updatePollTimer)
  updatePollTimer = 0
}

const pollUpdateStatus = async () => {
  updatePollTimer = 0
  try {
    await load()
  } catch {
    form.geoip_updating = false
    stopUpdatePolling()
    return
  }
  if (form.geoip_updating) {
    if (Date.now() >= updatePollDeadline) {
      if (updatePollNotifies) ElMessage.warning(T('GeoIPUpdateStatusTimeout'))
      stopUpdatePolling()
      return
    }
    updatePollTimer = window.setTimeout(pollUpdateStatus, 1200)
    return
  }
  if (updatePollNotifies) {
    if (form.geoip_last_error) ElMessage.error(T('GeoIPUpdateFailed', { message: form.geoip_last_error }))
    else ElMessage.success(T('GeoIPUpdateSucceeded'))
  }
  stopUpdatePolling()
}

const monitorUpdate = (notify = true) => {
  stopUpdatePolling()
  updatePollNotifies = notify
  updatePollDeadline = Date.now() + 10 * 60 * 1000
  updatePollTimer = window.setTimeout(pollUpdateStatus, 800)
}

const save = async () => {
  saving.value = true
  try {
    await saveSystemSettingForm(form)
    await load()
    ElMessage.success(T('Saved'))
  } finally { saving.value = false }
}
const formatTime = value => new Date(value * 1000).toLocaleString()
const updateDatabase = async () => {
  form.geoip_updating = true
  try {
    const result = await updateGeoIPDatabase()
    if (result.data?.updating) {
      ElMessage.info(T(result.data.started ? 'GeoIPUpdateStarted' : 'GeoIPUpdateInProgress'))
      monitorUpdate(true)
    } else {
      form.geoip_updating = false
      ElMessage.warning(T('GeoIPUpdateNotStarted'))
    }
  } catch { form.geoip_updating = false }
}
onMounted(async () => {
  await load()
  if (form.geoip_updating) monitorUpdate(false)
})
onBeforeUnmount(stopUpdatePolling)
</script>

<style scoped lang="scss">
.settings-page { max-width: 980px; margin: 0 auto; }
.settings-actions { display: flex; justify-content: flex-end; margin: -8px 4px 4px; }
.setting-heading { display: flex; min-width: 0; align-items: center; gap: 12px; }
.setting-heading > div { display: flex; min-width: 0; flex: 1; flex-direction: column; }
.setting-heading strong { color: var(--text-primary); font-size: 14px; }
.setting-heading small { margin-top: 3px; color: var(--text-tertiary); font-size: 11px; }
.setting-heading__icon { display: grid; width: 40px; height: 40px; flex: 0 0 auto; place-items: center; border-radius: 13px; background: var(--primary-soft); color: var(--primary); font-size: 17px; }
.country-note { margin-bottom: 20px; }
.unit { margin-left: 9px; color: var(--text-tertiary); font-size: 12px; }
.database-status { display: flex; min-width: 0; align-items: center; gap: 9px; padding: 13px 14px; border-radius: 13px; background: var(--surface-2); color: var(--text-secondary); font-size: 12px; }
.database-status small { min-width: 0; margin-left: auto; overflow: hidden; color: var(--danger); text-overflow: ellipsis; white-space: nowrap; }
@media (max-width: 600px) { .settings-actions { margin-top: -4px; }.settings-actions .el-button { width: 100%; }.database-status { align-items: flex-start; flex-wrap: wrap; }.database-status small { width: 100%; margin: 0; white-space: normal; }.database-status .el-button { margin-left: auto; } }
</style>
