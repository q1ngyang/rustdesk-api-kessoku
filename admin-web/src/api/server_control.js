import request from '@/utils/control_request'

const base = '/server-control/v1'
const pathSegment = value => encodeURIComponent(String(value))
const instanceBase = id => `${base}/instances/${pathSegment(id)}`

export const listInstances = () => request({ url: `${base}/instances` })
export const getPairingStatus = () => request({ url: `${base}/pairing` })
export const createControlPairing = (data, confirmation) => request({
  url: `${base}/pairing/control-agent`,
  method: 'post',
  headers: { 'X-Kessoku-Risk-Confirmation': confirmation },
  data,
})
export const revokeControlPairing = (enrollmentID, confirmation) => request({
  url: `${base}/pairing/control-agent/revoke`,
  method: 'post',
  headers: { 'X-Kessoku-Risk-Confirmation': confirmation },
  data: { enrollment_id: enrollmentID },
})
export const setManagedWriteEnabled = (id, writeEnabled, confirmation) => request({
  url: `${instanceBase(id)}/managed-write`,
  method: 'post',
  headers: { 'X-Kessoku-Risk-Confirmation': confirmation },
  data: { write_enabled: writeEnabled },
})
export const getCapabilities = id => request({ url: `${instanceBase(id)}/capabilities` })
export const getStatus = id => request({ url: `${instanceBase(id)}/status` })
export const getRelays = id => request({ url: `${instanceBase(id)}/relays` })
export const listRelayEnrollments = id => request({ url: `${instanceBase(id)}/relay-enrollments` })
export const createRelayPairing = (id, data, idempotencyKey, confirmation) => request({
  url: `${instanceBase(id)}/relay-enrollments/pairing`,
  method: 'post',
  headers: {
    'Idempotency-Key': idempotencyKey,
    ...(confirmation ? { 'X-Kessoku-Risk-Confirmation': confirmation } : {}),
  },
  data,
})
export const activateRelayEnrollment = (id, data, confirmation) => request({
  url: `${instanceBase(id)}/relay-enrollments/activate`,
  method: 'post',
  headers: { 'X-Kessoku-Risk-Confirmation': confirmation },
  data,
})
export const revokeRelayEnrollment = (id, data) => request({
  url: `${instanceBase(id)}/relay-enrollments/revoke`, method: 'post', data,
})
export const getLogSources = id => request({ url: `${instanceBase(id)}/logs/sources` })
export const getLogs = (id, sourceID, limit = 400) => request({ url: `${instanceBase(id)}/logs`, params: { source_id: sourceID, limit } })
export const setLogLevel = (id, sourceID, level) => request({ url: `${instanceBase(id)}/logs/level`, method: 'post', data: { source_id: sourceID, level } })

export const simulateAllocation = (id, data) => request({
  url: `${instanceBase(id)}/allocation-simulations`,
  method: 'post',
  data,
})

export const getConfig = id => request({ url: `${instanceBase(id)}/config` })
export const getConfigSchema = id => request({ url: `${instanceBase(id)}/config/schema` })

export const validateConfig = (id, document) => request({
  url: `${instanceBase(id)}/config/validate`,
  method: 'post',
  data: { document, format: 'yaml' },
})

export const planConfig = (id, document, etag) => request({
  url: `${instanceBase(id)}/config/plan`,
  method: 'post',
  headers: { 'If-Match': etag },
  data: { document, format: 'yaml' },
})

export const applyConfig = (id, plan, etag, idempotencyKey, comment, riskConfirmation) => request({
  url: `${instanceBase(id)}/config/apply`,
  method: 'post',
  headers: {
    'If-Match': etag,
    'Idempotency-Key': idempotencyKey,
    'X-Kessoku-Plan-Review': plan.review_token,
    ...(riskConfirmation ? { 'X-Kessoku-Risk-Confirmation': riskConfirmation } : {}),
  },
  data: {
    plan_id: plan.plan_id,
    candidate_digest: plan.candidate_digest,
    comment: comment || undefined,
  },
})

export const getOperation = (id, operationID) => request({
  url: `${instanceBase(id)}/operations/${pathSegment(operationID)}`,
})

export const getConfigHistory = id => request({ url: `${instanceBase(id)}/config/history` })

export const rollbackConfig = (id, revisionID, etag, idempotencyKey, comment, confirmation) => request({
  url: `${instanceBase(id)}/config/rollback`,
  method: 'post',
  headers: {
    'If-Match': etag,
    'Idempotency-Key': idempotencyKey,
    'X-Kessoku-Risk-Confirmation': confirmation,
  },
  data: { revision_id: revisionID, comment: comment || undefined },
})

export const reloadRuntime = (id, sourceDigest, idempotencyKey) => request({
  url: `${instanceBase(id)}/runtime/reload`,
  method: 'post',
  headers: { 'Idempotency-Key': idempotencyKey },
  data: { expected_source_digest: sourceDigest },
})

export const listAuditEvents = (page = 1, pageSize = 50, filters = {}) => request({
  url: `${base}/audit-events`,
  params: { page, page_size: pageSize, ...filters },
})

export function newIdempotencyKey () {
  if (globalThis.crypto?.randomUUID) {
    return globalThis.crypto.randomUUID()
  }
  const bytes = new Uint8Array(16)
  globalThis.crypto.getRandomValues(bytes)
  return `kessoku-${Array.from(bytes, value => value.toString(16).padStart(2, '0')).join('')}`
}
