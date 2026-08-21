<template>
  <div class="server-control">
    <el-alert
      title="Typed Starry Control API"
      description="This page sends only versioned DTOs to deployment-owned instances. It has no shell, arbitrary command, URL, path, or Docker-socket input."
      type="info"
      :closable="false"
      show-icon
    />

    <el-card class="toolbar" shadow="never">
      <el-space wrap>
        <el-select v-model="selectedID" placeholder="Select a Starry instance" style="width: 360px" @change="loadSelected">
          <el-option
            v-for="item in instances"
            :key="item.id"
            :label="`${item.name} (${item.id})`"
            :value="item.id"
            :disabled="!item.enabled || !item.available"
          />
        </el-select>
        <el-button :loading="loading.instances" @click="loadInstances">Refresh instances</el-button>
        <el-tag v-if="selectedInstance" :type="selectedInstance.available ? 'success' : 'danger'">
          {{ selectedInstance.available ? 'available' : selectedInstance.error_code || 'unavailable' }}
        </el-tag>
        <el-tag v-if="selectedInstance?.read_only" type="warning">read only</el-tag>
      </el-space>
    </el-card>

    <el-empty v-if="!selectedID" description="No available Starry instance is configured" />

    <el-tabs v-else v-model="activeTab" class="control-tabs">
      <el-tab-pane label="Status" name="status">
        <el-row :gutter="16">
          <el-col :xs="24" :lg="12">
            <el-card shadow="never" v-loading="loading.status">
              <template #header>Runtime</template>
              <el-descriptions :column="2" border>
                <el-descriptions-item label="Ready">{{ status.ready }}</el-descriptions-item>
                <el-descriptions-item label="State">{{ status.config?.status || '-' }}</el-descriptions-item>
                <el-descriptions-item label="Generation">{{ status.config?.generation ?? '-' }}</el-descriptions-item>
                <el-descriptions-item label="Schema">{{ status.config?.schema_version ?? '-' }}</el-descriptions-item>
                <el-descriptions-item label="Source digest" :span="2"><code>{{ status.config?.source_digest || '-' }}</code></el-descriptions-item>
                <el-descriptions-item label="Last error" :span="2">{{ status.config?.last_error || '-' }}</el-descriptions-item>
              </el-descriptions>
            </el-card>
          </el-col>
          <el-col :xs="24" :lg="12">
            <el-card shadow="never" v-loading="loading.capabilities">
              <template #header>Contract and authentication</template>
              <el-descriptions :column="2" border>
                <el-descriptions-item label="Protocol">{{ capabilities.protocol?.version || '-' }}</el-descriptions-item>
                <el-descriptions-item label="Starry">{{ capabilities.instance?.starry_version || '-' }}</el-descriptions-item>
                <el-descriptions-item label="Configured auth">{{ status.auth?.configured_mode || '-' }}</el-descriptions-item>
                <el-descriptions-item label="Effective auth">{{ status.auth?.effective_mode || '-' }}</el-descriptions-item>
                <el-descriptions-item label="Verifier">{{ status.auth?.verifier_state || '-' }}</el-descriptions-item>
                <el-descriptions-item label="Keys">{{ status.auth?.key_count ?? '-' }}</el-descriptions-item>
                <el-descriptions-item label="Schema digest" :span="2"><code>{{ capabilities.config?.schema_digest || '-' }}</code></el-descriptions-item>
              </el-descriptions>
            </el-card>
          </el-col>
        </el-row>
        <el-card class="section" shadow="never">
          <template #header>Authentication metrics</template>
          <el-descriptions :column="4" border>
            <el-descriptions-item v-for="(value, name) in status.auth?.metrics || {}" :key="name" :label="name">
              {{ value }}
            </el-descriptions-item>
          </el-descriptions>
        </el-card>
      </el-tab-pane>

      <el-tab-pane label="Relays and simulation" name="relays">
        <el-card shadow="never" v-loading="loading.relays">
          <template #header>
            <div class="card-header">
              <span>Relay inventory · generation {{ relays.config_generation ?? '-' }}</span>
              <el-button @click="loadRelays">Refresh</el-button>
            </div>
          </template>
          <el-alert v-if="relays.warning" :title="relays.warning" type="warning" :closable="false" />
          <el-table :data="relays.relays || []" border>
            <el-table-column prop="id" label="Relay" min-width="220" />
            <el-table-column prop="configured_order" label="Order" width="90" />
            <el-table-column prop="native.state" label="Native" width="110" />
            <el-table-column prop="websocket.state" label="WSS" width="110" />
            <el-table-column prop="websocket.latency_ms" label="WSS latency (ms)" width="150" />
            <el-table-column label="Eligible for" min-width="180">
              <template #default="{ row }">{{ (row.eligible_for || []).join(', ') || '-' }}</template>
            </el-table-column>
            <el-table-column label="Rules" min-width="180">
              <template #default="{ row }">{{ (row.referenced_by_rules || []).join(', ') || '-' }}</template>
            </el-table-column>
          </el-table>
        </el-card>

        <el-card class="section" shadow="never" v-loading="loading.simulation">
          <template #header>Side-effect-free allocation simulation</template>
          <el-form inline :model="simulationInput">
            <el-form-item label="Client A IP"><el-input v-model="simulationInput.clientA" placeholder="203.0.113.10" /></el-form-item>
            <el-form-item label="Client B IP"><el-input v-model="simulationInput.clientB" placeholder="2001:db8::20" /></el-form-item>
            <el-form-item label="Transport">
              <el-select v-model="simulationInput.transport" style="width: 130px">
                <el-option label="native" value="native" />
                <el-option label="wss" value="wss" />
                <el-option label="mixed" value="mixed" />
              </el-select>
            </el-form-item>
            <el-form-item><el-button type="primary" @click="simulate">Simulate</el-button></el-form-item>
          </el-form>
          <template v-if="simulationResult">
            <el-descriptions :column="3" border>
              <el-descriptions-item label="Rule">{{ simulationResult.matched_rule?.name || 'official fallback' }}</el-descriptions-item>
              <el-descriptions-item label="Selection">{{ simulationResult.selection?.kind }}</el-descriptions-item>
              <el-descriptions-item label="Relay">{{ simulationResult.selection?.relay_id || '-' }}</el-descriptions-item>
            </el-descriptions>
            <el-table :data="simulationResult.candidates || []" border class="section">
              <el-table-column prop="relay_id" label="Candidate" min-width="220" />
              <el-table-column prop="eligible" label="Eligible" width="100" />
              <el-table-column prop="priority" label="Priority" width="100" />
              <el-table-column prop="exclusion_reason" label="Exclusion reason" min-width="220" />
            </el-table>
          </template>
        </el-card>
      </el-tab-pane>

      <el-tab-pane label="Configuration" name="config">
        <el-alert v-if="configDocument.drift" title="Disk/runtime drift detected. Reconcile before applying another change." type="error" :closable="false" />
        <el-alert v-if="!canWrite" title="This instance is read-only. Validation and planning remain available; mutations are disabled." type="warning" :closable="false" />
        <el-card shadow="never" v-loading="loading.config">
          <template #header>
            <div class="card-header">
              <span>Managed YAML</span>
              <el-space>
                <el-button @click="loadConfig">Discard edits / reload</el-button>
                <el-button :loading="loading.validation" @click="validateCandidate">Validate</el-button>
                <el-button type="primary" :loading="loading.plan" @click="createPlan">Create review plan</el-button>
                <el-button type="warning" :disabled="!canWrite || !configDocument.source_digest" @click="reloadActiveRuntime">Reload runtime</el-button>
              </el-space>
            </div>
          </template>
          <el-descriptions :column="3" border>
            <el-descriptions-item label="ETag" :span="2"><code>{{ configDocument.etag || '-' }}</code></el-descriptions-item>
            <el-descriptions-item label="Generation">{{ configDocument.generation ?? '-' }}</el-descriptions-item>
            <el-descriptions-item label="Schema digest" :span="3"><code>{{ schemaBundle.digest || '-' }}</code></el-descriptions-item>
          </el-descriptions>

          <el-tabs v-model="editorMode" class="editor-tabs" @tab-change="changeEditorMode">
            <el-tab-pane label="YAML editor" name="yaml">
              <el-input v-model="candidateDocument" type="textarea" :rows="24" spellcheck="false" />
            </el-tab-pane>
            <el-tab-pane label="Schema form" name="form">
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
            Validation
            <el-tag :type="validationResult.valid ? 'success' : 'danger'">{{ validationResult.valid ? 'valid' : 'invalid' }}</el-tag>
          </template>
          <el-table :data="validationResult.diagnostics || []" border>
            <el-table-column prop="severity" label="Severity" width="100" />
            <el-table-column prop="code" label="Code" min-width="180" />
            <el-table-column prop="pointer" label="Pointer" min-width="180" />
            <el-table-column prop="message" label="Message" min-width="260" />
          </el-table>
        </el-card>

        <el-card v-if="plan" class="section plan" shadow="never">
          <template #header>
            <div class="card-header">
              <span>Explicit apply plan</span>
              <el-tag :type="riskTagType">risk: {{ plan.impact?.risk || 'unknown' }}</el-tag>
            </div>
          </template>
          <el-descriptions :column="2" border>
            <el-descriptions-item label="Plan ID"><code>{{ plan.plan_id }}</code></el-descriptions-item>
            <el-descriptions-item label="Expires">{{ formatTime(plan.expires_at) }}</el-descriptions-item>
            <el-descriptions-item label="Base generation">{{ plan.base_generation }}</el-descriptions-item>
            <el-descriptions-item label="Restart required">{{ plan.impact?.restart_required }}</el-descriptions-item>
            <el-descriptions-item label="Candidate digest" :span="2"><code>{{ plan.candidate_digest }}</code></el-descriptions-item>
          </el-descriptions>
          <el-table :data="plan.changes || []" border class="section">
            <el-table-column prop="pointer" label="Pointer" min-width="240" />
            <el-table-column prop="kind" label="Change" width="120" />
            <el-table-column label="Details" min-width="280"><template #default="{ row }"><code>{{ compactJSON(row) }}</code></template></el-table-column>
          </el-table>
          <el-input v-model="applyComment" maxlength="256" show-word-limit placeholder="Required operator context is recommended" />
          <el-button class="apply-button" type="danger" :disabled="!canWrite" :loading="loading.operation" @click="applyReviewedPlan">
            Apply this exact reviewed plan
          </el-button>
        </el-card>

        <el-card v-if="operation" class="section" shadow="never">
          <template #header>Operation {{ operation.id }}</template>
          <el-descriptions :column="2" border>
            <el-descriptions-item label="Kind">{{ operation.kind }}</el-descriptions-item>
            <el-descriptions-item label="State"><el-tag :type="operationTagType">{{ operation.state }}</el-tag></el-descriptions-item>
            <el-descriptions-item label="Generation">{{ operation.activation_ack?.generation ?? '-' }}</el-descriptions-item>
            <el-descriptions-item label="Error">{{ operation.error?.detail || '-' }}</el-descriptions-item>
          </el-descriptions>
        </el-card>

        <el-card class="section" shadow="never" v-loading="loading.history">
          <template #header>Revision history</template>
          <el-table :data="history" border>
            <el-table-column prop="generation" label="Generation" width="110" />
            <el-table-column prop="created_at" label="Created" min-width="180"><template #default="{ row }">{{ formatTime(row.created_at) }}</template></el-table-column>
            <el-table-column prop="actor" label="Actor" min-width="130" />
            <el-table-column prop="comment" label="Comment" min-width="220" />
            <el-table-column prop="result" label="Result" width="110" />
            <el-table-column label="Action" width="120">
              <template #default="{ row }"><el-button type="warning" plain :disabled="!canWrite" @click="rollback(row)">Rollback</el-button></template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-tab-pane>

      <el-tab-pane label="Audit" name="audit">
        <el-card shadow="never" v-loading="loading.audit">
          <template #header><div class="card-header"><span>Redacted control-plane audit</span><el-button @click="loadAudit">Refresh</el-button></div></template>
          <el-table :data="audit.list || []" border>
            <el-table-column prop="created_at" label="Time" min-width="180" />
            <el-table-column prop="actor_user_id" label="Actor" width="90" />
            <el-table-column prop="action" label="Action" min-width="230" />
            <el-table-column prop="target_id" label="Instance" min-width="180" />
            <el-table-column prop="result" label="Result" width="100" />
            <el-table-column prop="error_code" label="Error" min-width="160" />
            <el-table-column prop="request_id" label="Request ID" min-width="280" />
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
    </el-tabs>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { parseDocument, stringify as stringifyYAML } from 'yaml'
