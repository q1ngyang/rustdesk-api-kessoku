import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'

const root = new URL('..', import.meta.url)
const read = path => readFile(new URL(path, root), 'utf8')

test('management route uses the typed Server Control page', async () => {
  const router = await read('src/router/index.js')
  assert.match(router, /path: '\/server-control'/)
  assert.match(router, /name: 'ServerControl'/)
  assert.match(router, /views\/server_control\/index\.vue/)
  assert.doesNotMatch(router, /serverCmd|views\/rustdesk/)
})

test('Control API client exposes only fixed DTO operations', async () => {
  const api = await read('src/api/server_control.js')
  for (const path of [
    '/instances',
    '/capabilities',
    '/status',
    '/relays',
    '/allocation-simulations',
    '/config/schema',
    '/config/validate',
    '/config/plan',
    '/config/apply',
    '/config/history',
    '/config/rollback',
    '/runtime/reload',
    '/audit-events',
  ]) assert.ok(api.includes(path), `missing typed route ${path}`)
  assert.doesNotMatch(api, /https?:\/\//)
  assert.doesNotMatch(api, /\bcmd\b|command|option/)
})

test('Control response handling supports the dedicated HTTP envelope without token forwarding', async () => {
  const client = await read('src/utils/control_request.js')
  assert.match(client, /hasOwnProperty\.call\(body, 'data'\)/)
  assert.match(client, /problem\?\.request_id/)
  assert.match(client, /config\.headers\['api-token'\] = token/)
  assert.doesNotMatch(client, /window\.open|launch_url|Authorization/)
  assert.doesNotMatch(await read('src/store/user.js'), /console\.log/)
})

test('operator-provided Markdown is sanitized before v-html rendering', async () => {
  const view = await read('src/views/my/info.vue')
  assert.match(view, /import DOMPurify from 'dompurify'/)
  assert.match(view, /DOMPurify\.sanitize\(/)
  assert.match(view, /marked\.parse\(/)
  assert.doesNotMatch(view, /computed\([^)]*=>\s*marked\(/)
})

test('external identity popups cannot retain an opener', async () => {
  for (const path of ['src/store/user.js', 'src/views/my/info.vue']) {
    const source = await read(path)
    for (const call of source.matchAll(/window\.open\(([^\n]+)\)/g)) {
      assert.match(call[1], /'_blank', 'noopener,noreferrer'/, `${path} has an unsafe popup`)
    }
  }
})

test('web client launch uses only a connection grant and exact-origin in-memory delivery', async () => {
  const api = await read('src/api/web_client.js')
  const launcher = await read('src/utils/peer.js')

  assert.match(api, /\/api\/web-client\/v1\/grants/)
  assert.match(api, /\/api\/web-client\/v1\/logout/)
  assert.match(api, /Authorization: `Bearer \$\{accessToken\}`/)
  assert.match(api, /Authorization: `Bearer \$\{connectionToken\}`/)
  assert.match(api, /axios\.post\(logoutEndpoint, null,/)
  assert.match(api, /\{ platform: 'web' \}/)
  assert.match(api, /Number\.isSafeInteger\(grant\?\.expires_at\)/)
  assert.doesNotMatch(api, /peerId|peer_id|localStorage|sessionStorage|withCredentials: true/)

  assert.match(launcher, /web_client_public_origin/)
  assert.match(launcher, /web_client_mode !== 'builtin'/)
  assert.match(launcher, /url\.origin !== value/)
  assert.match(launcher, /popup\.postMessage\(payload, targetOrigin\)/)
  assert.match(launcher, /kessoku\.web-client\.grant\.v1/)
  assert.match(launcher, /kessoku\.web-client\.ready\.v1/)
  assert.match(launcher, /kessoku\.web-client\.grant-accepted\.v1/)
  assert.match(launcher, /event\.origin !== targetOrigin \|\| event\.source !== popup/)
  assert.match(launcher, /window\.removeEventListener\('message', receiveMessage\)/)
  assert.match(launcher, /payload\.token = ''/)
  assert.match(launcher, /await deliverGrant\(popup, targetOrigin, payload\)/)
  assert.match(launcher, /await revokeConnectionGrant\(tokenToRevoke\)/)
  assert.match(launcher, /connectionToken = ''/)
  assert.doesNotMatch(launcher, /Promise\.all\(/)
  assert.ok(launcher.indexOf("window.addEventListener('message', receiveMessage)") < launcher.indexOf('popup.location.replace(`${targetOrigin}/`)'), 'message listener must be installed before popup navigation')
  assert.doesNotMatch(launcher, /postMessage\([^,]+,\s*['"]\*['"]\)/)
  assert.doesNotMatch(launcher, /[?&#](?:token|grant)=|localStorage|sessionStorage|document\.cookie/)
})

test('dependency and verification commands are exact', async () => {
  const packageJSON = JSON.parse(await read('package.json'))
  assert.equal(packageJSON.private, true)
  assert.equal(packageJSON.packageManager, 'npm@11.12.1')
  assert.equal(packageJSON.engines.node, '24.15.0')
  assert.equal(packageJSON.engines.npm, '11.12.1')
  assert.equal(packageJSON.dependencies.axios, '1.19.0')
  assert.equal(packageJSON.dependencies['@vueuse/core'], '14.3.0')
  assert.equal(packageJSON.dependencies.dompurify, '3.4.13')
  assert.equal(packageJSON.dependencies['element-plus'], '2.14.4')
  assert.equal(packageJSON.scripts.lint, 'node scripts/check-source.mjs')
  assert.equal(packageJSON.scripts.test, 'node --test tests/*.test.mjs')
  for (const section of ['dependencies', 'devDependencies']) {
    for (const [name, version] of Object.entries(packageJSON[section])) {
      assert.match(version, /^\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?$/, `${name} is not exact`)
    }
  }
})

test('monorepo candidate CI is read-only and pins every action to a full commit', async () => {
  const workflow = await read('../.github/workflows/build.yml')
  assert.match(workflow, /permissions:\n  contents: read/)
  assert.match(workflow, /working-directory: admin-web/)
  assert.match(workflow, /NPM_VERSION: 11\.12\.1/)
  assert.match(workflow, /test "\$\(npm --version\)" = "\$\{NPM_VERSION\}"/)
  assert.match(workflow, /npm ci/)
  assert.match(workflow, /npm audit --omit=dev --audit-level=high/)
  assert.match(workflow, /npm audit signatures/)
  assert.match(workflow, /diff -u "\$\{RUNNER_TEMP\}\/admin-web-dist-1\.sha256"/)
  assert.doesNotMatch(workflow, /contents: write|packages: write|push: true|docker\/login-action/)
  for (const line of workflow.split('\n').filter(line => line.includes('uses:'))) {
    assert.match(line, /uses:\s+[^@\s]+@[0-9a-f]{40}(?:\s+#.*)?$/, `action is not immutable: ${line}`)
  }
})
