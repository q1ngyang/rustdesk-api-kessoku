import { createReadStream } from 'node:fs'
import { readFile, stat } from 'node:fs/promises'
import { createServer } from 'node:http'
import { extname, resolve, sep } from 'node:path'

const host = '0.0.0.0'
const port = Number.parseInt(process.env.PORT || '4173', 10)
const dist = resolve('dist')
const instanceID = 'qa-hbbs-1'
const token = 'local-browser-qa-token'

const contentTypes = {
  '.css': 'text/css; charset=utf-8',
  '.html': 'text/html; charset=utf-8',
  '.ico': 'image/x-icon',
  '.js': 'text/javascript; charset=utf-8',
  '.json': 'application/json; charset=utf-8',
  '.png': 'image/png',
  '.svg': 'image/svg+xml',
  '.woff2': 'font/woff2',
}

const document = `schema_version: 3
relay:
  default_transport: native
  nodes:
    - id: relay-sg-1
      address: relay.example.invalid:21117
      websocket_address: relay.example.invalid:21119
auth:
  mode: jwt
`

const schema = {
  type: 'object',
  additionalProperties: false,
  required: ['schema_version', 'relay', 'auth'],
  properties: {
    schema_version: { type: 'integer', title: 'Schema version', const: 3 },
    relay: {
      type: 'object',
      title: 'Relay policy',
      required: ['default_transport', 'nodes'],
      properties: {
        default_transport: { type: 'string', title: 'Default transport', enum: ['native', 'wss', 'mixed'] },
        nodes: {
          type: 'array',
          title: 'Relay nodes',
          items: {
            type: 'object',
            required: ['id', 'address'],
            properties: {
              id: { type: 'string', title: 'Node ID' },
              address: { type: 'string', title: 'Native endpoint' },
              websocket_address: { type: 'string', title: 'WSS endpoint' },
            },
          },
        },
      },
    },
    auth: {
      type: 'object',
      title: 'Connection authentication',
      required: ['mode'],
      properties: {
        mode: { type: 'string', title: 'Mode', enum: ['jwt', 'introspection'] },
      },
    },
  },
}

const json = (response, status, body) => {
  response.writeHead(status, {
    'Cache-Control': 'no-store',
    'Content-Type': 'application/json; charset=utf-8',
    'X-Content-Type-Options': 'nosniff',
  })
  response.end(JSON.stringify(body))
}

const control = (response, data, status = 200) => json(response, status, { data })
const legacy = (response, data) => json(response, 200, { code: 0, message: 'success', data })

async function readJSON (request) {
  let body = ''
  for await (const chunk of request) {
    body += chunk
    if (body.length > 2 * 1024 * 1024) throw new Error('request body too large')
  }
  return body ? JSON.parse(body) : {}
}