import SchemaField from '@/components/schema/SchemaField.vue'
import {
  applyConfig,
  getCapabilities,
  getConfig,
  getConfigHistory,
  getConfigSchema,
  getOperation,
  getRelays,
  getStatus,
  listAuditEvents,
  listInstances,
  newIdempotencyKey,
  planConfig,
  reloadRuntime,
  rollbackConfig,
  simulateAllocation,
  validateConfig,
} from '@/api/server_control'

const instances = ref([])
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
const audit = reactive({ list: [], page: 1, page_size: 50, total: 0 })
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
})

const selectedInstance = computed(() => instances.value.find(item => item.id === selectedID.value))
const canWrite = computed(() => Boolean(
  selectedInstance.value?.enabled &&
  selectedInstance.value?.available &&
  !selectedInstance.value?.read_only &&
  capabilities.value.capabilities?.config_transaction,
))
const riskTagType = computed(() => ({ high: 'danger', medium: 'warning', low: 'success' }[plan.value?.impact?.risk] || 'info'))
const operationTagType = computed(() => ({ succeeded: 'success', failed: 'danger', running: 'warning', pending: 'info' }[operation.value?.state] || 'info'))

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
  await Promise.allSettled([loadCapabilities(), loadStatus(), loadRelays(), loadConfig(), loadSchema(), loadHistory(), loadAudit()])
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
    const data = (await listAuditEvents(audit.page, audit.page_size)).data || {}
    audit.list = data.list || []
    audit.total = data.total || 0
    audit.page = data.page || audit.page
    audit.page_size = data.page_size || audit.page_size
  })
}

