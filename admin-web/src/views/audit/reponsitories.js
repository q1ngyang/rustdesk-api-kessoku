import { reactive } from 'vue'
import { list, remove, fileList, fileRemove, batchDelete, fileBatchDelete } from '@/api/audit'
import { ElMessage, ElMessageBox } from 'element-plus'
import { T } from '@/utils/i18n'
import { downBlob, jsonToCsv } from '@/utils/file'
import { normalizeRustDeskId } from '@/utils/rustdesk'
import { withDateRange } from '@/utils/dateRange'

export function useRepositories () {
  const listRes = reactive({
    list: [], total: 0, loading: false,
  })
  const listQuery = reactive({
    page: 1,
    page_size: 10,
    peer_id: null,
    from_peer: null,
    date_range: [],
  })

  const getList = async () => {
    listRes.loading = true
    const res = await list(withDateRange(listQuery)).catch(_ => false)
    listRes.loading = false
    if (res) {
      listRes.list = res.data.list || []
      listRes.total = res.data.total
    }
  }
  const handlerQuery = () => {
    listQuery.peer_id = normalizeRustDeskId(listQuery.peer_id)
    listQuery.from_peer = normalizeRustDeskId(listQuery.from_peer)
    if (listQuery.page === 1) {
      getList()
    } else {
      listQuery.page = 1
    }
  }

  const del = async (row) => {
    const cf = await ElMessageBox.confirm(T('Confirm?', { param: T('Delete') }), {
      confirmButtonText: T('Confirm'),
      cancelButtonText: T('Cancel'),
      type: 'warning',
    }).catch(_ => false)
    if (!cf) {
      return false
    }

    const res = await remove({ id: row.id }).catch(_ => false)
    if (res) {
      ElMessage.success(T('OperationSuccess'))
      getList()
    }
  }
  const batchdel = async (rows) => {
    const ids = rows.map(r => r.id)
    if (!ids.length) {
      ElMessage.warning(T('PleaseSelectData'))
      return false
    }
    const cf = await ElMessageBox.confirm(T('Confirm?', { param: T('BatchDelete') }), {
      confirmButtonText: T('Confirm'),
      cancelButtonText: T('Cancel'),
      type: 'warning',
    }).catch(_ => false)
    if (!cf) {
      return false
    }

    const res = await batchDelete({ ids }).catch(_ => false)
    if (res) {
      ElMessage.success(T('OperationSuccess'))
      getList()
    }
  }

  const toExport = async () => {
    const q = withDateRange(listQuery)
    q.page_size = 1000000
    q.page = 1
    const res = await list(q).catch(_ => false)
    if (res) {
      const csv = jsonToCsv(res.data.list)
      downBlob(csv, 'connectLog.csv')
    }
  }
  return {
    listRes,
    listQuery,
    getList,
    handlerQuery,
    del,
    batchdel,
    toExport,
  }
}

export function useFileRepositories () {
  const listRes = reactive({
    list: [], total: 0, loading: false,
  })
  const listQuery = reactive({
    page: 1,
    page_size: 10,
    peer_id: null,
    from_peer: null,
    date_range: [],
  })

  const getList = async () => {
    listRes.loading = true
    const res = await fileList(withDateRange(listQuery)).catch(_ => false)
    listRes.loading = false
    if (res) {
      listRes.list = (res.data.list || []).map(item => {
        try { item.info = item.info ? JSON.parse(item.info) : { files: [] } } catch { item.info = { files: [] } }
        if (!Array.isArray(item.info?.files)) item.info = { ...item.info, files: [] }
        return item
      })
      listRes.total = res.data.total
    }
  }
  const handlerQuery = () => {
    listQuery.peer_id = normalizeRustDeskId(listQuery.peer_id)
    listQuery.from_peer = normalizeRustDeskId(listQuery.from_peer)
    if (listQuery.page === 1) {
      getList()
    } else {
      listQuery.page = 1
    }
  }

  const del = async (row) => {
    const cf = await ElMessageBox.confirm(T('Confirm?', { param: T('Delete') }), {
      confirmButtonText: T('Confirm'),
      cancelButtonText: T('Cancel'),
      type: 'warning',
    }).catch(_ => false)
    if (!cf) {
      return false
    }

    const res = await fileRemove({ id: row.id }).catch(_ => false)
    if (res) {
      ElMessage.success(T('OperationSuccess'))
      getList()
    }
  }
  const batchdel = async (rows) => {
    const ids = rows.map(r => r.id)
    if (!ids.length) {
      ElMessage.warning(T('PleaseSelectData'))
      return false
    }
    const cf = await ElMessageBox.confirm(T('Confirm?', { param: T('BatchDelete') }), {
      confirmButtonText: T('Confirm'),
      cancelButtonText: T('Cancel'),
      type: 'warning',
    }).catch(_ => false)
    if (!cf) {
      return false
    }

    const res = await fileBatchDelete({ ids }).catch(_ => false)
    if (res) {
      ElMessage.success(T('OperationSuccess'))
      getList()
    }
  }

  const toExport = async () => {
    const q = withDateRange(listQuery)
    q.page_size = 1000000
    q.page = 1
    const res = await fileList(q).catch(_ => false)
    if (res) {
      const csv = jsonToCsv(res.data.list)
      downBlob(csv, 'fileTransformLog.csv')
    }
  }

  return {
    listRes,
    listQuery,
    getList,
    handlerQuery,
    del,
    batchdel,
    toExport,
  }
}