async function handleAdminAPI (request, response, url) {
  const path = url.pathname.slice('/api/admin'.length)
  if (path === '/config/admin' && request.method === 'GET') {
    legacy(response, {
      title: 'Kessoku local browser QA',
      hello: 'Welcome **QA** <img src="data:,qa" onerror="globalThis.__kessokuXSS=true"><a href="javascript:globalThis.__kessokuXSS=true">unsafe link</a><script>globalThis.__kessokuXSS=true</script>',
    })
    return true
  }
  if (path === '/login-options' && request.method === 'GET') {
    legacy(response, { ops: [], auto_oidc: false, disable_pwd: false, register: false, need_captcha: false })
    return true
  }
  if (path === '/login' && request.method === 'POST') {
    const credentials = await readJSON(request)
    if (credentials.username !== 'admin' || credentials.password !== 'test') {
      json(response, 200, { code: 401, message: 'local QA credentials are admin/test', data: null })
      return true
    }
    legacy(response, {
      token,
      username: 'admin',
      nickname: 'Local QA administrator',
      role: 'admin',
      route_names: ['*'],
    })
    return true
  }
  if (path === '/user/current' && request.method === 'GET') {
    if (request.headers['api-token'] !== token) {
      json(response, 401, { code: 403, message: 'missing local QA token', data: null })
      return true
    }
    legacy(response, {
      token,
      username: 'admin',
      nickname: 'Local QA administrator',
      role: 'admin',
      route_names: ['*'],
    })
    return true
  }
  if (path === '/config/app' && request.method === 'GET') {
    legacy(response, { web_client_mode: 'disabled', web_client_public_origin: '' })
    return true
  }
  if (path === '/user/myOauth' && request.method === 'POST') {
    legacy(response, [])
    return true
  }

  if (!path.startsWith('/server-control/v1/')) return false
  if (request.headers['api-token'] !== token) {
    json(response, 401, { error: { code: 'UNAUTHENTICATED', message: 'missing local QA token', request_id: 'qa-auth' } })
    return true
  }

  if (path === '/server-control/v1/instances' && request.method === 'GET') {
    control(response, [{
      id: instanceID,
      name: 'Singapore QA HBBS',
      enabled: true,
      available: true,
      read_only: false,
    }])
    return true
  }
  if (path === '/server-control/v1/audit-events' && request.method === 'GET') {
    control(response, {
      list: [{
        created_at: '2026-08-19T03:45:00Z',
        actor_user_id: 1,
        action: 'starry.config.plan',
        target_id: instanceID,
        result: 'success',
        error_code: '',
        request_id: 'qa-request-0001',
      }],
      page: Number(url.searchParams.get('page') || 1),
      page_size: Number(url.searchParams.get('page_size') || 50),
      total: 1,
    })
    return true
  }

  const prefix = `/server-control/v1/instances/${instanceID}`
  if (!path.startsWith(`${prefix}/`)) return false
  const operation = path.slice(prefix.length)

  if (operation === '/capabilities' && request.method === 'GET') {
    control(response, {
      protocol: { name: 'starry-control', version: 'v1' },
      instance: { id: instanceID, starry_version: 'patch-v1.2.2-local' },
      capabilities: { config_transaction: true, allocation_simulation: true, runtime_reload: true },
      config: { schema_version: 3, schema_digest: 'sha256:qa-schema-digest' },
    })
    return true
  }
  if (operation === '/status' && request.method === 'GET') {
    control(response, {
      ready: true,
      config: { status: 'active', generation: 7, schema_version: 3, source_digest: 'sha256:qa-source-digest', last_error: '' },
      auth: {
        configured_mode: 'jwt',
        effective_mode: 'jwt',
        verifier_state: 'ready',
        key_count: 2,
        metrics: { accepted: 1247, rejected: 3, jwks_refresh_success: 8, introspection_failures: 0 },
      },
    })
    return true
  }
  if (operation === '/relays' && request.method === 'GET') {
    control(response, {
      config_generation: 7,
      warning: '',
      relays: [{
        id: 'relay-sg-1',
        configured_order: 1,
        native: { state: 'healthy', latency_ms: 16 },
        websocket: { state: 'healthy', latency_ms: 19 },
        eligible_for: ['native', 'wss', 'mixed'],
        referenced_by_rules: ['default-apac'],
      }],
    })
    return true
  }
  if (operation === '/allocation-simulations' && request.method === 'POST') {
    const input = await readJSON(request)
    control(response, {
      request: input,
      matched_rule: { name: 'default-apac' },
      selection: { kind: 'relay', relay_id: 'relay-sg-1' },
      candidates: [{ relay_id: 'relay-sg-1', eligible: true, priority: 1, exclusion_reason: '' }],
    })
    return true
  }
  if (operation === '/config' && request.method === 'GET') {
    control(response, { document, format: 'yaml', etag: '"qa-etag-7"', generation: 7, source_digest: 'sha256:qa-source-digest', drift: false })
    return true
  }
  if (operation === '/config/schema' && request.method === 'GET') {
    control(response, { digest: 'sha256:qa-schema-digest', schema })
    return true
  }
  if (operation === '/config/validate' && request.method === 'POST') {
    const candidate = await readJSON(request)
    control(response, {
      valid: candidate.format === 'yaml' && typeof candidate.document === 'string' && candidate.document.includes('schema_version'),
      diagnostics: [],
      candidate_digest: 'sha256:qa-candidate-digest',
    })
    return true
  }
  if (operation === '/config/plan' && request.method === 'POST') {
    if (request.headers['if-match'] !== '"qa-etag-7"') {
      json(response, 412, { error: { code: 'ETAG_MISMATCH', message: 'stale local QA config', request_id: 'qa-plan-precondition' } })
      return true
    }
    await readJSON(request)
    control(response, {
      plan_id: 'qa-plan-1',
      candidate_digest: 'sha256:qa-candidate-digest',
      base_generation: 7,
      expires_at: '2099-08-19T04:00:00Z',
      impact: { risk: 'high', restart_required: false },
      changes: [{ pointer: '/relay/default_transport', kind: 'replace', from: 'native', to: 'mixed' }],
    })
    return true
  }
  if (operation === '/config/apply' && request.method === 'POST') {
    const input = await readJSON(request)
    if (!request.headers['idempotency-key'] || request.headers['if-match'] !== '"qa-etag-7"' || input.plan_id !== 'qa-plan-1') {
      json(response, 400, { error: { code: 'INVALID_APPLY', message: 'missing guarded apply fields', request_id: 'qa-apply' } })
      return true
    }
    control(response, { id: 'qa-operation-1', kind: 'config_apply', state: 'pending' })
    return true
  }
  if (operation === '/operations/qa-operation-1' && request.method === 'GET') {
    control(response, { id: 'qa-operation-1', kind: 'config_apply', state: 'succeeded', activation_ack: { generation: 8 } })
    return true
  }
  if (operation === '/config/history' && request.method === 'GET') {
    control(response, [{
      id: 'qa-revision-7',
      generation: 7,
      created_at: '2026-08-19T03:30:00Z',
      actor: 'admin',
      comment: 'Browser QA baseline',
      result: 'active',
    }])
    return true
  }
  if (operation === '/config/rollback' && request.method === 'POST') {
    await readJSON(request)
    control(response, { id: 'qa-operation-rollback-1', kind: 'config_rollback', state: 'succeeded', activation_ack: { generation: 8 } })
    return true
  }
  if (operation === '/runtime/reload' && request.method === 'POST') {
    await readJSON(request)
    control(response, { generation: 7, source_digest: 'sha256:qa-source-digest', acknowledged: true })
    return true
  }
  return false
}