function parseCandidateForForm () {
  try {
    const parsed = parseDocument(candidateDocument.value, { prettyErrors: true, uniqueKeys: true })
    if (parsed.errors.length) throw parsed.errors[0]
    const value = parsed.toJS()
    if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error('Configuration root must be a mapping')
    formModel.value = value
    formParseError.value = ''
    return true
  } catch (error) {
    formParseError.value = `Cannot open schema form: ${error.message}`
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
    if (validationResult.value.valid) ElMessage.success('Agent-authoritative validation passed')
  })
  return validationResult.value?.valid === true
}

async function createPlan () {
  const valid = await validateCandidate()
  if (!valid) {
    ElMessage.warning('Fix validation diagnostics before planning')
    return
  }
  await withLoading('plan', async () => {
    plan.value = (await planConfig(selectedID.value, candidateDocument.value, configDocument.value.etag)).data
  })
}

async function applyReviewedPlan () {
  if (!plan.value || Date.parse(plan.value.expires_at) <= Date.now()) {
    ElMessage.error('The plan is missing or expired; create a new plan')
    return
  }
  const highRisk = plan.value.impact?.risk === 'high'
  const confirmed = await ElMessageBox.confirm(
    `${highRisk ? 'HIGH-RISK CHANGE. ' : ''}Apply plan ${plan.value.plan_id} with ${(plan.value.changes || []).length} reviewed changes?`,
    'Confirm exact plan apply',
    { type: highRisk ? 'error' : 'warning', confirmButtonText: 'Apply exact plan', cancelButtonText: 'Cancel' },
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
    `Rollback to revision ${revision.id} (generation ${revision.generation})?`,
    'Confirm guarded rollback',
    { type: 'error', confirmButtonText: 'Rollback', cancelButtonText: 'Cancel' },
  ).catch(() => false)
  if (!confirmed) return

  const comment = `Rollback to generation ${revision.generation}`
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
    ElMessage.success('Operation succeeded and runtime acknowledgement was received')
    plan.value = null
    await Promise.allSettled([loadConfig(), loadStatus(), loadRelays(), loadHistory(), loadAudit()])
  } else if (operation.value?.state === 'failed') {
    ElMessage.error(operation.value.error?.detail || 'Operation failed')
  } else {
    ElMessage.warning('Operation remains non-terminal; use its ID to continue investigation')
  }
}

