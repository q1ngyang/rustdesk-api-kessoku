import assert from 'node:assert/strict'
import test from 'node:test'

import { csvCell, jsonToCsv } from '../src/utils/file.js'

test('CSV export neutralizes spreadsheet formulas and quotes values', async () => {
  for (const input of ['=1+1', '+cmd', '-1', '@SUM(A1)', '  =hidden', '\tformula']) {
    assert.ok(csvCell(input).startsWith('"\''), `unsafe CSV cell for ${JSON.stringify(input)}`)
  }
  assert.equal(csvCell('a"b'), '"a""b"')
  assert.equal(csvCell(null), '""')

  const text = await jsonToCsv([{ name: '=1+1', note: 'safe' }]).text()
  assert.equal(text, '"name","note"\r\n"\'=1+1","safe"\r\n')
  assert.equal(await jsonToCsv([]).text(), '')
})
