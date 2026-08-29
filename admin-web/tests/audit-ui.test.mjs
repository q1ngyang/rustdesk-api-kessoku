import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import test from 'node:test'

const root = new URL('..', import.meta.url)
const read = path => readFile(new URL(path, root), 'utf8')

test('all audit type headers keep the help icon centered beside the label', async () => {
  const [adminLogin, myLogin, tokens, connections, files, infoLabel] = await Promise.all([
    read('src/views/login/log.vue'),
    read('src/views/my/login_log/index.vue'),
    read('src/views/user/token.vue'),
    read('src/views/audit/connList.vue'),
    read('src/views/audit/fileList.vue'),
    read('src/components/common/InfoLabel.vue'),
  ])
  for (const source of [adminLogin, myLogin]) {
    assert.match(source, /<InfoLabel compact :label="T\('ClientType'\)"/)
    assert.match(source, /<InfoLabel compact :label="T\('Type'\)"/)
  }
  assert.match(tokens, /<InfoLabel compact :label="T\('ClientType'\)" :help="T\('LoginClientHelp'\)"/)
  assert.match(connections, /<InfoLabel compact :label="T\('Type'\)" :help="T\('ConnectionTypeHelp'\)"/)
  assert.match(files, /<InfoLabel compact :label="T\('Type'\)" :help="T\('FileAuditTypeHelp'\)"/)
  assert.match(infoLabel, /info-label--compact \{ width: auto; justify-content: center; gap: 0\.45em;/)
})

test('token status precedes username and long usernames wrap without truncation', async () => {
  const [tokens, adminLogin, usernameCell, endpoint] = await Promise.all([
    read('src/views/user/token.vue'),
    read('src/views/login/log.vue'),
    read('src/components/common/UsernameCell.vue'),
    read('src/components/common/AuditEndpoint.vue'),
  ])
  const id = tokens.indexOf('<el-table-column prop="id"')
  const status = tokens.indexOf(':label="T(\'Status\')"')
  const username = tokens.indexOf(':label="T(\'Username\')"')
  assert.ok(id >= 0 && status > id && username > status)
  for (const source of [tokens, adminLogin]) {
    assert.match(source, /:label="T\('Username'\)" align="center" width="168"/)
    assert.match(source, /<UsernameCell :value="usernameFor\(row\.user_id\)"/)
  }
  assert.match(usernameCell, /max-width: 10em;/)
  assert.match(usernameCell, /overflow-wrap: anywhere;/)
  assert.match(usernameCell, /white-space: normal;/)
  assert.doesNotMatch(usernameCell, /text-overflow: ellipsis/)
  assert.match(endpoint, /audit-endpoint__username > span:last-child \{ max-width: 10em;/)
  assert.match(endpoint, /overflow-wrap: anywhere;/)
})

test('all activity pages show the persisted database id as the sequence number', async () => {
  const [tokens, adminLogin, myLogin, connections, files] = await Promise.all([
    read('src/views/user/token.vue'),
    read('src/views/login/log.vue'),
    read('src/views/my/login_log/index.vue'),
    read('src/views/audit/connList.vue'),
    read('src/views/audit/fileList.vue'),
  ])
  for (const source of [tokens, adminLogin, myLogin, connections, files]) {
    assert.match(source, /<el-table-column prop="id" :label="T\('IndexNum'\)" align="center" width="86"\/>/)
  }
  for (const source of [connections, files]) assert.doesNotMatch(source, /\$index|listQuery\.page_size \+ 1/)
  assert.match(files, /T\('ControlledFullPath'\)/)
  assert.match(files, /row\.controlled_paths/)
})

test('platform retention uses zero as the no-cleanup default', async () => {
  const [defaults, platform] = await Promise.all([
    read('src/views/settings/shared.js'),
    read('src/views/settings/platform.vue'),
  ])
  for (const field of [
    'user_token_retention_days',
    'login_log_retention_days',
    'audit_conn_retention_days',
    'audit_file_retention_days',
    'control_audit_retention_days',
  ]) assert.match(defaults, new RegExp(`${field}: 0`))
  assert.match(platform, /:min="0"/)
  assert.match(platform, /T\('RetentionZeroHelp'\)/)
})

test('legacy guest share records are not exposed as address-book access history', async () => {
  const router = await read('src/router/index.js')
  assert.doesNotMatch(router, /ShareRecord|share_record/)
})
