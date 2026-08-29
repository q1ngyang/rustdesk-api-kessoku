<template>
  <div class="server-control">
    <div class="page-heading">
      <div class="page-heading__starry">
        <StarryLogo class="page-heading__starry-logo" width="308"/>
        <div>
          <h2>{{ T('ServerControl') }}</h2>
          <p>{{ T('ServerControlDescription') }}</p>
        </div>
      </div>
      <div v-if="selectedInstance" class="page-heading__actions">
        <StatusValue :value="selectedInstance.available" :text="selectedInstance.available ? T('Available') : selectedInstance.error_code || T('NotAvailable')"/>
        <el-tag v-if="selectedInstance.read_only" type="warning" effect="light">{{ T('ReadOnlyMode') }}</el-tag>
      </div>
    </div>
    <el-alert
      class="policy-alert"
      :title="T('TypedStarryControlAPI')"
      :description="T('TypedStarryControlDescription')"
      type="info"
      :closable="false"
      show-icon
    />

    <el-card class="toolbar" shadow="never">
      <div class="instance-toolbar">
        <div class="instance-toolbar__copy">
          <small>{{ T('ManagedInstance') }}</small>
          <strong>{{ selectedInstance?.name || T('SelectInstance') }}</strong>
        </div>
        <el-select v-model="selectedID" :placeholder="T('SelectInstance')" style="width: 360px" @change="loadSelected">
          <el-option
            v-for="item in instances"
            :key="item.id"
            :label="`${item.name} (${item.id})`"
            :value="item.id"
            :disabled="!item.enabled || !item.available"
          />
        </el-select>
        <el-button :loading="loading.instances" @click="loadInstances">{{ T('Refresh') }}</el-button>
      </div>
    </el-card>

    <el-empty v-if="!selectedID" :description="T('NoStarryInstance')" />

    <el-tabs v-else v-model="activeTab" class="control-tabs">
      <el-tab-pane :label="T('Status')" name="status">
        <el-row :gutter="16">
          <el-col :xs="24" :lg="12">
            <el-card class="status-card" shadow="never" v-loading="loading.status">
              <template #header>{{ T('Runtime') }}</template>
              <el-descriptions :column="detailColumns" border>
                <el-descriptions-item><template #label><InfoLabel :label="T('Ready')" :help="T('ReadyHelp')"/></template><StatusValue :value="status.ready"/></el-descriptions-item>
                <el-descriptions-item><template #label><InfoLabel :label="T('State')" :help="T('RuntimeStateHelp')"/></template><StatusValue :value="status.config?.status || '-'"/></el-descriptions-item>
                <el-descriptions-item><template #label><InfoLabel :label="T('Generation')" :help="T('GenerationHelp')"/></template>{{ status.config?.generation ?? '-' }}</el-descriptions-item>
                <el-descriptions-item><template #label><InfoLabel :label="T('Schema')" :help="T('SchemaHelp')"/></template>{{ status.config?.schema_version ?? '-' }}</el-descriptions-item>
                <el-descriptions-item :span="2"><template #label><InfoLabel :label="T('SourceDigest')" :help="T('SourceDigestHelp')"/></template><code>{{ status.config?.source_digest || '-' }}</code></el-descriptions-item>
                <el-descriptions-item :span="2"><template #label><InfoLabel :label="T('LastError')" :help="T('LastErrorHelp')"/></template>{{ status.config?.last_error || '-' }}</el-descriptions-item>
              </el-descriptions>
            </el-card>
          </el-col>
          <el-col :xs="24" :lg="12">
            <el-card class="status-card" shadow="never" v-loading="loading.capabilities">
              <template #header>{{ T('ContractAndAuthentication') }}</template>
              <el-descriptions :column="detailColumns" border>
                <el-descriptions-item><template #label><InfoLabel :label="T('Protocol')" :help="T('ProtocolHelp')"/></template>{{ capabilities.protocol?.version || '-' }}</el-descriptions-item>
                <el-descriptions-item><template #label><InfoLabel label="Starry" :help="T('StarryVersionHelp')"/></template>{{ capabilities.instance?.starry_version || '-' }}</el-descriptions-item>
                <el-descriptions-item><template #label><InfoLabel :label="T('ConfiguredAuth')" :help="T('ConfiguredAuthHelp')"/></template><StatusValue :value="status.auth?.configured_mode || '-'"/></el-descriptions-item>
                <el-descriptions-item><template #label><InfoLabel :label="T('EffectiveAuth')" :help="T('EffectiveAuthHelp')"/></template><StatusValue :value="status.auth?.effective_mode || '-'"/></el-descriptions-item>
                <el-descriptions-item><template #label><InfoLabel :label="T('Verifier')" :help="T('VerifierHelp')"/></template><StatusValue :value="status.auth?.verifier_state || '-'"/></el-descriptions-item>
                <el-descriptions-item><template #label><InfoLabel :label="T('Keys')" :help="T('KeysHelp')"/></template>{{ status.auth?.key_count ?? '-' }}</el-descriptions-item>
                <el-descriptions-item :span="2"><template #label><InfoLabel :label="T('SchemaDigest')" :help="T('SchemaDigestHelp')"/></template><code>{{ capabilities.config?.schema_digest || '-' }}</code></el-descriptions-item>
              </el-descriptions>
            </el-card>
          </el-col>
        </el-row>
        <el-card class="section" shadow="never">
          <template #header>{{ T('AuthenticationMetrics') }}</template>
          <el-descriptions :column="metricColumns" border>
            <el-descriptions-item v-for="(value, name) in status.auth?.metrics || {}" :key="name">
              <template #label><InfoLabel :label="formatMetricName(name)" :help="metricHelp(name)"/></template>
              {{ value }}
            </el-descriptions-item>
          </el-descriptions>
        </el-card>
      </el-tab-pane>

      <el-tab-pane :label="T('RelaysAndSimulation')" name="relays">
        <el-card shadow="never" v-loading="loading.relays">
          <template #header>
            <div class="card-header">
              <span>{{ T('RelayInventory') }} · {{ T('Generation') }} {{ relays.config_generation ?? '-' }}</span>
              <el-button @click="loadRelays">{{ T('Refresh') }}</el-button>
            </div>
          </template>
          <el-alert v-if="relays.warning" :title="relays.warning" type="warning" :closable="false" />
          <el-table :data="relays.relays || []" border>
            <el-table-column prop="id" :label="T('Relay')" min-width="220" />
            <el-table-column prop="configured_order" :label="T('Order')" width="90" />
            <el-table-column :label="T('Native')" width="120"><template #default="{ row }"><StatusValue :value="row.native?.state || '-'"/></template></el-table-column>
            <el-table-column label="WSS" width="120"><template #default="{ row }"><StatusValue :value="row.websocket?.state || '-'"/></template></el-table-column>
            <el-table-column prop="websocket.latency_ms" :label="T('WSSLatency')" width="150" />
            <el-table-column :label="T('EligibleFor')" min-width="180">
              <template #default="{ row }">{{ (row.eligible_for || []).join(', ') || '-' }}</template>
            </el-table-column>
            <el-table-column :label="T('Rules')" min-width="180">
              <template #default="{ row }">{{ (row.referenced_by_rules || []).join(', ') || '-' }}</template>
            </el-table-column>
          </el-table>
        </el-card>

        <el-card class="section" shadow="never" v-loading="loading.simulation">
          <template #header>{{ T('AllocationSimulation') }}</template>
          <el-form inline :model="simulationInput">
            <el-form-item :label="T('ClientAIP')"><el-input v-model="simulationInput.clientA" placeholder="203.0.113.10" /></el-form-item>
            <el-form-item :label="T('ClientBIP')"><el-input v-model="simulationInput.clientB" placeholder="2001:db8::20" /></el-form-item>
            <el-form-item :label="T('Transport')">
              <el-select v-model="simulationInput.transport" style="width: 130px">
                <el-option label="native" value="native" />
                <el-option label="wss" value="wss" />
                <el-option label="mixed" value="mixed" />
              </el-select>
            </el-form-item>
            <el-form-item><el-button type="primary" @click="simulate">{{ T('Simulate') }}</el-button></el-form-item>
          </el-form>
          <template v-if="simulationResult">
            <el-descriptions :column="wideColumns" border>
              <el-descriptions-item :label="T('Rule')">{{ simulationResult.matched_rule?.name || T('OfficialFallback') }}</el-descriptions-item>
              <el-descriptions-item :label="T('Selection')">{{ simulationResult.selection?.kind }}</el-descriptions-item>
              <el-descriptions-item :label="T('Relay')">{{ simulationResult.selection?.relay_id || '-' }}</el-descriptions-item>
            </el-descriptions>
            <el-table :data="simulationResult.candidates || []" border class="section">
              <el-table-column prop="relay_id" :label="T('Candidate')" min-width="220" />
              <el-table-column :label="T('Eligible')" width="110"><template #default="{ row }"><StatusValue :value="row.eligible"/></template></el-table-column>
              <el-table-column prop="priority" :label="T('Priority')" width="100" />
              <el-table-column prop="exclusion_reason" :label="T('ExclusionReason')" min-width="220" />
            </el-table>
          </template>
        </el-card>
      </el-tab-pane>

      <el-tab-pane :label="T('Configuration')" name="config">
        <el-alert v-if="configDocument.drift" :title="T('ConfigDriftDetected')" type="error" :closable="false" />
        <el-alert v-if="!canPlan" :title="T('ControlModeReadOnlyTitle')" :description="T('ControlModeReadOnlyDescription')" type="warning" :closable="false" show-icon />
        <el-collapse class="control-mode-guide">
          <el-collapse-item name="guide">
            <template #title><span class="control-mode-guide__title">{{ T('ControlModeGuide') }}<el-tag :type="canWrite ? 'success' : 'warning'" size="small" effect="light">{{ canWrite ? T('WriteControlEnabled') : T('ReadOnlyMode') }}</el-tag></span></template>
            <div class="control-mode-guide__content">
              <ol><li>{{ T('ControlModeStepOne') }}</li><li>{{ T('ControlModeStepTwo') }}</li><li>{{ T('ControlModeStepThree') }}</li><li>{{ T('ControlModeStepFour') }}</li></ol>
              <el-alert :title="T('ControlModeRiskTitle')" :description="T('ControlModeRiskDescription')" type="error" :closable="false" show-icon/>
            </div>
          </el-collapse-item>
        </el-collapse>
        <el-card shadow="never" v-loading="loading.config">
          <template #header>
            <div class="card-header">
              <span>{{ T('ManagedYAML') }}</span>
              <el-space>
                <el-button @click="loadConfig">{{ T('DiscardAndReload') }}</el-button>
                <el-button :loading="loading.validation" @click="validateCandidate">{{ T('Validate') }}</el-button>
                <el-button type="primary" :disabled="!canPlan" :loading="loading.plan" @click="createPlan">{{ T('CreateReviewPlan') }}</el-button>
                <el-button type="warning" :disabled="!canWrite || !configDocument.source_digest" @click="reloadActiveRuntime">{{ T('ReloadRuntime') }}</el-button>
              </el-space>
            </div>
          </template>
          <el-descriptions :column="wideColumns" border>
            <el-descriptions-item label="ETag" :span="2"><code>{{ configDocument.etag || '-' }}</code></el-descriptions-item>
            <el-descriptions-item :label="T('Generation')">{{ configDocument.generation ?? '-' }}</el-descriptions-item>
            <el-descriptions-item :label="T('SchemaDigest')" :span="3"><code>{{ schemaBundle.digest || '-' }}</code></el-descriptions-item>
          </el-descriptions>

          <el-tabs v-model="editorMode" class="editor-tabs" @tab-change="changeEditorMode">
            <el-tab-pane :label="T('YAMLEditor')" name="yaml">
              <el-input v-model="candidateDocument" type="textarea" :rows="24" spellcheck="false" />
            </el-tab-pane>
            <el-tab-pane :label="T('SchemaForm')" name="form">
              <el-alert v-if="formParseError" :title="formParseError" type="error" :closable="false" />
              <el-form v-if="schemaBundle.schema && !formParseError" label-position="top">
                <SchemaField
                  :schema="schemaBundle.schema"
                  :root-schema="schemaBundle.schema"
                  :model-value="formModel"
                  @update:model-value="updateFormModel"
                />
              </el-form>
            </el-tab-pane>
          </el-tabs>
        </el-card>

        <el-card v-if="validationResult" class="section" shadow="never">
          <template #header>
            {{ T('Validation') }}
            <StatusValue :value="validationResult.valid" :text="validationResult.valid ? T('Valid') : T('Invalid')"/>
          </template>
          <el-table :data="validationResult.diagnostics || []" border>
            <el-table-column prop="severity" :label="T('Severity')" width="100" />
            <el-table-column prop="code" :label="T('Code')" min-width="180" />
            <el-table-column prop="pointer" :label="T('Pointer')" min-width="180" />
            <el-table-column prop="message" :label="T('Message')" min-width="260" />
          </el-table>
        </el-card>

        <el-card v-if="plan" class="section plan" shadow="never">
          <template #header>
            <div class="card-header">
              <span>{{ T('ExplicitApplyPlan') }}</span>
              <el-tag :type="riskTagType">{{ T('Risk') }}: {{ plan.impact?.risk || T('Unknown') }}</el-tag>
            </div>
          </template>
          <el-descriptions :column="detailColumns" border>
            <el-descriptions-item :label="T('PlanID')"><code>{{ plan.plan_id }}</code></el-descriptions-item>
            <el-descriptions-item :label="T('Expires')">{{ formatTime(plan.expires_at) }}</el-descriptions-item>
            <el-descriptions-item :label="T('BaseGeneration')">{{ plan.base_generation }}</el-descriptions-item>
            <el-descriptions-item :label="T('RestartRequired')">{{ plan.impact?.restart_required }}</el-descriptions-item>
            <el-descriptions-item :label="T('CandidateDigest')" :span="2"><code>{{ plan.candidate_digest }}</code></el-descriptions-item>
          </el-descriptions>
          <el-table :data="plan.changes || []" border class="section">
            <el-table-column prop="pointer" :label="T('Pointer')" min-width="240" />
            <el-table-column prop="kind" :label="T('Change')" width="120" />
            <el-table-column :label="T('Details')" min-width="280"><template #default="{ row }"><code>{{ compactJSON(row) }}</code></template></el-table-column>
          </el-table>
          <el-input v-model="applyComment" maxlength="256" show-word-limit :placeholder="T('OperatorContextPlaceholder')" />
          <el-button class="apply-button" type="danger" :disabled="!canWrite" :loading="loading.operation" @click="applyReviewedPlan">
            {{ T('ApplyExactPlan') }}
          </el-button>
        </el-card>

        <el-card v-if="operation" class="section" shadow="never">
          <template #header>{{ T('Operation') }} {{ operation.id }}</template>
          <el-descriptions :column="detailColumns" border>
            <el-descriptions-item :label="T('Kind')">{{ operation.kind }}</el-descriptions-item>
            <el-descriptions-item :label="T('State')"><StatusValue :value="operation.state"/></el-descriptions-item>
            <el-descriptions-item :label="T('Generation')">{{ operation.activation_ack?.generation ?? '-' }}</el-descriptions-item>
            <el-descriptions-item :label="T('Error')">{{ operation.error?.detail || '-' }}</el-descriptions-item>
          </el-descriptions>
        </el-card>

        <el-card class="section" shadow="never" v-loading="loading.history">
          <template #header>{{ T('RevisionHistory') }}</template>
          <el-table :data="history" border>
            <el-table-column prop="generation" :label="T('Generation')" width="110" />
            <el-table-column prop="created_at" :label="T('Created')" min-width="180"><template #default="{ row }">{{ formatTime(row.created_at) }}</template></el-table-column>
            <el-table-column prop="actor" :label="T('Actor')" min-width="130" />
            <el-table-column prop="comment" :label="T('Comment')" min-width="220" />
            <el-table-column :label="T('Result')" width="130"><template #default="{ row }"><StatusValue :value="row.result"/></template></el-table-column>
            <el-table-column :label="T('Action')" width="120">
              <template #default="{ row }"><el-button type="warning" plain :disabled="!canWrite" @click="rollback(row)">{{ T('Rollback') }}</el-button></template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-tab-pane>

      <el-tab-pane :label="T('Audit')" name="audit">
        <el-card shadow="never" v-loading="loading.audit">
          <template #header><div class="card-header"><span>{{ T('ControlPlaneAudit') }}</span><div class="audit-filter"><DateRangeFilter v-model="audit.date_range"/><el-button @click="loadAudit">{{ T('Filter') }}</el-button></div></div></template>
          <el-table :data="audit.list || []" border>
            <el-table-column prop="created_at" :label="T('Time')" min-width="180" />
            <el-table-column prop="actor_user_id" :label="T('Actor')" width="90" />
            <el-table-column prop="action" :label="T('Action')" min-width="230" />
            <el-table-column prop="target_id" :label="T('Instance')" min-width="180" />
            <el-table-column :label="T('Result')" width="130"><template #default="{ row }"><StatusValue :value="row.result"/></template></el-table-column>
            <el-table-column prop="error_code" :label="T('Error')" min-width="160" />
            <el-table-column prop="request_id" :label="T('RequestID')" min-width="280" />
          </el-table>
          <el-pagination
            v-model:current-page="audit.page"
            v-model:page-size="audit.page_size"
            :total="audit.total || 0"
            :page-sizes="[25, 50, 100]"
            layout="prev, pager, next, sizes, total"
            @current-change="loadAudit"
            @size-change="loadAudit"
          />
        </el-card>
      </el-tab-pane>

      <el-tab-pane :label="T('Logs')" name="logs">
        <el-card shadow="never" v-loading="loading.logs">
          <template #header><div class="card-header"><span>{{ T('DiagnosticLogs') }}</span><div class="log-actions"><el-button @click="loadLogs">{{ T('Refresh') }}</el-button><el-button :disabled="!logResult.entries.length" @click="exportLogs">{{ T('Export') }}</el-button></div></div></template>
          <el-alert v-if="!logSources.length" class="log-deployment-alert" :title="T('LogSourcesNotConfigured')" :description="T('LogSourcesNotConfiguredDescription')" type="warning" :closable="false" show-icon/>
          <el-alert v-else-if="unavailableLogSources.length" class="log-deployment-alert" :title="T('LogSourcesUnavailable')" :description="T('LogSourcesUnavailableDescription', { sources: unavailableLogSources.map(source => source.label).join(', ') })" type="warning" :closable="false" show-icon/>
          <div class="log-toolbar">
            <el-select v-model="selectedLogSource" :placeholder="T('SelectLogSource')" @change="loadLogs">
              <el-option v-for="source in logSources" :key="source.id" :value="source.id" :label="`${source.label} · ${source.component}`"><template #default><span>{{ source.label }}</span><StatusValue :value="source.available" class="log-source-state"/></template></el-option>
            </el-select>
            <el-select v-model="logLimit" style="width: 130px" @change="loadLogs"><el-option :label="'200 ' + T('Lines')" :value="200"/><el-option :label="'400 ' + T('Lines')" :value="400"/><el-option :label="'1000 ' + T('Lines')" :value="1000"/><el-option :label="'2000 ' + T('Lines')" :value="2000"/></el-select>
            <el-input v-model="logFilter" clearable :placeholder="T('FilterLogs')"/>
            <template v-if="activeLogSource?.level_mutable">
              <el-select v-model="logLevel" :disabled="selectedInstance?.read_only" style="width: 130px" @change="changeLogLevel"><el-option v-for="level in logLevels" :key="level" :label="level.toUpperCase()" :value="level"/></el-select>
            </template>
            <el-tooltip v-else :content="T('StarryRuntimeLevelUnavailable')"><el-tag type="info" effect="plain">{{ T('DeploymentManagedLevel') }}</el-tag></el-tooltip>
          </div>
          <el-alert v-if="logResult.truncated" :title="T('LogsTruncated')" type="info" :closable="false" show-icon/>
          <div class="log-viewer" role="log" aria-live="polite">
            <div v-for="entry in filteredLogEntries" :key="entry.sequence" class="log-line"><span>{{ String(entry.sequence).padStart(4, '0') }}</span><b :class="`log-level--${entry.level}`">{{ entry.level }}</b><code>{{ entry.text }}</code></div>
            <el-empty v-if="!filteredLogEntries.length" :description="T('NoLogsAvailable')"/>
          </div>
        </el-card>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { parseDocument, stringify as stringifyYAML } from 'yaml'
