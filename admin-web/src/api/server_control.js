import request from '@/utils/control_request'

const base = '/server-control/v1'
const pathSegment = value => encodeURIComponent(String(value))
const instanceBase = id => `${base}/instances/${pathSegment(id)}`

export const listInstances = () => request({ url: `${base}/instances` })
export const getCapabilities = id => request({ url: `${instanceBase(id)}/capabilities` })
export const getStatus = id => request({ url: `${instanceBase(id)}/status` })
export const getRelays = id => request({ url: `${instanceBase(id)}/relays` })
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

export const applyConfig = (id, plan, etag, idempotencyKey, comment) => request({
  url: `${instanceBase(id)}/config/apply`,
  method: 'post',
  headers: { 'If-Match': etag, 'Idempotency-Key': idempotencyKey },
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

export const rollbackConfig = (id, revisionID, etag, idempotencyKey, comment) => request({
  url: `${instanceBase(id)}/config/rollback`,
  method: 'post',
  headers: { 'If-Match': etag, 'Idempotency-Key': idempotencyKey },
  data: { revision_id: revisionID, comment: comment || undefined },
})

export const reloadRuntime = (id, sourceDigest, idempotencyKey) => request({
  url: `${instanceBase(id)}/runtime/reload`,
  method: 'post',
  headers: { 'Idempotency-Key': idempotencyKey },
  data: { expected_source_digest: sourceDigest },
})

export const listAuditEvents = (page = 1, pageSize = 50) => request({
  url: `${base}/audit-events`,
  params: { page, page_size: pageSize },
})

export function newIdempotencyKey () {
  if (globalThis.crypto?.randomUUID) {
    return globalThis.crypto.randomUUID()
  }
  const bytes = new Uint8Array(16)
  globalThis.crypto.getRandomValues(bytes)
  return `kessoku-${Array.from(bytes, value => value.toString(16).padStart(2, '0')).join('')}`
}
