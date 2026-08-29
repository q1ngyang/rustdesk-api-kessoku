export function normalizeRustDeskId (value) {
  return String(value ?? '').replace(/[\s\u200B\uFEFF]+/gu, '')
}