import SchemaField from '@/components/schema/SchemaField.vue'
import InfoLabel from '@/components/common/InfoLabel.vue'
import StatusValue from '@/components/common/StatusValue.vue'
import StarryLogo from '@/components/brand/StarryLogo.vue'
import DateRangeFilter from '@/components/common/DateRangeFilter.vue'
import { useAppStore } from '@/store/app'
import { T } from '@/utils/i18n'
import { downBlob } from '@/utils/file'
import { withDateRange } from '@/utils/dateRange'
import {
  applyConfig,
  getCapabilities,
  getConfig,
  getConfigHistory,
  getConfigSchema,
  getOperation,
  getRelays,
  getLogSources,
  getLogs,
  getStatus,
  listAuditEvents,
  listInstances,
  newIdempotencyKey,
  planConfig,
  reloadRuntime,
  rollbackConfig,
  simulateAllocation,
  setLogLevel,
  validateConfig,
} from '@/api/server_control'

const instances = ref([])
const appStore = useAppStore()
const selectedID = ref('')
const capabilities = ref({})
const status = ref({})
const relays = ref({})
const simulationResult = ref(null)
const configDocument = ref({})
const schemaBundle = ref({})
const candidateDocument = ref('')
const formModel = ref({})
const formParseError = ref('')
const validationResult = ref(null)
const plan = ref(null)
const operation = ref(null)
const history = ref([])
const applyComment = ref('')
const editorMode = ref('yaml')
const activeTab = ref('status')
const simulationInput = reactive({ clientA: '', clientB: '', transport: 'native' })
const audit = reactive({ list: [], page: 1, page_size: 50, total: 0, date_range: [] })
const logSources = ref([])
const selectedLogSource = ref('')
const logLimit = ref(400)
const logFilter = ref('')
const logLevel = ref('info')
const logLevels = ['error', 'warn', 'info', 'debug', 'trace']
const logResult = reactive({ source: {}, entries: [], truncated: false })
const loading = reactive({
  instances: false,
  capabilities: false,
  status: false,
  relays: false,
  simulation: false,
  config: false,
  validation: false,
  plan: false,
  operation: false,
  history: false,
  audit: false,
  logs: false,
})

