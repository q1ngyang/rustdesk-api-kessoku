export function mergeBrandingDefaults (saved = {}, defaults = {}) {
  const initializeLegacyText = saved?.defaults_initialized !== true
  return Object.fromEntries(Object.entries(defaults).map(([key, fallback]) => {
    const value = saved?.[key]
    if (typeof value !== 'string') return [key, fallback]
    // Empty asset URLs intentionally select the built-in themed artwork.
    // Empty legacy text columns, however, mean the deployment predates the
    // branding fields and should open with editable defaults populated.
    if (initializeLegacyText && value.trim() === '' && typeof fallback === 'string' && fallback.trim() !== '') return [key, fallback]
    return [key, value]
  }))
}
