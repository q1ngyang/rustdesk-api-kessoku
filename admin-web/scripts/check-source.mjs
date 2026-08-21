import { readdir, readFile, stat } from 'node:fs/promises'
import { join, relative } from 'node:path'

const root = new URL('..', import.meta.url).pathname
const sourceRoot = join(root, 'src')
const forbiddenPaths = [
  'src/api/rustdesk.js',
  'src/utils/webclient.js',
  'src/utils/webclient',
  'src/views/rustdesk',
  'src/views/address_book/components/shareByWebClient.vue',
]
const forbiddenSource = [
  '/rustdesk/sendCmd',
  '/rustdesk/cmdList',
  '/rustdesk/cmdCreate',
  '/rustdesk/cmdUpdate',
  '/rustdesk/cmdDelete',
  'shareByWebClient',
  '/webclient2',
  '@/utils/webclient',
  'wc-option:local:access_token',
  'ShareByWebClient',
  'custom-rendezvous-server',
  "localStorage.setItem(`${prefix}key`",
  'console.log(',
  'WEB_CLIENT_PROVIDER',
  'web_client_provider',
  '/config/web-client-provider',
]

async function exists (path) {
  try {
    await stat(path)
    return true
  } catch (error) {
    if (error.code === 'ENOENT') return false
    throw error
  }
}

async function filesBelow (directory) {
  const result = []
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    const path = join(directory, entry.name)
    if (entry.isDirectory()) result.push(...await filesBelow(path))
    else if (/\.(js|mjs|ts|vue)$/.test(entry.name)) result.push(path)
  }
  return result
}

for (const path of forbiddenPaths) {
  if (await exists(join(root, path))) throw new Error(`forbidden legacy/browser-client source remains: ${path}`)
}

for (const file of await filesBelow(sourceRoot)) {
  const contents = await readFile(file, 'utf8')
  for (const forbidden of forbiddenSource) {
    if (contents.includes(forbidden)) {
      throw new Error(`${relative(root, file)} contains forbidden legacy/browser-client input ${forbidden}`)
    }
  }
}

const controlAPI = await readFile(join(root, 'src/api/server_control.js'), 'utf8')
for (const required of [
  "const base = '/server-control/v1'",
  '/allocation-simulations',
  '/config/validate',
  '/config/plan',
  '/config/apply',
  '/config/rollback',
  '/runtime/reload',
  "'If-Match'",
  "'Idempotency-Key'",
]) {
  if (!controlAPI.includes(required)) throw new Error(`typed Control API client is missing ${required}`)
}
if (/https?:\/\//.test(controlAPI)) throw new Error('Control API client must not accept or embed an absolute URL')

const webClientAPI = await readFile(join(root, 'src/api/web_client.js'), 'utf8')
const webClientLauncher = await readFile(join(root, 'src/utils/peer.js'), 'utf8')
if (!webClientAPI.includes("const grantEndpoint = '/api/web-client/v1/grants'")) {
  throw new Error('web client API must use the fixed connection-grant endpoint')
}
if (!webClientAPI.includes("const logoutEndpoint = '/api/web-client/v1/logout'")) {
  throw new Error('web client API must use the fixed connection-token revocation endpoint')
}
if (!webClientAPI.includes('Authorization: `Bearer ${connectionToken}`')) {
  throw new Error('web client logout must revoke only the issued connection token')
}
if (/peer_?[Ii]d/.test(webClientAPI)) {
  throw new Error('remote peer ID must not be submitted as grant identity')
}
if (!webClientLauncher.includes('popup.postMessage(payload, targetOrigin)')) {
  throw new Error('web client grant must use an exact postMessage target origin')
}
for (const required of [
  'kessoku.web-client.ready.v1',
  'kessoku.web-client.grant-accepted.v1',
  'event.origin !== targetOrigin || event.source !== popup',
  "window.removeEventListener('message', receiveMessage)",
  "payload.token = ''",
  'await deliverGrant(popup, targetOrigin, payload)',
  'await revokeConnectionGrant(tokenToRevoke)',
  "connectionToken = ''",
]) {
  if (!webClientLauncher.includes(required)) {
    throw new Error(`web client launcher is missing one-time handoff control: ${required}`)
  }
}
for (const forbidden of ['postMessage(payload, \'*\')', 'localStorage', 'sessionStorage', 'document.cookie']) {
  if (webClientLauncher.includes(forbidden)) {
    throw new Error(`web client launcher contains persistent or wildcard grant transport: ${forbidden}`)
  }
}
if (webClientLauncher.indexOf("window.addEventListener('message', receiveMessage)") >= webClientLauncher.indexOf('popup.location.replace(`${targetOrigin}/`)')) {
  throw new Error('web client message listener must be installed before popup navigation')
}

console.log('Kessoku admin source policy passed')
