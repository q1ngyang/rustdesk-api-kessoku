<template>
  <div>
    <el-card class="list-query" shadow="hover">
      <el-form inline label-width="80px">
        <el-form-item :label="T('User')">
          <el-select v-model="listQuery.user_id" clearable>
            <el-option
                v-for="item in allUsers"
                :key="item.id"
                :label="item.username"
                :value="item.id"
            ></el-option>
          </el-select>
        </el-form-item>
        <el-form-item :label="T('DateRange')"><DateRangeFilter v-model="listQuery.date_range"/></el-form-item>
        <el-form-item>
          <el-button type="primary" @click="handlerQuery">{{ T('Filter') }}</el-button>
          <el-button type="danger" @click="toBatchDelete">{{ T('BatchDelete') }}</el-button>
        </el-form-item>
      </el-form>
    </el-card>
    <el-card class="list-body" shadow="hover">
      <el-table :data="listRes.list" v-loading="listRes.loading" border @selection-change="handleSelectionChange">
        <el-table-column type="selection" align="center" width="50"/>
        <el-table-column prop="id" :label="T('IndexNum')" align="center" width="86"/>
        <el-table-column :label="T('Status')" align="center" width="110"><template #default="{row}"><el-tag :type="sessionStatus(row).type" effect="light">{{ T(sessionStatus(row).label) }}</el-tag></template></el-table-column>
        <el-table-column :label="T('Username')" align="center" width="168">
          <template #default="{row}">
            <UsernameCell :value="usernameFor(row.user_id)"/>
          </template>
        </el-table-column>
        <el-table-column :label="T('SessionIdentifier')" align="center" min-width="180">
          <template #default="{row}">
            <code>{{ compactIdentifier(row.credential_id, row.id) }}</code>
          </template>
        </el-table-column>
        <el-table-column prop="device_id" :label="T('Device')" align="center" min-width="170"><template #default="{row}">{{ row.device_id || '-' }}</template></el-table-column>
        <el-table-column prop="device_uuid" label="UUID" align="center" min-width="250"><template #default="{row}">{{ row.device_uuid || '-' }}</template></el-table-column>
        <el-table-column prop="client" align="center" width="142"><template #header><InfoLabel compact :label="T('ClientType')" :help="T('LoginClientHelp')"/></template><template #default="{row}">{{ row.client || '-' }}</template></el-table-column>
        <el-table-column :label="T('Platform')" align="center" width="120"><template #default="{row}">{{ row.platform || '-' }}</template></el-table-column>
        <el-table-column :label="T('CreatedIP')" align="center" width="170"><template #default="{row}"><IpAddress :value="row.created_ip"/></template></el-table-column>
        <el-table-column :label="T('IssuedAt')" align="center" min-width="175"><template #default="{row}">{{ formatUnix(row.issued_at) }}</template></el-table-column>
        <el-table-column :label="T('CreatedAt')" align="center" min-width="175"><template #default="{row}">{{ formatDate(row.created_at) }}</template></el-table-column>
        <el-table-column :label="T('ExpireTime')" prop="expired_at" align="center" min-width="175">
          <template #default="{row}">
            {{ formatUnix(row.expired_at) }}
          </template>
        </el-table-column>
        <el-table-column :label="T('Actions')" align="center" width="148" fixed="right">
          <template #default="{row}">
            <el-button type="danger" plain :disabled="Boolean(row.revoked_at) || expired(row)" @click="del(row)">{{ T('Logout') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
    <el-card class="list-page" shadow="hover">
      <el-pagination background
                     layout="prev, pager, next, sizes, jumper"
                     :page-sizes="[10,20,50,100]"
                     v-model:page-size="listQuery.page_size"
                     v-model:current-page="listQuery.page"
                     :total="listRes.total">
      </el-pagination>
    </el-card>
  </div>
</template>

<script setup>
  import { onActivated, onMounted, ref, watch } from 'vue'
  import { loadAllUsers } from '@/global'
  import { useRepositories } from '@/views/user/token.js'
  import { T } from '@/utils/i18n'
  import IpAddress from '@/components/common/IpAddress.vue'
  import DateRangeFilter from '@/components/common/DateRangeFilter.vue'
  import InfoLabel from '@/components/common/InfoLabel.vue'
  import UsernameCell from '@/components/common/UsernameCell.vue'

  const { allUsers, getAllUsers } = loadAllUsers()
  getAllUsers()

  const {
    listRes,
    listQuery,
    getList,
    handlerQuery,
    del,
    batchDelete,
  } = useRepositories()

  onMounted(getList)
  onActivated(getList)

  watch(() => listQuery.page, getList)

  watch(() => listQuery.page_size, handlerQuery)
  const usernameFor = userId => allUsers.value?.find(user => user.id === userId)?.username || ''
  const compactIdentifier = (value, id) => value ? `${value.slice(0, 8)}…${value.slice(-4)}` : `session-${id}`
  const formatUnix = value => value ? new Date(value * 1000).toLocaleString() : '-'
  const formatDate = value => value ? new Date(value).toLocaleString() : '-'
  const expired = (row) => {
    const now = new Date().getTime()
    return row.expired_at * 1000 < now
  }
  const sessionStatus = row => row.revoked_at ? { type: 'info', label: 'Revoked' } : expired(row) ? { type: 'warning', label: 'Expired' } : { type: 'success', label: 'Active' }

  const multipleSelection = ref([])
  const handleSelectionChange = (val) => {
    multipleSelection.value = val
  }
  const toBatchDelete = () => {
    if (multipleSelection.value.length === 0) {
      return
    }
    batchDelete(multipleSelection.value.map(v => v.id))
  }
</script>

<style scoped lang="scss">
.list-query .el-select {
  --el-select-width: 160px;
}
code { font-size: 12px; color: var(--text-secondary); }


</style>
