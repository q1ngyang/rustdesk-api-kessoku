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

    <section class="versions-section">
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
import { about } from '@/api/config'
import DeploymentAsset from '@/components/brand/DeploymentAsset.vue'
import StarryIcon from '@/components/brand/StarryIcon.vue'
import StatusValue from '@/components/common/StatusValue.vue'
import { useAppStore } from '@/store/app'
import { T } from '@/utils/i18n'

const info = ref({ kessoku: {}, starry: {}, instances: [] })
const loading = ref(false)
const appStore = useAppStore()
const branding = computed(() => appStore.setting.branding)
const columns = computed(() => appStore.setting.viewportWidth < 700 ? 1 : 3)
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
.versions-section { margin-top: 14px; padding: 22px; border: 1px solid var(--border-subtle); border-radius: 22px; background: var(--surface-1); }.section-heading,.instance-heading { display: flex; align-items: center; justify-content: space-between; gap: 14px; }.section-heading h3 { margin: 0; }.section-heading p { margin: 5px 0 0; color: var(--text-tertiary); font-size: 11px; }.instance-card { margin-top: 14px; }.instance-heading > div { display: flex; min-width: 0; flex-direction: column; gap: 4px; }.instance-heading code { color: var(--text-tertiary); font-size: 10px; }.relay-table { margin-top: 14px; }
@media (max-width: 900px) { .project-grid { grid-template-columns: 1fr; } }
@media (max-width: 700px) { .about-hero { min-height: 360px; padding: 28px 20px; border-radius: 22px; }.about-hero__copy { max-width: 100%; }.about-hero__logo { top: auto; right: 18px; bottom: 18px; max-width: 72%; opacity: .62; }.project-card { grid-template-columns: auto minmax(0, 1fr); }.project-card > .el-tag { grid-column: 2; justify-self: start; } }
</style>
