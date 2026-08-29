import assert from 'node:assert/strict'
import test from 'node:test'

import { mergeBrandingDefaults } from '../src/utils/branding.js'
import { normalizeRustDeskId } from '../src/utils/rustdesk.js'

test('legacy empty branding text receives defaults while empty assets retain built-ins', () => {
  const defaults = {
    admin_title: 'RustDesk API',
    login_copy: 'Default copy',
    brand_logo_light_url: '',
  }
  assert.deepEqual(mergeBrandingDefaults({
    admin_title: '',
    login_copy: '  ',
    brand_logo_light_url: '',
  }, defaults), defaults)
  assert.equal(mergeBrandingDefaults({ admin_title: 'Enterprise Desk' }, defaults).admin_title, 'Enterprise Desk')
})

test('initialized branding preserves intentionally empty optional text', () => {
  const defaults = { admin_title: 'RustDesk API', login_copy: 'Default copy' }
  assert.deepEqual(mergeBrandingDefaults({
    defaults_initialized: true,
    admin_title: '',
    login_copy: '',
  }, defaults), { admin_title: '', login_copy: '' })
})

test('RustDesk IDs copied with presentation whitespace are canonicalized', () => {
  assert.equal(normalizeRustDeskId(' 384\u00a0308\u3000369 '), '384308369')
  assert.equal(normalizeRustDeskId('desk\u200b-\ufeff01'), 'desk-01')
})