const selectedInstance = computed(() => instances.value.find(item => item.id === selectedID.value))
const canWrite = computed(() => Boolean(
  selectedInstance.value?.enabled &&
  selectedInstance.value?.available &&
  !selectedInstance.value?.read_only &&
  capabilities.value.capabilities?.config_transaction,
))
const canPlan = computed(() => Boolean(selectedInstance.value?.available && capabilities.value.capabilities?.config_transaction))
const riskTagType = computed(() => ({ high: 'danger', medium: 'warning', low: 'success' }[plan.value?.impact?.risk] || 'info'))
const detailColumns = computed(() => appStore.setting.viewportWidth < 720 ? 1 : 2)
const metricColumns = computed(() => appStore.setting.viewportWidth < 720 ? 1 : appStore.setting.viewportWidth < 1100 ? 2 : 4)
const wideColumns = computed(() => appStore.setting.viewportWidth < 720 ? 1 : 3)
const activeLogSource = computed(() => logSources.value.find(item => item.id === selectedLogSource.value))
const unavailableLogSources = computed(() => logSources.value.filter(item => !item.available))
const filteredLogEntries = computed(() => {
  const needle = logFilter.value.trim().toLowerCase()
  return needle ? logResult.entries.filter(item => item.text.toLowerCase().includes(needle) || item.level.includes(needle)) : logResult.entries
})

