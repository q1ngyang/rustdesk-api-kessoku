<template>
  <div class="about-page">
    <section class="about-hero">
      <div class="about-hero__logo"><DeploymentAsset variant="logo" width="min(360px, 76vw)" :light-url="branding.brand_logo_light_url" :dark-url="branding.brand_logo_dark_url" alt=""/></div>
      <div class="about-hero__copy"><span class="about-hero__eyebrow">{{ T('About') }}</span><h2>RustDesk API KESSOKU</h2><p>{{ T('AboutDescription') }}</p></div>
    </section>

    <section class="project-grid">
      <article class="project-card">
        <DeploymentAsset variant="icon" width="46" :light-url="branding.brand_icon_light_url" :dark-url="branding.brand_icon_dark_url" alt=""/><div><small>KESSOKU</small><h3>RustDesk API Kessoku</h3><p>{{ T('KessokuProjectIntroduction') }}</p><a :href="info.kessoku?.github" target="_blank" rel="noopener noreferrer">GitHub ↗</a></div>
        <el-tag>{{ T('Version') }} {{ info.kessoku?.version || '-' }}</el-tag>
      </article>
      <article class="project-card project-card--starry">
        <StarryIcon :size="46"/><div><small>STARRY</small><h3>RustDesk Server Starry</h3><p>{{ T('StarryProjectIntroduction') }}</p><a :href="info.starry?.github" target="_blank" rel="noopener noreferrer">GitHub ↗</a></div>
      </article>
    </section>

    <section id="client-connection" class="connection-section">
      <div class="section-heading connection-heading">
        <div><h3>{{ T('ConnectionServerInformation') }}</h3><p>{{ T('ConnectionServerInformationDescription') }}</p></div>
        <el-tag effect="plain" round>{{ T('OfficialClientConfiguration') }}</el-tag>
      </div>
      <div class="connection-layout">
        <div class="connection-content">
          <dl class="server-details">
            <div><dt>{{ T('IDServer') }}</dt><dd><code>{{ connection.id_server || T('NotConfigured') }}</code></dd></div>
            <div class="relay-blank"><dt>{{ T('RelayServer') }}</dt><dd><strong>{{ T('LeaveBlankRecommended') }}</strong><small>{{ T('RelayLeaveBlankHelp') }}</small></dd></div>
            <div><dt>{{ T('APIServer') }}</dt><dd><code>{{ connection.api_server || T('NotConfigured') }}</code></dd></div>
            <div><dt>{{ T('Key') }}</dt><dd><code class="server-key">{{ connection.key || T('NotConfigured') }}</code></dd></div>
          </dl>

          <div class="relay-navigation"><div><strong>{{ T('RelayNodeStatus') }}</strong><small>{{ T('RelayNodeStatusHelp') }}</small></div><el-button plain @click="scrollToVersions">{{ T('ViewRelayNodeStatus') }}</el-button></div>

          <div class="config-string">
            <div class="config-string__heading"><strong>{{ T('ConfigurationString') }}</strong><span>{{ T('ConfigurationStringDescription') }}</span></div>
            <div class="config-string__value"><code>{{ connection.config_string || T('NotConfigured') }}</code><el-button type="primary" :icon="CopyDocument" :disabled="!connectionReady" @click="copyConfiguration">{{ T('Copy') }}</el-button></div>
          </div>
        </div>

        <aside class="configuration-qr">
          <div v-if="connectionReady" class="configuration-qr__canvas"><QrcodeVue :value="connection.config_string" :size="174" level="M" render-as="svg" :margin="2"/></div>
          <el-empty v-else :image-size="70" :description="T('ClientConfigurationIncomplete')"/>
          <strong>{{ T('ScanToConfigure') }}</strong><small>{{ T('ScanToConfigureDescription') }}</small>
        </aside>
      </div>
      <div class="connection-guide">
        <div><span>1</span><p>{{ T('ClientConfigurationStepOne') }}</p></div>
        <div><span>2</span><p>{{ T('ClientConfigurationStepTwo') }}</p></div>
        <div><span>3</span><p>{{ T('ClientConfigurationStepThree') }}</p></div>
      </div>
      <el-alert class="official-client-alert" :title="T('OfficialClientConfigurationRequired')" :description="T('OfficialClientConfigurationRequiredDescription')" type="warning" show-icon :closable="false"/>
    </section>

    <section id="deployment-versions" class="versions-section">
      <div class="section-heading"><div><h3>{{ T('DeploymentVersions') }}</h3><p>{{ T('DeploymentVersionsDescription') }}</p></div><el-button :loading="loading" @click="load">{{ T('Refresh') }}</el-button></div>
      <el-empty v-if="!loading && !(info.instances || []).length" :description="T('NoVisibleStarryInstances')"/>
      <el-card v-for="instance in info.instances || []" :key="instance.id" class="instance-card" shadow="never">
        <template #header><div class="instance-heading"><div><strong>{{ instance.name }}</strong><code>{{ instance.id }}</code></div><StatusValue :value="instance.available" :text="instance.available ? T('Available') : T('NotAvailable')"/></div></template>
        <el-descriptions :column="columns" border>
          <el-descriptions-item label="Starry">{{ instance.starry_version || '-' }}</el-descriptions-item>
          <el-descriptions-item :label="T('UpstreamVersion')">{{ instance.upstream_version || '-' }}</el-descriptions-item>
          <el-descriptions-item :label="T('ControlProtocol')">{{ instance.protocol_version || '-' }}</el-descriptions-item>
          <el-descriptions-item v-if="instance.error" :label="T('State')"><StatusValue :value="instance.error"/></el-descriptions-item>
        </el-descriptions>
        <el-table v-if="instance.relays?.length" :data="instance.relays" class="relay-table">
          <el-table-column prop="id" :label="T('Relay')" min-width="220"/>
          <el-table-column :label="T('Version')" min-width="150"><template #default="{ row }">{{ row.version === 'not_reported' ? T('VersionNotReported') : row.version }}</template></el-table-column>
          <el-table-column :label="T('Native')" width="140"><template #default="{ row }"><StatusValue :value="row.native_state"/></template></el-table-column>
          <el-table-column label="WSS" width="140"><template #default="{ row }"><StatusValue :value="row.wss_state"/></template></el-table-column>
        </el-table>
      </el-card>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { CopyDocument } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import QrcodeVue from 'qrcode.vue'
