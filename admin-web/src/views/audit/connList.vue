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
      <el-table :data="listRes.list" v-loading="listRes.loading" border @selection-change="handleSelectionChange">
        <el-table-column type="selection" align="center" width="50"/>
        <el-table-column prop="id" :label="T('IndexNum')" align="center" width="86"/>
        <el-table-column :label="T('Controller')" min-width="180"><template #default="{ row }"><AuditEndpoint :id="row.from_peer" :ip="row.ip" :username="row.controller_username || row.from_name"/></template></el-table-column>
        <el-table-column :label="T('ControlledDevice')" min-width="180"><template #default="{ row }"><AuditEndpoint :id="row.peer_id" :ip="row.controlled_ip" :username="row.controlled_username"/></template></el-table-column>
        <el-table-column align="center" width="142">
          <template #header><InfoLabel compact :label="T('Type')" :help="T('ConnectionTypeHelp')"/></template>
          <template #default="{ row }"><el-tag :type="connectionType(row.type).tone" effect="light">{{ T(connectionType(row.type).label) }}</el-tag></template>
        </el-table-column>
        <el-table-column prop="uuid" label="UUID" align="center" min-width="190" show-overflow-tooltip><template #default="{ row }">{{ row.uuid || '-' }}</template></el-table-column>
        <el-table-column :label="T('CreatedAt')" align="center" width="168"><template #default="{ row }"><DateTimeCell :value="row.created_at"/></template></el-table-column>
        <el-table-column :label="T('CloseTime')" align="center" width="168"><template #default="{ row }"><DateTimeCell :value="row.close_time" unix/></template></el-table-column>
        <el-table-column :label="T('Actions')" align="center" width="110" fixed="right"><template #default="{ row }"><el-button type="danger" plain @click="del(row)">{{ T('Delete') }}</el-button></template></el-table-column>
      </el-table>
    </el-card>
    <el-card class="list-page" shadow="hover"><el-pagination v-model:page-size="listQuery.page_size" v-model:current-page="listQuery.page" background layout="prev, pager, next, sizes, jumper" :page-sizes="[10,20,50,100]" :total="listRes.total"/></el-card>
  </div>
</template>

<script setup>
import { onActivated, onMounted, ref, watch } from 'vue'
import { useRepositories } from '@/views/audit/reponsitories'
import AuditEndpoint from '@/components/common/AuditEndpoint.vue'
import DateRangeFilter from '@/components/common/DateRangeFilter.vue'
import DateTimeCell from '@/components/common/DateTimeCell.vue'
import InfoLabel from '@/components/common/InfoLabel.vue'
import { T } from '@/utils/i18n'

const { listRes, listQuery, getList, handlerQuery, del, batchdel, toExport } = useRepositories()
const connectionTypes = {
  0: { label: 'RemoteDesktop', tone: '' }, 1: { label: 'FileTransfer', tone: 'warning' },
  2: { label: 'TCPTunnel', tone: 'info' }, 3: { label: 'ViewCamera', tone: 'success' }, 4: { label: 'Terminal', tone: 'danger' },
}
const connectionType = value => connectionTypes[value] || { label: 'UnknownType', tone: 'info' }
onMounted(getList); onActivated(getList)
watch(() => listQuery.page, getList); watch(() => listQuery.page_size, handlerQuery)
const multipleSelection = ref([])
const handleSelectionChange = value => { multipleSelection.value = value }
const toBatchDelete = () => { if (multipleSelection.value.length) batchdel(multipleSelection.value) }
</script>
