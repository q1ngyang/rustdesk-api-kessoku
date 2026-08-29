<template>
  <div class="platform-page page-stack">
    <section class="page-hero"><div><h2>{{ T('PlatformSettings') }}</h2><p>{{ T('PlatformSettingsDescription') }}</p></div></section>
    <div class="settings-actions"><el-button type="primary" :loading="saving" @click="save">{{ T('Save') }}</el-button></div>

    <div class="platform-grid">
      <el-card shadow="never">
        <template #header><div class="setting-heading"><span class="setting-heading__icon"><el-icon><Clock/></el-icon></span><div><strong>{{ T('LoginValidity') }}</strong><small>{{ T('LoginValidityDescription', { hours: form.login_maximum_hours }) }}</small></div></div></template>
        <div class="setting-list">
          <div class="setting-row"><div><strong>{{ T('WebLoginValidity') }}</strong><small>{{ T('WebLoginValidityHelp') }}</small></div><span><el-input-number v-model="form.web_login_hours" :min="1" :max="form.login_maximum_hours || 168"/><em>{{ T('HourUnit') }}</em></span></div>
          <div class="setting-row"><div><strong>{{ T('ClientLoginValidity') }}</strong><small>{{ T('ClientLoginValidityHelp') }}</small></div><span><el-input-number v-model="form.client_login_hours" :min="1" :max="form.login_maximum_hours || 168"/><em>{{ T('HourUnit') }}</em></span></div>
        </div>
        <el-alert class="policy-alert" :title="T('LoginValidityMaximumHelp', { hours: form.login_maximum_hours })" type="info" :closable="false" show-icon/>
      </el-card>

      <el-card shadow="never">
        <template #header><div class="setting-heading"><span class="setting-heading__icon setting-heading__icon--data"><el-icon><DataAnalysis/></el-icon></span><div><strong>{{ T('DataRetention') }}</strong><small>{{ T('DataRetentionDescription') }}</small></div></div></template>
        <div class="setting-list retention-list">
          <div v-for="item in retentionFields" :key="item.key" class="setting-row"><div><strong>{{ T(item.label) }}</strong><small>{{ T(item.help) }}</small></div><span><el-input-number v-model="form[item.key]" :min="0" :max="3650"/><em>{{ form[item.key] === 0 ? T('NeverCleanup') : T('DayUnit') }}</em></span></div>
        </div>
        <el-alert class="policy-alert" :title="T('RetentionZeroHelp')" type="info" :closable="false" show-icon/>
        <el-alert class="policy-alert" :title="T('RetentionCleanupHelp')" type="warning" :closable="false" show-icon/>
      </el-card>
    </div>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { Clock, DataAnalysis } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { T } from '@/utils/i18n'
import { loadSystemSettingForm, saveSystemSettingForm, systemSettingDefaults } from './shared'

const form = reactive({ ...systemSettingDefaults })
const saving = ref(false)
const retentionFields = [
  { key: 'user_token_retention_days', label: 'UserTokenRetention', help: 'UserTokenRetentionHelp' },
  { key: 'login_log_retention_days', label: 'LoginLogRetention', help: 'LoginLogRetentionHelp' },
  { key: 'audit_conn_retention_days', label: 'ConnectionLogRetention', help: 'ConnectionLogRetentionHelp' },
  { key: 'audit_file_retention_days', label: 'FileLogRetention', help: 'FileLogRetentionHelp' },
  { key: 'control_audit_retention_days', label: 'ControlAuditRetention', help: 'ControlAuditRetentionHelp' },
]
const load = () => loadSystemSettingForm(form)
const save = async () => {
  saving.value = true
  try { await saveSystemSettingForm(form); await load(); ElMessage.success(T('Saved')) } finally { saving.value = false }
}
onMounted(load)
</script>

<style scoped lang="scss">
.platform-page { max-width: 1040px; margin: 0 auto; }.settings-actions { display: flex; justify-content: flex-end; margin: -8px 4px 4px; }.platform-grid { display: grid; gap: 16px; grid-template-columns: 1fr; }.setting-heading { display: flex; min-width: 0; align-items: center; gap: 12px; }.setting-heading > div { display: flex; min-width: 0; flex: 1; flex-direction: column; }.setting-heading strong { color: var(--text-primary); font-size: 14px; }.setting-heading small { margin-top: 3px; color: var(--text-tertiary); font-size: 11px; line-height: 1.5; }.setting-heading__icon { display: grid; width: 40px; height: 40px; flex: 0 0 auto; place-items: center; border-radius: 13px; background: var(--primary-soft); color: var(--primary); font-size: 17px; }.setting-heading__icon--data { background: color-mix(in srgb, var(--kessoku-yellow) 18%, var(--surface-1)); color: #a77b10; }.setting-list { display: grid; gap: 0; }.setting-row { display: flex; min-width: 0; align-items: center; justify-content: space-between; gap: 20px; padding: 15px 2px; border-bottom: 1px solid var(--border-subtle); }.setting-row:last-child { border-bottom: 0; }.setting-row > div { display: flex; min-width: 0; flex: 1; flex-direction: column; gap: 4px; }.setting-row strong { color: var(--text-primary); font-size: 12px; }.setting-row small { color: var(--text-tertiary); font-size: 10px; line-height: 1.55; }.setting-row > span { display: flex; flex: 0 0 auto; align-items: center; gap: 8px; }.setting-row em { color: var(--text-tertiary); font-size: 11px; font-style: normal; }.setting-row :deep(.el-input-number) { width: 148px; }.policy-alert { margin-top: 14px; }.retention-list { grid-template-columns: repeat(2, minmax(0, 1fr)); column-gap: 28px; }.retention-list .setting-row:last-child:nth-child(odd) { grid-column: 1 / -1; }
@media (max-width: 760px) { .retention-list { grid-template-columns: 1fr; }.retention-list .setting-row:last-child:nth-child(odd) { grid-column: auto; }.setting-row { align-items: flex-start; flex-direction: column; gap: 10px; }.setting-row > span { width: 100%; }.setting-row :deep(.el-input-number) { width: min(220px, calc(100% - 42px)); }.settings-actions .el-button { width: 100%; } }
</style>