async function withLoading (name, action) {
  loading[name] = true
  try {
    return await action()
  } finally {
    loading[name] = false
  }
}

async function loadInstances () {
  await withLoading('instances', async () => {
    const response = await listInstances()
    instances.value = response.data || []
    if (!instances.value.some(item => item.id === selectedID.value && item.enabled && item.available)) {
      selectedID.value = instances.value.find(item => item.enabled && item.available)?.id || ''
    }
  })
  if (selectedID.value) await loadSelected()
}

async function loadSelected () {
  plan.value = null
  validationResult.value = null
  operation.value = null
  await Promise.allSettled([loadCapabilities(), loadStatus(), loadRelays(), loadConfig(), loadSchema(), loadHistory(), loadAudit(), loadLogSources()])
}

const loadCapabilities = () => withLoading('capabilities', async () => { capabilities.value = (await getCapabilities(selectedID.value)).data })
const loadStatus = () => withLoading('status', async () => { status.value = (await getStatus(selectedID.value)).data })
const loadRelays = () => withLoading('relays', async () => { relays.value = (await getRelays(selectedID.value)).data })
const loadSchema = () => withLoading('config', async () => { schemaBundle.value = (await getConfigSchema(selectedID.value)).data })
const loadHistory = () => withLoading('history', async () => { history.value = (await getConfigHistory(selectedID.value)).data || [] })