async function serveStatic (response, pathname) {
  let file = resolve(dist, pathname === '/' ? 'index.html' : `.${pathname}`)
  if (file !== dist && !file.startsWith(`${dist}${sep}`)) {
    json(response, 400, { error: 'invalid path' })
    return
  }
  let info
  try {
    info = await stat(file)
  } catch (error) {
    if (error.code !== 'ENOENT') throw error
  }
  if (!info?.isFile()) file = resolve(dist, 'index.html')
  response.writeHead(200, {
    'Cache-Control': 'no-store',
    'Content-Type': contentTypes[extname(file)] || 'application/octet-stream',
    'Content-Security-Policy': "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self'; connect-src 'self'",
    'Referrer-Policy': 'no-referrer',
    'X-Content-Type-Options': 'nosniff',
    'X-Frame-Options': 'DENY',
  })
  createReadStream(file).pipe(response)
}

const server = createServer(async (request, response) => {
  try {
    const url = new URL(request.url || '/', `http://${request.headers.host || 'localhost'}`)
    if (url.pathname.startsWith('/api/admin')) {
      if (!await handleAdminAPI(request, response, url)) json(response, 404, { code: 404, message: 'mock endpoint not found', data: null })
      return
    }
    await serveStatic(response, url.pathname)
  } catch (error) {
    json(response, 500, { error: String(error?.message || error) })
  }
})

server.listen(port, host, async () => {
  await readFile(resolve(dist, 'index.html'))
  console.log(`Kessoku browser QA server listening on http://${host}:${port}`)
})
