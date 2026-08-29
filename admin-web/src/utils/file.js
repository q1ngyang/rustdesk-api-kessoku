export function get_suffix (filename) {
  var pos = filename.lastIndexOf('.')
  var suffix = ''
  if (pos !== -1) {
    suffix = filename.substring(pos)
  }
  return suffix
}

export function random_string (len) {
  len = len || 32
  var chars = 'ABCDEFGHJKMNPQRSTWXYZabcdefhijkmnprstwxyz2345678'
  var maxPos = chars.length
  var pwd = ''
  for (let i = 0; i < len; i++) {
    pwd += chars.charAt(Math.floor(Math.random() * maxPos))
  }
  return pwd
}

export function random_filename (filename) {
  var suffix = get_suffix(filename)
  var time = new Date()
  var time2 = new Date('2020/01/01')
  return Math.ceil((time.getTime() - time2.getTime()) / 1000) + '_' + random_string(10) + suffix
}

export function utf8_to_b64 (str) {
  return window.btoa(unescape(encodeURIComponent(str)))
}

export function b64_to_utf8 (str) {
  return decodeURIComponent(escape(window.atob(str)))
}

export function jsonToCsv (data) {
  if (!Array.isArray(data) || data.length === 0) {
    return new Blob([''], { type: 'text/csv;charset=utf-8' })
  }
  let csv = ''
  let keys = Object.keys(data[0])
  csv += keys.map(csvCell).join(',') + '\r\n'
  data.forEach(row => {
    csv += keys.map(key => csvCell(row[key])).join(',') + '\r\n'
  })
  return new Blob([csv], { type: 'text/csv;charset=utf-8' })
}

export function csvCell (value) {
  let text = value == null ? '' : String(value)
  const firstNonWhitespace = text.trimStart().charAt(0)
  if ((firstNonWhitespace !== '' && '=+-@'.includes(firstNonWhitespace)) || /^[\t\r\n]/.test(text)) {
    text = `'${text}`
  }
  return `"${text.replaceAll('"', '""')}"`
}

export function downBlob (blob, filename) {
  const url = window.URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  setTimeout(() => {
    window.URL.revokeObjectURL(url)
    document.body.removeChild(a)
  })
}

export function sizeFormat (size) {
	const value = Number(size)
	if (!Number.isFinite(value) || value < 0) return '-'
	if (value < 1024) {
		return value + ' B'
	} else if (value < 1024 * 1024) {
		return (value / 1024).toFixed(2) + ' KB'
	} else if (value < 1024 * 1024 * 1024) {
		return (value / 1024 / 1024).toFixed(2) + ' MB'
	} else if (value < 1024 * 1024 * 1024 * 1024) {
		return (value / 1024 / 1024 / 1024).toFixed(2) + ' GB'
	} else {
		return (value / 1024 / 1024 / 1024 / 1024).toFixed(2) + ' TB'
	}
}