async function loadConfig () {
  await withLoading('config', async () => {
    configDocument.value = (await getConfig(selectedID.value)).data
    candidateDocument.value = configDocument.value.document || ''
    plan.value = null
    validationResult.value = null
    parseCandidateForForm()
  })
}

async function loadAudit () {
  await withLoading('audit', async () => {
    const data = (await listAuditEvents(audit.page, audit.page_size, withDateRange({ date_range: audit.date_range }))).data || {}
    audit.list = data.list || []
    audit.total = data.total || 0
    audit.page = data.page || audit.page
    audit.page_size = data.page_size || audit.page_size
  })
}

async function loadLogSources () {
  await withLoading('logs', async () => {
    logSources.value = (await getLogSources(selectedID.value)).data || []
    if (!logSources.value.some(item => item.id === selectedLogSource.value)) selectedLogSource.value = logSources.value[0]?.id || ''
    const source = activeLogSource.value
    if (source?.current_level) logLevel.value = source.current_level
  })
  if (selectedLogSource.value) await loadLogs()
}

async function loadLogs () {
  if (!selectedID.value || !selectedLogSource.value) return
  await withLoading('logs', async () => {
    const data = (await getLogs(selectedID.value, selectedLogSource.value, logLimit.value)).data || {}
    logResult.source = data.source || {}; logResult.entries = data.entries || []; logResult.truncated = Boolean(data.truncated)
    if (data.source?.current_level) logLevel.value = data.source.current_level
  })
}

