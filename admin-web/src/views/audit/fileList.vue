<template>
  <div>
    <el-card class="list-query" shadow="hover">
      <el-form inline label-width="auto">
        <el-form-item :label="T('ControllerID')"><el-input v-model="listQuery.from_peer" clearable/></el-form-item>
        <el-form-item :label="T('ControlledID')"><el-input v-model="listQuery.peer_id" clearable/></el-form-item>
        <el-form-item :label="T('DateRange')"><DateRangeFilter v-model="listQuery.date_range"/></el-form-item>
        <el-form-item><el-button type="primary" @click="handlerQuery">{{ T('Filter') }}</el-button><el-button type="danger" @click="toBatchDelete">{{ T('BatchDelete') }}</el-button><el-button type="success" @click="toExport">{{ T('Export') }}</el-button></el-form-item>
      </el-form>
    </el-card>
    <el-card class="list-body" shadow="hover">
      <el-table :data="listRes.list" v-loading="listRes.loading" border max-height="750" @selection-change="handleSelectionChange">
        <el-table-column type="selection" align="center" width="50"/>
        <el-table-column prop="id" :label="T('IndexNum')" align="center" width="86"/>
        <el-table-column :label="T('Controller')" min-width="180"><template #default="{ row }"><AuditEndpoint :id="row.from_peer" :ip="row.ip" :username="row.controller_username || row.from_name"/></template></el-table-column>
        <el-table-column :label="T('ControlledDevice')" min-width="180"><template #default="{ row }"><AuditEndpoint :id="row.peer_id" :ip="row.controlled_ip" :username="row.controlled_username"/></template></el-table-column>
        <el-table-column align="center" width="184">
          <template #header><InfoLabel compact :label="T('Type')" :help="T('FileAuditTypeHelp')"/></template>
          <template #default="{ row }"><el-tag :type="row.type === 1 ? 'warning' : 'success'" effect="light">{{ T(row.type === 1 ? 'ControllerToControlled' : 'ControlledToController') }}</el-tag></template>
        </el-table-column>
        <el-table-column :label="T('FileInfo')" min-width="210">
          <template #default="{ row }">
            <div class="file-summary"><strong>{{ fileName(row) }}</strong><small>{{ fileSize(row) }}<template v-if="fileEntries(row).length > 1"> · {{ T('FileCount', { count: fileEntries(row).length }) }}</template></small><el-button v-if="fileEntries(row).length > 1" link type="primary" @click="showAllFile(fileEntries(row))">{{ T('ViewDetails') }}</el-button></div>
          </template>
        </el-table-column>
        <el-table-column :label="T('ControlledFullPath')" min-width="260" show-overflow-tooltip>
          <template #default="{ row }"><div class="controlled-path"><code>{{ controlledPaths(row)[0] || '-' }}</code><small v-if="controlledPaths(row).length > 1">+{{ controlledPaths(row).length - 1 }}</small></div></template>
        </el-table-column>
        <el-table-column prop="uuid" label="UUID" align="center" min-width="190" show-overflow-tooltip><template #default="{ row }">{{ row.uuid || '-' }}</template></el-table-column>
        <el-table-column :label="T('CreatedAt')" align="center" width="168"><template #default="{ row }"><DateTimeCell :value="row.created_at"/></template></el-table-column>
        <el-table-column :label="T('Actions')" align="center" width="110" fixed="right"><template #default="{ row }"><el-button type="danger" plain @click="del(row)">{{ T('Delete') }}</el-button></template></el-table-column>
      </el-table>
    </el-card>
    <el-card class="list-page" shadow="hover"><el-pagination v-model:page-size="listQuery.page_size" v-model:current-page="listQuery.page" background layout="prev, pager, next, sizes, jumper" :page-sizes="[10,20,50,100]" :total="listRes.total"/></el-card>
    <el-dialog v-model="allFilesVisible" :title="T('FileInfo')" width="min(620px, calc(100vw - 28px))">
      <el-table :data="showFiles" max-height="520"><el-table-column type="index" :label="T('IndexNum')" width="76" align="center"/><el-table-column prop="0" :label="T('FileName')" min-width="220" show-overflow-tooltip/><el-table-column :label="T('Size')" width="130"><template #default="{ row }">{{ sizeFormat(row[1]) }}</template></el-table-column></el-table>
      <template #footer><el-button type="primary" @click="allFilesVisible = false">{{ T('Close') }}</el-button></template>
    </el-dialog>
  </div>
</template>

<script setup>
import { onActivated, onMounted, ref, watch } from 'vue'
import { useFileRepositories } from '@/views/audit/reponsitories'
import AuditEndpoint from '@/components/common/AuditEndpoint.vue'
import DateRangeFilter from '@/components/common/DateRangeFilter.vue'
import DateTimeCell from '@/components/common/DateTimeCell.vue'
import InfoLabel from '@/components/common/InfoLabel.vue'
import { sizeFormat } from '@/utils/file'
import { T } from '@/utils/i18n'

const { listRes, listQuery, getList, handlerQuery, del, batchdel, toExport } = useFileRepositories()
const fileEntries = row => Array.isArray(row.info?.files) ? row.info.files.filter(item => Array.isArray(item)) : []
const controlledPaths = row => Array.isArray(row.controlled_paths) && row.controlled_paths.length ? row.controlled_paths : row.path ? [row.path] : []
const basename = value => String(value || '').split(/[\\/]/).filter(Boolean).pop() || T('File')
const fileName = row => fileEntries(row)[0]?.[0] || basename(row.path)
const fileSize = row => {
  const entries = fileEntries(row)
  if (!entries.length) return T('SizeUnknown')
  const bytes = entries.reduce((sum, item) => sum + (Number(item[1]) || 0), 0)
  return sizeFormat(bytes)
}
onMounted(getList); onActivated(getList)
watch(() => listQuery.page, getList); watch(() => listQuery.page_size, handlerQuery)
const multipleSelection = ref([])
const handleSelectionChange = value => { multipleSelection.value = value }
const toBatchDelete = () => { if (multipleSelection.value.length) batchdel(multipleSelection.value) }
const allFilesVisible = ref(false); const showFiles = ref([])
const showAllFile = files => { showFiles.value = files; allFilesVisible.value = true }
</script>

<style scoped>
.file-summary { display: flex; min-width: 0; align-items: flex-start; flex-direction: column; gap: 3px; }.file-summary strong { max-width: 100%; overflow: hidden; color: var(--text-primary); font-size: 11px; text-overflow: ellipsis; white-space: nowrap; }.file-summary small { color: var(--text-tertiary); font-size: 10px; }.file-summary .el-button { height: auto; padding: 1px 0; font-size: 10px; }.controlled-path { display: flex; min-width: 0; align-items: center; gap: 6px; }.controlled-path code { min-width: 0; overflow: hidden; color: var(--text-secondary); font-size: 10px; text-overflow: ellipsis; white-space: nowrap; }.controlled-path small { flex: 0 0 auto; color: var(--primary); font-size: 9px; font-weight: 700; }
</style>
