<template>
  <div>
    <el-card class="list-query" shadow="hover">
      <el-form inline label-width="80px">
        <el-form-item :label="T('Username')">
          <el-input v-model="listQuery.username"></el-input>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="handlerQuery">{{ T('Filter') }}</el-button>
          <el-button type="danger" @click="toAdd">{{ T('Add') }}</el-button>
          <el-button type="success" @click="toExport">{{ T('Export') }}</el-button>
        </el-form-item>
      </el-form>
    </el-card>
    <el-card class="list-body" shadow="hover">
      <el-table :data="listRes.list" v-loading="listRes.loading" border>
        <el-table-column prop="id" label="ID" align="center"></el-table-column>
        <el-table-column prop="username" :label="T('Username')" align="center"/>
        <el-table-column prop="email" :label="T('Email')" align="center"/>
        <el-table-column prop="nickname" :label="T('Nickname')" align="center"/>
        <el-table-column prop="role" :label="T('Role')" align="center" width="140">
          <template #default="{row}"><el-tag :type="roleType(row.role)" effect="light">{{ T(roleLabel(row.role)) }}</el-tag></template>
        </el-table-column>
        <el-table-column :label="T('Group')" align="center">
          <template #default="{row}">
            <span v-if="row.group_id"> <el-tag>{{ listRes.groups?.find(g => g.id === row.group_id)?.name || `#${row.group_id}` }} </el-tag> </span>
            <span v-else> - </span>
          </template>
        </el-table-column>
        <el-table-column :label="T('Status')" align="center">
          <template #default="{row}">
            <el-switch v-model="row.status"
                       :active-value="ENABLE_STATUS"
                       :inactive-value="DISABLE_STATUS"
                       @change="changeStatus(row)"
            ></el-switch>
          </template>
        </el-table-column>
        <el-table-column prop="remark" :label="T('Remark')" align="center"/>
        <el-table-column prop="created_at" :label="T('CreatedAt')" align="center"/>
        <el-table-column prop="updated_at" :label="T('UpdatedAt')" align="center"/>
        <el-table-column :label="T('Actions')" align="center" min-width="520" class-name="table-actions" fixed="right">
          <template #default="{row}">
            <el-button v-if="isSuperAdmin" @click="toTag(row)">{{ T('UserTags') }}</el-button>
            <el-button v-if="isSuperAdmin" @click="toAddressBook(row)">{{ T('UserAddressBook') }}</el-button>
            <el-button v-if="isSuperAdmin && row.role === 'admin'" type="primary" plain @click="toAccess(row)">{{ T('AccessScope') }}</el-button>
            <el-button @click="toEdit(row)">{{ T('Edit') }}</el-button>
            <el-button type="warning" @click="changePass(row)">{{ T('ResetPassword') }}</el-button>
            <el-button @click="revoke(row)">{{ T('RevokeSessions') }}</el-button>
            <el-button v-if="isSuperAdmin" type="danger" @click="remove(row)">{{ T('Delete') }}</el-button>
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
  import { useRepositories, useDel, useToEditOrAdd, useChangePwd } from '@/views/user/composables'
  import { T } from '@/utils/i18n'
  import { DISABLE_STATUS, ENABLE_STATUS } from '@/utils/common_options'
  import { revokeSessions, update } from '@/api/user'
  import { ElMessageBox, ElMessage } from 'element-plus'
  import { computed, onMounted, watch } from 'vue'
  import { useUserStore } from '@/store/user'
  //列表
  const {
    listRes,
    listQuery,
    handlerQuery,
    getList,
    getGroups,
    toExport,
  } = useRepositories()

  onMounted(getGroups)

  onMounted(getList)

  watch(() => listQuery.page, getList)
  watch(() => listQuery.page_size, handlerQuery)

  const { toEdit, toAdd, toAddressBook, toTag, toAccess } = useToEditOrAdd()
  const userStore = useUserStore()
  const isSuperAdmin = computed(() => userStore.role === 'super_admin')
  const roleLabel = role => ({ user: 'OrdinaryUser', admin: 'ScopedAdministrator', super_admin: 'SuperAdministrator' }[role] || 'OrdinaryUser')
  const roleType = role => ({ user: 'info', admin: 'warning', super_admin: 'danger' }[role] || 'info')

  const { changePass } = useChangePwd()

  //删除
  const { del } = useDel()
  const remove = async (row) => {
    const res = await del(row.id)
    if (res) {
      getList(listQuery)
    }
  }

  const changeStatus = async (row) => {
    /*const confirm = await ElMessageBox.confirm(T('Confirm?', { param: T('Update') }), {
      confirmButtonText: T('Confirm'),
      cancelButtonText: T('Cancel'),
    }).catch(_ => false)
    if (!confirm) {
      return false
    }*/
    const res = await update(row).catch(_ => false)
    if (res) {
      ElMessage.success(T('OperationSuccess'))
      getList(listQuery)
    }
  }

  const revoke = async row => {
    const confirmed = await ElMessageBox.confirm(T('Confirm?', { param: T('RevokeSessions') }), { confirmButtonText: T('Confirm'), cancelButtonText: T('Cancel'), type: 'warning' }).catch(() => false)
    if (!confirmed) return
    const res = await revokeSessions({ id: row.id }).catch(() => false)
    if (res) ElMessage.success(T('OperationSuccess'))
  }

</script>

<style scoped>
</style>