async function reloadActiveRuntime () {
  const confirmed = await ElMessageBox.confirm(
    `Reload runtime only if disk source digest is still ${configDocument.value.source_digest}?`,
    'Confirm audited runtime reload',
    { type: 'warning' },
  ).catch(() => false)
  if (!confirmed) return
  await withLoading('operation', async () => {
    const ack = (await reloadRuntime(selectedID.value, configDocument.value.source_digest, newIdempotencyKey())).data
    ElMessage.success(`Runtime generation ${ack.generation} acknowledged`)
    await Promise.allSettled([loadConfig(), loadStatus(), loadAudit()])
  })
}

async function simulate () {
  if (!simulationInput.clientA || !simulationInput.clientB) {
    ElMessage.warning('Both client IP addresses are required')
    return
  }
  await withLoading('simulation', async () => {
    simulationResult.value = (await simulateAllocation(selectedID.value, {
      client_a: { ip: simulationInput.clientA },
      client_b: { ip: simulationInput.clientB },
      transport: simulationInput.transport,
      explain: true,
      expected_config_generation: relays.value.config_generation || undefined,
    })).data
  })
}

const compactJSON = value => JSON.stringify(value)
const formatTime = value => value ? new Date(value).toLocaleString() : '-'

onMounted(loadInstances)
</script>

<style scoped lang="scss">
.server-control { padding: 16px; }
.toolbar, .control-tabs, .section { margin-top: 16px; }
.card-header { align-items: center; display: flex; justify-content: space-between; gap: 16px; }
.editor-tabs { margin-top: 16px; }
.apply-button { margin-top: 16px; }
code { overflow-wrap: anywhere; white-space: normal; }
:deep(.el-textarea__inner) { font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; }
</style>