async function changeLogLevel () {
  if (!activeLogSource.value?.level_mutable) return
  const data = (await setLogLevel(selectedID.value, selectedLogSource.value, logLevel.value)).data
  if (data?.current_level) logLevel.value = data.current_level
  ElMessage.success(T('LogLevelUpdated'))
}

function exportLogs () {
  const text = logResult.entries.map(entry => entry.text).join('\n') + '\n'
  downBlob(new Blob([text], { type: 'text/plain;charset=utf-8' }), `${selectedLogSource.value}-${new Date().toISOString().replaceAll(':', '-')}.log`)
}

function parseCandidateForForm () {
  try {
    const parsed = parseDocument(candidateDocument.value, { prettyErrors: true, uniqueKeys: true })
    if (parsed.errors.length) throw parsed.errors[0]
    const value = parsed.toJS()
    if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error(T('ConfigurationRootMapping'))
    formModel.value = value
    formParseError.value = ''
    return true
  } catch (error) {
    formParseError.value = T('CannotOpenSchemaForm', { message: error.message })
    return false
  }
}

function changeEditorMode (name) {
  if (name === 'form' && !parseCandidateForForm()) editorMode.value = 'yaml'
}

function updateFormModel (value) {
  formModel.value = value
}

watch(formModel, value => {
  if (editorMode.value === 'form' && !formParseError.value) {
    candidateDocument.value = stringifyYAML(value, { lineWidth: 0 })
    plan.value = null
    validationResult.value = null
  }
}, { deep: true })

watch(candidateDocument, () => {
  if (editorMode.value === 'yaml') {
    plan.value = null
    validationResult.value = null
  }
})

async function validateCandidate () {
  await withLoading('validation', async () => {
    validationResult.value = (await validateConfig(selectedID.value, candidateDocument.value)).data
    if (validationResult.value.valid) ElMessage.success(T('ValidationPassed'))
  })
  return validationResult.value?.valid === true
}

async function createPlan () {
  if (!canPlan.value) {
    ElMessage.warning(T('ControlModeReadOnlyDescription'))
    return
  }
  const valid = await validateCandidate()
  if (!valid) {
    ElMessage.warning(T('FixValidationBeforePlanning'))
    return
  }
  await withLoading('plan', async () => {
    plan.value = (await planConfig(selectedID.value, candidateDocument.value, configDocument.value.etag)).data
  })
}

async function applyReviewedPlan () {
  if (!plan.value || Date.parse(plan.value.expires_at) <= Date.now()) {
    ElMessage.error(T('PlanMissingOrExpired'))
    return
  }
  const highRisk = plan.value.impact?.risk === 'high'
  const confirmed = await ElMessageBox.confirm(
    T('ConfirmApplyPlan', { risk: highRisk ? T('HighRiskPrefix') : '', id: plan.value.plan_id, count: (plan.value.changes || []).length }),
    T('ConfirmExactPlanApply'),
    { type: highRisk ? 'error' : 'warning', confirmButtonText: T('ApplyExactPlan'), cancelButtonText: T('Cancel') },
  ).catch(() => false)
  if (!confirmed) return

  await withLoading('operation', async () => {
    operation.value = (await applyConfig(
      selectedID.value,
      plan.value,
      configDocument.value.etag,
      newIdempotencyKey(),
      applyComment.value,
    )).data
    await pollOperation(operation.value.id)
  })
}

