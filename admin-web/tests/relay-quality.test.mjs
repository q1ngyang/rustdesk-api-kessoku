import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'

import { parseSchemaFormDocument, serializeSchemaFormDocument } from '../src/utils/schema_form.js'

const source = path => readFile(new URL(`../${path}`, import.meta.url), 'utf8')

test('Starry schema form round trip preserves adaptive and unknown fields', () => {
  const original = `version: 4
relay_quality:
  enabled: true
  strategy: adaptive
  primary_probe_samples: 3
  future_adaptive_setting:
    nested: preserved
future_root:
  value: 17
`
  const model = parseSchemaFormDocument(original)
  model.relay_quality.primary_probe_samples = 4
  const roundTripped = parseSchemaFormDocument(serializeSchemaFormDocument(model))
  assert.equal(roundTripped.relay_quality.strategy, 'adaptive')
  assert.equal(roundTripped.relay_quality.primary_probe_samples, 4)
  assert.deepEqual(roundTripped.relay_quality.future_adaptive_setting, { nested: 'preserved' })
  assert.deepEqual(roundTripped.future_root, { value: 17 })
})

test('schema renderer consumes Starry ui-schema without a copied quality specification', async () => {
  const [view, field] = await Promise.all([
    source('src/views/server_control/index.vue'),
    source('src/components/schema/SchemaField.vue'),
  ])
  assert.match(view, /:ui-schema="schemaBundle\.ui_schema"/)
  assert.match(view, /:help-overrides="relayQualityHelp"/)
  for (const directive of ['ui:order', 'ui:widget', 'ui:help', 'ui:placeholder', 'ui:readonly']) {
    assert.ok(field.includes(directive), `SchemaField does not consume ${directive}`)
  }
  assert.match(field, /Object\.keys\(objectValue\.value\)/)
  assert.doesNotMatch(view, /max_candidates:\s*\{|primary_probe_samples:\s*\{|primary_accept_score:\s*\{/)
})

test('Relay Quality dashboard is aggregate-only and marks simulation non-binding', async () => {
  const view = await source('src/views/server_control/index.vue')
  for (const field of [
    'primary_accepted', 'expansions_triggered', 'p2p_cancellations', 'estimated_probe_attempts_saved',
    'stage_timeouts', 'reports_invalid', 'reports_late', 'fallback_reasons',
    'quality_candidate', 'age_seconds', 'relay_probe_protocol', 'relay_load_protocol',
  ]) assert.ok(view.includes(field), `missing aggregate UI field ${field}`)
  assert.match(view, /RelayTelemetryStaleAlert/)
  assert.match(view, /NoQualityCandidateAlert/)
  assert.match(view, /NonBindingSimulationHelp/)
  assert.match(view, /PossibleAdaptiveFlow/)
  assert.match(view, /TransportEligibility/)
  assert.doesNotMatch(view, /allocation_id|session_uuid|nonce|raw_report|connection_token/)
  assert.doesNotMatch(view, /predicted.*rtt|quality_score|final_score/i)
})

test('adaptive controls and aggregate states have explicit English and Chinese help', async () => {
  const [english, chinese] = await Promise.all([
    source('src/utils/i18n/en.json').then(JSON.parse),
    source('src/utils/i18n/zh_CN.json').then(JSON.parse),
  ])
  const helpKeys = [
    'RelayQualityStrategyHelp', 'RelayQualityPrimarySamplesHelp', 'RelayQualityAcceptScoreHelp',
    'RelayQualityLossGateHelp', 'RelayQualityP2PGraceHelp', 'NonBindingSimulationHelp',
    'PrimaryAcceptedRatioHelp', 'ExpansionTriggeredRatioHelp', 'P2PCancelledHelp',
    'EstimatedProbeAttemptsSavedHelp', 'StageTimeoutsHelp', 'InvalidReportsHelp', 'LateReportsHelp',
    'RelayTelemetryStaleAlertHelp', 'NoQualityCandidateAlertHelp',
  ]
  for (const key of helpKeys) {
    assert.ok(english[key]?.One, `missing English ${key}`)
    assert.ok(chinese[key]?.One, `missing Chinese ${key}`)
    assert.notEqual(english[key].One, chinese[key].One, `${key} was not localized`)
  }
})
