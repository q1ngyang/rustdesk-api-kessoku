import { parseDocument, stringify as stringifyYAML } from 'yaml'

export function parseSchemaFormDocument (source) {
  const parsed = parseDocument(source, { prettyErrors: true, uniqueKeys: true })
  if (parsed.errors.length) throw parsed.errors[0]
  const value = parsed.toJS()
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    const error = new TypeError('configuration root must be a mapping')
    error.code = 'CONFIGURATION_ROOT_NOT_MAPPING'
    throw error
  }
  return value
}

// The form edits the parsed document directly and never projects it through a
// Kessoku-owned field list. Unknown fields therefore survive a YAML -> form ->
// YAML round trip and remain subject to Starry's authoritative validation.
export function serializeSchemaFormDocument (value) {
  return stringifyYAML(value, { lineWidth: 0 })
}