async function rollback (revision) {
  const confirmed = await ElMessageBox.confirm(
    T('ConfirmRollbackRevision', { id: revision.id, generation: revision.generation }),
    T('ConfirmGuardedRollback'),
    { type: 'error', confirmButtonText: T('Rollback'), cancelButtonText: T('Cancel') },
  ).catch(() => false)
  if (!confirmed) return

  const comment = T('RollbackToGeneration', { generation: revision.generation })
  await withLoading('operation', async () => {
    operation.value = (await rollbackConfig(
      selectedID.value,
      revision.id,
      configDocument.value.etag,
      newIdempotencyKey(),
      comment,
    )).data
    await pollOperation(operation.value.id)
  })
}

async function pollOperation (operationID) {
  const terminal = new Set(['succeeded', 'failed'])
  for (let attempt = 0; attempt < 60 && !terminal.has(operation.value?.state); attempt += 1) {
    await new Promise(resolve => setTimeout(resolve, 1000))
    operation.value = (await getOperation(selectedID.value, operationID)).data
  }
  if (operation.value?.state === 'succeeded') {
    ElMessage.success(T('OperationSucceededAcknowledged'))
    plan.value = null
    await Promise.allSettled([loadConfig(), loadStatus(), loadRelays(), loadHistory(), loadAudit()])
  } else if (operation.value?.state === 'failed') {
    ElMessage.error(operation.value.error?.detail || T('OperationFailed'))
  } else {
    ElMessage.warning(T('OperationNonTerminal'))
  }
}

async function reloadActiveRuntime () {
  const confirmed = await ElMessageBox.confirm(
    T('ConfirmRuntimeReload', { digest: configDocument.value.source_digest }),
    T('ConfirmAuditedRuntimeReload'),
    { type: 'warning' },
  ).catch(() => false)
  if (!confirmed) return
  await withLoading('operation', async () => {
    const ack = (await reloadRuntime(selectedID.value, configDocument.value.source_digest, newIdempotencyKey())).data
    ElMessage.success(T('RuntimeGenerationAcknowledged', { generation: ack.generation }))
    await Promise.allSettled([loadConfig(), loadStatus(), loadAudit()])
  })
}

async function simulate () {
  if (!simulationInput.clientA || !simulationInput.clientB) {
    ElMessage.warning(T('BothClientIPsRequired'))
    return
  }
  await withLoading('simulation', async () => {
    simulationResult.value = (await simulateAllocation(selectedID.value, {
      client_a: { ip: simulationInput.clientA },
      client_b: { ip: simulationInput.clientB },
      transport: simulationInput.transport,
      explain: true,
      expected_config_generation: relays.value.config_generation,
    })).data
  })
}

const compactJSON = value => JSON.stringify(value)
const formatTime = value => value ? new Date(value).toLocaleString() : '-'
const formatMetricName = name => String(name).replaceAll('_', ' ').replace(/\b\w/g, value => value.toUpperCase())
const metricHelp = name => {
  const normalized = String(name).toLowerCase().replaceAll('-', '_')
  const keys = {
    attempts: 'AuthMetricAttemptsHelp', allowed: 'AuthMetricAllowedHelp', denied: 'AuthMetricDeniedHelp',
    audit_would_deny: 'AuthMetricAuditWouldDenyHelp', cache_hits: 'AuthMetricCacheHitsHelp',
    introspection_requests: 'AuthMetricIntrospectionRequestsHelp', introspection_failures: 'AuthMetricIntrospectionFailuresHelp',
  }
  return T(keys[normalized] || 'AuthenticationMetricHelp', { name: formatMetricName(name) })
}

onMounted(loadInstances)
</script>

