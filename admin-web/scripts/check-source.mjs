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

console.log('Kessoku admin source policy passed')
