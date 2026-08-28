<template>
  <div class="branding-page page-stack">
    <section class="page-hero">
      <div><h2>{{ T('Branding') }}</h2><p>{{ T('BrandingDescription') }}</p></div>
    </section>
    <div class="branding-actions"><el-button @click="restoreDefaults">{{ T('RestoreDefaults') }}</el-button><el-button type="primary" :loading="saving" @click="save">{{ T('Save') }}</el-button></div>

    <el-form label-position="top" class="branding-grid">
      <el-card class="branding-wide" shadow="never">
        <template #header><div class="branding-card-title"><strong>{{ T('SharedBrandAssets') }}</strong><small>{{ T('BrandingDescription') }}</small></div></template>
        <div class="field-grid field-grid--titles"><el-form-item :label="T('MainTitle')"><el-input v-model="form.admin_title" maxlength="120" show-word-limit/></el-form-item><el-form-item :label="T('Subtitle')"><el-input v-model="form.admin_subtitle" maxlength="120" show-word-limit/></el-form-item><el-form-item :label="`${T('WebClientBrand')} · ${T('SystemTitle')}`"><el-input v-model="form.web_client_title" maxlength="120" show-word-limit/></el-form-item></div>
        <div class="asset-theme-grid asset-theme-grid--shared">
          <AssetField v-model="form.brand_logo_light_url" :label="`${T('AdminLogo')} · ${T('LightMode')}`" :default-preview="defaultLogoLight"/>
          <AssetField v-model="form.brand_logo_dark_url" :label="`${T('AdminLogo')} · ${T('DarkMode')}`" :default-preview="defaultLogoDark" preview-tone="dark"/>
          <AssetField v-model="form.brand_icon_light_url" :label="`${T('AdminIcon')} · ${T('LightMode')}`" :default-preview="defaultIconLight"/>
          <AssetField v-model="form.brand_icon_dark_url" :label="`${T('AdminIcon')} · ${T('DarkMode')}`" :default-preview="defaultIconDark" preview-tone="dark"/>
        </div>
        <small>{{ T('SidebarUsesAdminIcon') }}</small>
      </el-card>
      <el-card class="branding-wide branding-login" shadow="never"><template #header><strong>{{ T('LoginExperience') }}</strong></template>
        <div class="asset-theme-grid asset-theme-grid--backgrounds">
          <AssetField v-model="form.login_background_light_url" :label="`${T('LoginBackground')} · ${T('LightMode')}`" default-kind="background"/>
          <AssetField v-model="form.login_background_dark_url" :label="`${T('LoginBackground')} · ${T('DarkMode')}`" default-kind="background" preview-tone="dark"/>
        </div>
        <div class="field-grid"><el-form-item :label="T('KickerText')"><el-input v-model="form.login_kicker" type="textarea" :rows="2" maxlength="160"/><small>{{ T('LoginBrandTitleHelp') }}</small></el-form-item><el-form-item :label="T('HeadingText')"><el-input v-model="form.login_heading" maxlength="240"/></el-form-item></div>
        <el-form-item :label="T('BodyCopy')"><el-input v-model="form.login_copy" type="textarea" :rows="3" maxlength="2000" show-word-limit/></el-form-item>
        <el-form-item :label="T('CustomHTML')"><el-input v-model="form.login_custom_html" type="textarea" :rows="6" maxlength="16384" show-word-limit/><small>{{ T('CustomHTMLSafety') }}</small></el-form-item>
        <el-form-item :label="T('CustomCSS')"><el-input v-model="form.login_custom_css" type="textarea" :rows="6" maxlength="16384" show-word-limit/><small>{{ T('CustomCSSSafety') }}</small></el-form-item>
      </el-card>
      <el-card shadow="never"><template #header><strong>{{ T('WebClientBrand') }}</strong></template>
        <div class="asset-theme-grid">
          <AssetField v-model="form.web_client_background_light_url" :label="`${T('WebClientBackground')} · ${T('LightMode')}`" default-kind="background"/>
          <AssetField v-model="form.web_client_background_dark_url" :label="`${T('WebClientBackground')} · ${T('DarkMode')}`" default-kind="background" preview-tone="dark"/>
        </div>
      </el-card>
      <el-card shadow="never"><template #header><strong>{{ T('FooterHTML') }}</strong></template>
        <el-form-item :label="T('FooterHTML')"><el-input v-model="form.footer_html" type="textarea" :rows="7" maxlength="2048" show-word-limit/><small>{{ T('FooterHTMLHelp') }}</small></el-form-item>
      </el-card>
    </el-form>
  </div>
</template>

<script setup>
import { defineComponent, h, onMounted, reactive, ref } from 'vue'
import { ElButton, ElFormItem, ElImage, ElInput, ElMessage, ElTag, ElUpload } from 'element-plus'
import defaultLogoDark from '@/assets/brand/starrydesk-logo-dark.svg'
import defaultIconDark from '@/assets/brand/starrydesk-icon-dark.svg'
import defaultLogoLight from '@/assets/brand/starrydesk-logo-light.svg'
import defaultIconLight from '@/assets/brand/starrydesk-icon-light.svg'
import { branding, updateBranding, uploadBrandAsset } from '@/api/config'
import { useAppStore } from '@/store/app'
import { T } from '@/utils/i18n'