<style scoped lang="scss">
.page-heading__starry { display: flex; min-width: 0; align-items: center; gap: 20px; }
.page-heading__starry > div { min-width: 0; }
.page-heading__starry-logo { width: 308px !important; max-width: min(308px, 36vw); padding-right: 24px; border-right: 1px solid var(--border-subtle); }
.policy-alert { margin-bottom: 14px; border: 1px solid var(--primary-border); border-radius: 14px; background: color-mix(in srgb, var(--primary-soft) 72%, var(--surface-1)); }
.toolbar { margin-bottom: 16px; }
.toolbar :deep(.el-card__body) { padding: 14px 17px; }
.instance-toolbar { display: flex; min-width: 0; align-items: center; gap: 12px; }
.instance-toolbar__copy { display: flex; min-width: 150px; flex: 1; flex-direction: column; }
.instance-toolbar__copy small { color: var(--text-tertiary); font-size: 10px; }
.instance-toolbar__copy strong { margin-top: 2px; color: var(--text-primary); font-size: 13px; }
.control-tabs :deep(.el-tabs__header) { margin: 0 0 16px; padding: 5px 8px; border: 1px solid var(--border-subtle); border-radius: 14px; background: var(--surface-1); box-shadow: var(--shadow-sm); }
.control-tabs :deep(.el-tabs__nav-wrap::after) { display: none; }
.control-tabs :deep(.el-tabs__nav-scroll) { padding-left: 12px; }
.control-tabs :deep(.el-tabs__item) { height: 38px; padding: 0 16px; color: var(--text-secondary); font-size: 12px; font-weight: 700; }
.control-tabs :deep(.el-tabs__item.is-active) { color: var(--primary); }
.control-tabs :deep(.el-tabs__active-bar) { height: 3px; border-radius: 999px; background: var(--primary); }
.audit-filter { display: flex; min-width: 0; align-items: center; gap: 8px; }
.status-card { height: 100%; }
.section { margin-top: 14px; }
.card-header { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.card-header > span { color: var(--text-primary); font-size: 13px; font-weight: 740; }
.editor-tabs { margin-top: 16px; }
.control-mode-guide { margin: 12px 0; padding: 0 15px; border: 1px solid var(--border-subtle); border-radius: 14px; background: var(--surface-1); }
.control-mode-guide__title { display: flex; width: 100%; align-items: center; justify-content: space-between; gap: 12px; padding-right: 12px; font-size: 12px; font-weight: 720; }
.control-mode-guide__content { padding: 0 2px 15px; color: var(--text-secondary); font-size: 12px; line-height: 1.7; }.control-mode-guide__content ol { margin: 0 0 14px; padding-left: 22px; }.control-mode-guide__content li + li { margin-top: 6px; }
.apply-button { width: 100%; margin-top: 16px; }
code { overflow-wrap: anywhere; color: var(--text-secondary); font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; font-size: 10px; white-space: normal; }
:deep(.el-descriptions__label) { color: var(--text-tertiary) !important; font-size: 10px; font-weight: 700; }
:deep(.el-descriptions__content) { color: var(--text-primary) !important; font-size: 11px; }
:deep(.el-textarea__inner) { padding: 14px; border-radius: 12px; background: #171b24; color: #dfe5f1; font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; font-size: 11px; line-height: 1.65; tab-size: 2; }
:deep(.el-pagination) { margin-top: 14px; }
.log-actions,.log-toolbar { display: flex; align-items: center; gap: 9px; }.log-toolbar { margin-bottom: 12px; flex-wrap: wrap; }.log-toolbar > .el-select:first-child { width: min(320px, 100%); }.log-toolbar > .el-input { min-width: 200px; flex: 1; }.log-source-state { float: right; margin-left: 18px; }
.log-deployment-alert { margin-bottom: 12px; }
.log-viewer { min-height: 320px; max-height: 620px; overflow: auto; padding: 12px 0; border: 1px solid #282e3a; border-radius: 14px; background: #11151c; color: #dce2ec; font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; }
.log-line { display: grid; min-width: 760px; gap: 10px; padding: 3px 12px; grid-template-columns: 42px 48px minmax(0,1fr); font-size: 11px; line-height: 1.55; }.log-line:hover { background: rgba(255,255,255,.035); }.log-line > span { color: #667084; text-align: right; }.log-line > b { color: #8da0bd; font-size: 9px; text-transform: uppercase; }.log-line code { color: inherit; font-size: inherit; white-space: pre-wrap; }.log-level--error,.log-level--fatal,.log-level--panic { color: #ff7f8a !important; }.log-level--warn { color: #e9c95c !important; }.log-level--debug,.log-level--trace { color: #9d89e8 !important; }

@media (max-width: 767px) {
  .page-heading__starry { align-items: flex-start; flex-direction: column; gap: 12px; }
  .page-heading__starry-logo { width: min(248px, 72vw) !important; max-width: 72vw; padding: 0; border: 0; }
  .policy-alert :deep(.el-alert__description) { font-size: 10px; line-height: 1.55; }
  .instance-toolbar { align-items: stretch; flex-direction: column; }
  .instance-toolbar .el-select { width: 100% !important; }
  .control-tabs :deep(.el-tabs__header) { overflow: hidden; padding-inline: 5px; }
  .control-tabs :deep(.el-tabs__nav-wrap) { overflow-x: auto; scrollbar-width: none; }
  .control-tabs :deep(.el-tabs__nav-scroll) { overflow: visible; padding-left: 10px; }
  .control-tabs :deep(.el-tabs__nav) { float: none; width: max-content; }
  .control-tabs :deep(.el-tabs__item) { padding: 0 13px; }
  .control-tabs :deep(.el-row) { row-gap: 12px; }
  .control-tabs :deep(.el-card__header) { padding: 15px; }
  .control-tabs :deep(.el-card__body) { overflow-x: auto; padding: 13px; }
  .control-tabs :deep(.el-table) { min-width: 720px; }
  .card-header { align-items: stretch; flex-direction: column; }
  .card-header :deep(.el-space) { display: flex; width: 100%; flex-wrap: wrap; }
  .card-header :deep(.el-space__item) { flex: 1 1 calc(50% - 8px); }
  .card-header :deep(.el-button) { width: 100%; margin: 0; }
  .control-tabs :deep(.el-form--inline) { display: grid; grid-template-columns: 1fr; }
  .control-tabs :deep(.el-form--inline .el-form-item) { width: 100%; margin-right: 0; }
  .control-tabs :deep(.el-form--inline .el-input),.control-tabs :deep(.el-form--inline .el-select) { width: 100% !important; }
  :deep(.el-textarea__inner) { min-height: 430px !important; }
  .log-toolbar > * { width: 100% !important; }.log-actions { width: 100%; }.log-actions .el-button { flex: 1; margin: 0; }
}
</style>