import { about } from '@/api/config'
import DeploymentAsset from '@/components/brand/DeploymentAsset.vue'
import StarryIcon from '@/components/brand/StarryIcon.vue'
import StatusValue from '@/components/common/StatusValue.vue'
import { useAppStore } from '@/store/app'
import { T } from '@/utils/i18n'

const info = ref({ kessoku: {}, starry: {}, client_connection: {}, instances: [] })
const loading = ref(false)
const appStore = useAppStore()
const branding = computed(() => appStore.setting.branding)
const connection = computed(() => info.value.client_connection || {})
const connectionReady = computed(() => Boolean(connection.value.id_server && connection.value.api_server && connection.value.key && connection.value.config_string))
const columns = computed(() => appStore.setting.viewportWidth < 700 ? 1 : 3)
const scrollToVersions = () => document.querySelector('#deployment-versions')?.scrollIntoView({ behavior: 'smooth', block: 'start' })
const copyConfiguration = async () => {
  if (!connectionReady.value) return
  try {
    await navigator.clipboard.writeText(connection.value.config_string)
    ElMessage.success(T('CopySuccess'))
  } catch {
    const input = document.createElement('textarea')
    input.value = connection.value.config_string
    input.setAttribute('readonly', '')
    input.style.position = 'fixed'
    input.style.opacity = '0'
    document.body.appendChild(input)
    input.select()
    const copied = document.execCommand('copy')
    input.remove()
    copied ? ElMessage.success(T('CopySuccess')) : ElMessage.error(T('CopyFailed'))
  }
}
const load = async () => {
  loading.value = true
  try { info.value = (await about()).data || info.value } finally { loading.value = false }
}
onMounted(load)
</script>

