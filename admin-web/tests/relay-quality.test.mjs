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

test('Starry schema v5 form round trip preserves every frozen Fast Relay field and extensions', () => {
  const original = `version: 5
fast_mode:
  relay:
    fast_compat_enabled: false
    fast_media_v1_enabled: true
    authorization_ttl_seconds: 45
    max_bitrate_kbps: 12000
    relay_max_datagram: 1200
    future_fast_setting:
      mode: preserved
future_root:
  value: 18
`
  const model = parseSchemaFormDocument(original)
  model.fast_mode.relay.max_bitrate_kbps = 16000
  const roundTripped = parseSchemaFormDocument(serializeSchemaFormDocument(model))
  assert.deepEqual(roundTripped.fast_mode.relay, {
    fast_compat_enabled: false,
    fast_media_v1_enabled: true,
    authorization_ttl_seconds: 45,
    max_bitrate_kbps: 16000,
    relay_max_datagram: 1200,
    future_fast_setting: { mode: 'preserved' },
  })
  assert.deepEqual(roundTripped.future_root, { value: 18 })
})

test('schema renderer consumes Starry ui-schema without a copied quality specification', async () => {
  const [view, field] = await Promise.all([
    source('src/views/server_control/index.vue'),
    source('src/components/schema/SchemaField.vue'),
  ])
  assert.match(view, /:ui-schema="schemaBundle\.ui_schema"/)
  assert.match(view, /:help-overrides="starrySchemaHelp"/)
  for (const directive of ['ui:order', 'ui:widget', 'ui:help', 'ui:placeholder', 'ui:readonly']) {
    assert.ok(field.includes(directive), `SchemaField does not consume ${directive}`)
  }
  assert.match(field, /Object\.keys\(objectValue\.value\)/)
  assert.doesNotMatch(view, /max_candidates:\s*\{|primary_probe_samples:\s*\{|primary_accept_score:\s*\{/)
})

test('FastRelay dashboard stays aggregate-only and distinguishes support, configuration, dependencies and authorization', async () => {
  const view = await source('src/views/server_control/index.vue')
  for (const field of [
    'fast_relay_authorization', 'fast_media_relay_udp', 'fast_compat_enabled', 'fast_media_v1_enabled',
    'fast_relay?.enabled', 'aggregate.issued',
    'active_fast_media_authorizations', 'fast_media_unavailable', 'reliable_fallbacks', 'cookie_rejected',
    'bind_succeeded', 'bind_rejected', 'role_mismatch', 'session_mismatch', 'allocation_mismatch',
    'rebinds', 'forwarded_packets', 'dropped_packets', 'rate_limited', 'expired_allocations',
  ]) assert.ok(view.includes(field), `missing FastRelay aggregate UI field ${field}`)
  for (const state of ['ConfigurationDisabled', 'DependencyUnmet', 'EnabledNoHealthyCandidate', 'AuthorizedServerEvent', 'UDPActivityUnknown', 'ReliableFallback']) {
    assert.ok(view.includes(state), `missing explicit FastRelay state ${state}`)
  }
  assert.doesNotMatch(view, /session_uuid|allocation_uuid|client_ip|stage_token|signed_grant|private_key|media_content/)
  assert.match(view, /formatOptionalMetric/)
})

test('Relay Quality dashboard is aggregate-only and marks simulation non-binding', async () => {
  const view = await source('src/views/server_control/index.vue')
  for (const field of [
    'primary_probes', 'primary_accepted', 'expansions_triggered', 'p2p_cancellations', 'estimated_probe_attempts_saved',
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
  assert.match(view, /adaptivePrimaryProbeTotal/)
  assert.doesNotMatch(view, /primary_accepted \|\| 0\) \+ Number\(relays\.value\.quality\?\.expansions_triggered/)
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
    'RelayTelemetryStaleAlertHelp', 'NoQualityCandidateAlertHelp', 'FastRelayUnsupportedHelp',
    'ServerCountersOnlyHelp', 'FastRelayProtocolHelp', 'FastCompatStateHelp', 'FastMediaStateHelp',
    'ActiveAuthorizationsHelp', 'ActiveFastMediaAuthorizationsHelp', 'FastMediaUnavailableHelp',
    'ReliableFallbacksHelp', 'ResponseMissesHelp', 'RelayUDPCountersHelp', 'FastCompatConfigHelp',
    'FastMediaConfigHelp', 'AuthorizationTTLHelp', 'MaxBitrateHelp', 'RelayMaxDatagramHelp',
    'RegistryInitializationHelp',
  ]
  for (const key of helpKeys) {
    assert.ok(english[key]?.One, `missing English ${key}`)
    assert.ok(chinese[key]?.One, `missing Chinese ${key}`)
    assert.notEqual(english[key].One, chinese[key].One, `${key} was not localized`)
  }
})

test('SP1 and Relay enrollment UI is allowlist-bound and does not persist sensitive claim material', async () => {
  const [view, api] = await Promise.all([
    source('src/views/server_control/index.vue'),
    source('src/api/server_control.js'),
  ])
  for (const required of [
    'agent_origin_id', 'pairingStatus.agent_origins', 'createControlPairing', 'revokeControlPairing', 'createRelayPairing',
    'activateRelayEnrollment', 'configuration_digest', 'health_snapshot_id', 'activate_after_health',
  ]) assert.ok(view.includes(required) || api.includes(required), `missing pairing UI binding ${required}`)
  assert.doesNotMatch(view, /v-model="controlPairingForm\.(?:agent_url|callback|origin)"/)
  assert.doesNotMatch(view + api, /localStorage|sessionStorage/)
  assert.match(view, /onBeforeUnmount\(clearOneTimeCodes\)/)
  assert.match(view, /confirm:revoke-pairing:/)
  for (const forbidden of ['telemetry_secret', 'node_certificate_pem', 'csr_pem', 'private_key', 'stage_token', 'signed_grant']) {
    assert.ok(!view.includes(forbidden), `pairing UI exposes ${forbidden}`)
  }
})
