<template>
  <div class="settings-page page-stack">
    <section class="page-hero">
      <div><h2>{{ T('AnnouncementSettings') }}</h2><p>{{ T('AnnouncementSettingsDescription') }}</p></div>
    </section>
    <div class="settings-actions"><el-button type="primary" :loading="saving" @click="save">{{ T('Save') }}</el-button></div>

    <el-card shadow="never">
      <template #header><div class="setting-heading"><span class="setting-heading__icon"><el-icon><Bell/></el-icon></span><div><strong>{{ T('Announcement') }}</strong><small>{{ T('AnnouncementHelp') }}</small></div></div></template>
      <el-form-item :label="T('AnnouncementContent')">
        <el-input v-model="form.announcement" type="textarea" :rows="9" maxlength="16384" show-word-limit/>
      </el-form-item>
      <el-alert :title="T('MarkdownSupported')" type="info" :closable="false" show-icon/>
    </el-card>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { Bell } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { useAppStore } from '@/store/app'
import { T } from '@/utils/i18n'
import { loadSystemSettingForm, saveSystemSettingForm, systemSettingDefaults } from './shared'

const appStore = useAppStore()
const form = reactive({ ...systemSettingDefaults })
const saving = ref(false)
const load = () => loadSystemSettingForm(form)
const save = async () => {
  saving.value = true
  try {
    await saveSystemSettingForm(form)
    await Promise.all([load(), appStore.getAdminConfig()])
    ElMessage.success(T('Saved'))
  } finally { saving.value = false }
}
onMounted(load)
</script>

<style scoped lang="scss">
.settings-page { max-width: 980px; margin: 0 auto; }
.settings-actions { display: flex; justify-content: flex-end; margin: -8px 4px 4px; }
.setting-heading { display: flex; min-width: 0; align-items: center; gap: 12px; }
.setting-heading > div { display: flex; min-width: 0; flex: 1; flex-direction: column; }
.setting-heading strong { color: var(--text-primary); font-size: 14px; }
.setting-heading small { margin-top: 3px; color: var(--text-tertiary); font-size: 11px; }
.setting-heading__icon { display: grid; width: 40px; height: 40px; flex: 0 0 auto; place-items: center; border-radius: 13px; background: color-mix(in srgb, var(--kessoku-pink) 15%, var(--surface-1)); color: #c35d87; font-size: 17px; }
@media (max-width: 600px) { .settings-actions { margin-top: -4px; }.settings-actions .el-button { width: 100%; } }
</style>