<style scoped lang="scss">
.about-page { width: min(1040px, 100%); margin: 0 auto; }
.about-hero { position: relative; min-height: 250px; overflow: hidden; box-sizing: border-box; padding: clamp(34px, 6vw, 70px); border: 1px solid var(--border-subtle); border-radius: 28px; background: var(--surface-1); box-shadow: var(--shadow-md); }
.about-hero::before { position: absolute; top: -90px; right: -30px; width: 520px; height: 330px; border-radius: 50%; background: radial-gradient(circle, color-mix(in srgb, var(--primary) 14%, transparent), transparent 68%); content: ''; }
.about-hero__copy { position: relative; z-index: 2; max-width: min(680px, 72%); }
.about-hero__logo { position: absolute; top: 32px; right: 34px; z-index: 1; display: flex; justify-content: flex-end; opacity: .7; }
.about-hero__eyebrow { color: var(--primary); font-size: clamp(25px, 3vw, 32px); font-weight: 800; letter-spacing: .03em; }
.about-hero h2 { margin: 8px 0 14px; color: var(--text-primary); font-size: clamp(27px, 3.4vw, 36px); line-height: 1.12; letter-spacing: -.035em; }
.about-hero p { max-width: 660px; margin: 0; color: var(--text-secondary); font-size: 14px; line-height: 1.8; }
.project-grid { display: grid; gap: 14px; margin-top: 14px; grid-template-columns: repeat(2, minmax(0, 1fr)); }
.project-card { display: grid; min-width: 0; align-items: start; gap: 15px; padding: 22px; border: 1px solid var(--border-subtle); border-radius: 20px; background: var(--surface-1); box-shadow: var(--shadow-sm); grid-template-columns: auto minmax(0, 1fr) auto; }
.project-card--starry { background: linear-gradient(135deg, color-mix(in srgb, var(--kessoku-pink) 8%, var(--surface-1)), var(--surface-1)); }.project-card small { color: var(--primary); font-size: 10px; font-weight: 800; letter-spacing: .1em; }.project-card h3 { margin: 5px 0 7px; font-size: 15px; }.project-card p { margin: 0 0 10px; color: var(--text-secondary); font-size: 11px; line-height: 1.65; }.project-card a { color: var(--primary); font-size: 11px; font-weight: 700; text-decoration: none; }
.connection-section { margin-top: 14px; padding: clamp(20px, 3vw, 28px); border: 1px solid var(--border-subtle); border-radius: 22px; background: var(--surface-1); box-shadow: var(--shadow-sm); scroll-margin-top: 18px; }
.connection-heading { align-items: flex-start; }
.connection-heading :deep(.el-tag) { flex: none; }
.connection-layout { display: grid; align-items: stretch; gap: 18px; margin-top: 20px; grid-template-columns: minmax(0, 1fr) 216px; }
.connection-content { display: grid; min-width: 0; gap: 16px; }
.server-details { display: grid; overflow: hidden; margin: 0; border: 1px solid var(--border-subtle); border-radius: 16px; background: var(--surface-2); grid-template-columns: repeat(2, minmax(0, 1fr)); }
.server-details > div { display: grid; min-width: 0; gap: 7px; padding: 15px 16px; border-right: 1px solid var(--border-subtle); border-bottom: 1px solid var(--border-subtle); }
.server-details > div:nth-child(2n) { border-right: 0; }.server-details > div:nth-last-child(-n + 2) { border-bottom: 0; }
.server-details dt { color: var(--text-tertiary); font-size: 11px; font-weight: 700; }.server-details dd { min-width: 0; margin: 0; }.server-details code { display: block; overflow-wrap: anywhere; color: var(--text-primary); font-family: var(--font-mono, ui-monospace, monospace); font-size: 12px; line-height: 1.55; }.server-key { color: var(--primary) !important; }
.relay-blank dd { display: flex; min-width: 0; flex-direction: column; gap: 4px; }.relay-blank strong { color: var(--success); font-size: 13px; }.relay-blank small { color: var(--text-tertiary); font-size: 10px; line-height: 1.45; }
.relay-navigation { display: flex; min-width: 0; align-items: center; justify-content: space-between; gap: 14px; padding: 12px 14px; border-radius: 14px; background: var(--surface-2); }.relay-navigation > div { display: flex; min-width: 0; flex-direction: column; gap: 4px; }.relay-navigation strong,.config-string strong { font-size: 12px; }.relay-navigation small { color: var(--text-tertiary); font-size: 10px; line-height: 1.5; }
.config-string { display: grid; gap: 9px; }.config-string__heading { display: flex; align-items: baseline; justify-content: space-between; gap: 12px; }.config-string__heading span { color: var(--text-tertiary); font-size: 10px; text-align: right; }.config-string__value { display: grid; align-items: center; gap: 10px; padding: 10px 10px 10px 14px; border: 1px solid var(--border-subtle); border-radius: 14px; background: var(--surface-2); grid-template-columns: minmax(0, 1fr) auto; }.config-string__value code { overflow: hidden; color: var(--text-secondary); font-family: var(--font-mono, ui-monospace, monospace); font-size: 10px; text-overflow: ellipsis; white-space: nowrap; }
.configuration-qr { display: flex; min-width: 0; align-items: center; justify-content: center; box-sizing: border-box; padding: 18px; flex-direction: column; border: 1px solid var(--border-subtle); border-radius: 18px; background: linear-gradient(145deg, var(--surface-2), var(--surface-1)); text-align: center; }.configuration-qr__canvas { display: grid; overflow: hidden; margin-bottom: 12px; padding: 8px; place-items: center; border: 1px solid color-mix(in srgb, var(--text-primary) 8%, transparent); border-radius: 13px; background: #fff; }.configuration-qr strong { font-size: 12px; }.configuration-qr small { margin-top: 6px; color: var(--text-tertiary); font-size: 10px; line-height: 1.55; }
.connection-guide { display: grid; gap: 10px; margin-top: 18px; grid-template-columns: repeat(3, minmax(0, 1fr)); }.connection-guide > div { display: flex; min-width: 0; align-items: flex-start; gap: 9px; padding: 12px; border-radius: 13px; background: var(--surface-2); }.connection-guide span { display: grid; width: 22px; height: 22px; flex: 0 0 22px; place-items: center; border-radius: 50%; background: var(--primary-soft); color: var(--primary); font-size: 10px; font-weight: 800; }.connection-guide p { margin: 1px 0 0; color: var(--text-secondary); font-size: 10px; line-height: 1.55; }.official-client-alert { margin-top: 14px; }
.versions-section { margin-top: 14px; padding: 22px; border: 1px solid var(--border-subtle); border-radius: 22px; background: var(--surface-1); scroll-margin-top: 18px; }.section-heading,.instance-heading { display: flex; align-items: center; justify-content: space-between; gap: 14px; }.section-heading h3 { margin: 0; }.section-heading p { margin: 5px 0 0; color: var(--text-tertiary); font-size: 11px; }.instance-card { margin-top: 14px; }.instance-heading > div { display: flex; min-width: 0; flex-direction: column; gap: 4px; }.instance-heading code { color: var(--text-tertiary); font-size: 10px; }.relay-table { margin-top: 14px; }
@media (max-width: 900px) { .project-grid { grid-template-columns: 1fr; }.connection-layout { grid-template-columns: minmax(0, 1fr) 200px; }.connection-guide { grid-template-columns: 1fr; } }
@media (max-width: 700px) { .about-hero { min-height: 360px; padding: 28px 20px; border-radius: 22px; }.about-hero__copy { max-width: 100%; }.about-hero__logo { top: auto; right: 18px; bottom: 18px; max-width: 72%; opacity: .62; }.project-card { grid-template-columns: auto minmax(0, 1fr); }.project-card > .el-tag { grid-column: 2; justify-self: start; }.connection-layout { grid-template-columns: 1fr; }.configuration-qr { justify-self: stretch; }.server-details { grid-template-columns: 1fr; }.server-details > div,.server-details > div:nth-child(2n),.server-details > div:nth-last-child(-n + 2) { border-right: 0; border-bottom: 1px solid var(--border-subtle); }.server-details > div:last-child { border-bottom: 0; }.config-string__heading { align-items: flex-start; flex-direction: column; }.config-string__heading span { text-align: left; } }
@media (max-width: 460px) { .connection-heading { align-items: flex-start; flex-direction: column; }.config-string__value { grid-template-columns: 1fr; }.config-string__value code { padding: 3px 0; }.config-string__value .el-button { width: 100%; } }
</style>