const defaults = {
  admin_title: 'RustDesk API', admin_subtitle: 'KESSOKU',
  brand_logo_light_url: '', brand_logo_dark_url: '', brand_icon_light_url: '', brand_icon_dark_url: '',
  login_background_light_url: '', login_background_dark_url: '', web_client_background_light_url: '', web_client_background_dark_url: '', login_kicker: 'RustDesk API\nKESSOKU',
  login_heading: T('WelcomeToKessoku'),
  login_copy: T('AuthIntroduction'),
  footer_html: '<a href="https://github.com/q1ngyang/rustdesk-api-kessoku" target="_blank" rel="noopener noreferrer"><span>RustDesk API Kessoku</span><span>Github</span></a>',
  login_custom_html: T('DefaultCustomHTML'),
  login_custom_css: '.brand-custom p { margin: 0 0 8px; line-height: 1.65; }\n.brand-custom strong { color: var(--primary); }',
  web_client_title: 'Kessoku Remote',
}
const payloadKeys = Object.keys(defaults)
const form = reactive({ ...defaults })
const saving = ref(false)
const appStore = useAppStore()

const AssetField = defineComponent({
  props: {
    modelValue: { type: String, default: '' }, label: { type: String, required: true },
    defaultPreview: { type: String, default: '' }, defaultKind: { type: String, default: '' }, previewTone: { type: String, default: 'light' },
  },
  emits: ['update:modelValue'],
  setup (props, { emit }) {
    const upload = async ({ file }) => {
      const result = await uploadBrandAsset(file)
      emit('update:modelValue', result.data.url)
      ElMessage.success(T('ImageUploaded'))
    }
    const preview = () => props.modelValue || props.defaultPreview
    return () => h(ElFormItem, { label: props.label }, () => h('div', { class: 'asset-field' }, [
      h('div', { class: ['asset-field__visual', `asset-field__visual--${props.previewTone}`, props.defaultKind === 'background' ? 'asset-field__visual--background' : ''] }, [
        preview() ? h(ElImage, { src: preview(), fit: 'contain', class: 'asset-field__preview' }) : h('span', null, T('DefaultBackground')),
        !props.modelValue ? h(ElTag, { size: 'small', effect: 'plain', class: 'asset-field__default' }, () => T('DefaultAsset')) : null,
      ]),
      h('div', { class: 'asset-field__controls' }, [
        h(ElInput, { modelValue: props.modelValue, placeholder: 'https://cdn.example.com/brand/image.png', 'onUpdate:modelValue': value => emit('update:modelValue', value) }, { prepend: () => T('ImageURL') }),
        h('div', { class: 'asset-field__buttons' }, [
          h(ElUpload, { showFileList: false, accept: 'image/png,image/jpeg,image/webp', httpRequest: upload }, () => h(ElButton, null, () => T('UploadImage'))),
          props.modelValue ? h(ElButton, { onClick: () => emit('update:modelValue', '') }, () => T('UseDefault')) : null,
        ]),
      ]),
    ]))
  },
})

const restoreDefaults = () => Object.assign(form, defaults)
const load = async () => {
  const saved = (await branding()).data || {}
  for (const key of payloadKeys) form[key] = typeof saved[key] === 'string' ? saved[key] : defaults[key]
}
const save = async () => {
  saving.value = true
  try {
    const payload = Object.fromEntries(payloadKeys.map(key => [key, form[key]]))
    await updateBranding(payload)
    await appStore.getAdminConfig()
    ElMessage.success(T('Saved'))
  } finally { saving.value = false }
}
onMounted(load)
</script>

<style scoped lang="scss">
.branding-actions { display: flex; justify-content: flex-end; gap: 8px; margin: -8px 4px 4px; flex-wrap: wrap; }
.branding-grid { display: grid; gap: 16px; grid-template-columns: repeat(2, minmax(0, 1fr)); }
.branding-wide { grid-column: 1 / -1; }
.branding-card-title { display: flex; flex-direction: column; gap: 5px; }.branding-card-title small { margin: 0; font-weight: 400; }
.field-grid { display: grid; gap: 16px; grid-template-columns: repeat(2, minmax(0, 1fr)); }
.field-grid--titles { grid-template-columns: repeat(3, minmax(0, 1fr)); }
.asset-theme-grid { display: grid; gap: 14px; grid-template-columns: 1fr; }
.asset-theme-grid--shared,.asset-theme-grid--backgrounds { grid-template-columns: repeat(2, minmax(0, 1fr)); }
:deep(.asset-field) { display: grid; width: 100%; gap: 10px; grid-template-columns: 124px minmax(0, 1fr); }
:deep(.asset-field__visual) { position: relative; display: grid; height: 72px; overflow: hidden; place-items: center; border: 1px solid var(--border-subtle); border-radius: 12px; background: var(--surface-2); color: var(--text-tertiary); font-size: 11px; }
:deep(.asset-field__visual--dark) { border-color: color-mix(in srgb, #fff 13%, transparent); background: #151922; }
:deep(.asset-field__visual--background) { background: radial-gradient(circle at 72% 30%, var(--primary-soft), transparent 48%), linear-gradient(135deg, var(--surface-1), var(--surface-3)); }
:deep(.asset-field__preview) { width: 100%; height: 100%; }
:deep(.asset-field__default) { position: absolute; right: 5px; bottom: 5px; background: var(--surface-1); }
:deep(.asset-field__controls) { display: grid; min-width: 0; align-content: center; gap: 8px; }
:deep(.asset-field__buttons) { display: flex; gap: 8px; }
small { display: block; margin-top: 6px; color: var(--text-tertiary); line-height: 1.5; }
@media (max-width: 1180px) { .field-grid--titles { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
@media (max-width: 800px) { .branding-grid,.field-grid,.field-grid--titles,.asset-theme-grid,.asset-theme-grid--shared,.asset-theme-grid--backgrounds { grid-template-columns: 1fr; }.branding-wide { grid-column: auto; } }
@media (max-width: 520px) { :deep(.asset-field) { grid-template-columns: 1fr; }:deep(.asset-field__visual) { height: 92px; }.branding-actions,.branding-actions .el-button { width: 100%; } }
</style>
