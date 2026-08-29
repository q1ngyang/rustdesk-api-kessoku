export function withDateRange (query) {
  const params = { ...query }
  const range = Array.isArray(params.date_range) ? params.date_range : []
  delete params.date_range
  if (range.length === 2) {
    const from = new Date(`${range[0]}T00:00:00`)
    const to = new Date(`${range[1]}T00:00:00`)
    to.setDate(to.getDate() + 1)
    if (!Number.isNaN(from.getTime()) && !Number.isNaN(to.getTime())) {
      params.created_from = from.toISOString()
      params.created_to = to.toISOString()
    }
  }
  return params
}
