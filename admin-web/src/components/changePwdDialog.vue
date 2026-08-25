<template>
  <el-dialog v-model="v" width="520px" :title="T('ChangePassword')" :close-on-click-modal="false" destroy-on-close @closed="resetForm">
    <p class="change-password-note">{{ T('PasswordSecurityDescription') }}</p>
    <el-form ref="cpwd" class="dialog-form" :model="changePwdForm" :rules="chagePwdRules" label-position="top">
      <el-form-item :label="T('OldPassword')" prop="old_password">
        <el-input v-model="changePwdForm.old_password" type="password" autocomplete="current-password" :placeholder="T('For OIDC login without a password, enter any 4-20 letters')" show-password></el-input>
      </el-form-item>
      <el-form-item :label="T('NewPassword')" prop="new_password">
        <el-input v-model="changePwdForm.new_password" type="password" autocomplete="new-password" show-password></el-input>
      </el-form-item>
      <el-form-item :label="T('ConfirmPassword')" prop="confirmPwd">
        <el-input v-model="changePwdForm.confirmPwd" type="password" autocomplete="new-password" show-password></el-input>
      </el-form-item>
      <div class="change-password-actions"><el-button @click="cancelChangePwd">{{ T('Cancel') }}</el-button><el-button type="primary" @click="changePassword">{{ T('Confirm') }}</el-button></div>
    </el-form>
  </el-dialog>
</template>

<script setup>

  import { computed, reactive, ref } from 'vue'
  import { ElMessageBox } from 'element-plus'
  import { changeCurPwd } from '@/api/user'
  import { useUserStore } from '@/store/user'
  import { T } from '@/utils/i18n'

  const props = defineProps({
    visible: Boolean,
  })
  const v = computed({
    get: () => props.visible,
    set: (val) => {
      emit('update:visible', val)
    },
  })
  const emit = defineEmits(['update:visible'])

  // Watch for changes to the prop and emit an event if necessary
  // watch(() => props.visible, (newVal) => {
  //   emit('update:visible', newVal);
  // });
  const changePwdForm = reactive({
    old_password: '',
    new_password: '',
    confirmPwd: '',
  })
  const chagePwdRules = computed(_ => ({
    old_password: [{ required: true, message: T('ParamRequired', { param: T('OldPassword') }), trigger: 'blur' }],
    new_password: [
      { required: true, message: T('ParamRequired', { param: T('NewPassword') }), trigger: 'blur' },
      {
        validator: (rule, value, callback) => {
          if (value === changePwdForm.old_password) {
            callback(new Error(T('NewPasswordEqualOldPassword'))) //'新密码不能与旧密码相同'
          } else {
            callback()
          }
        },
        trigger: 'blur',
      }],
    confirmPwd: [
      { required: true, message: T('ParamRequired', { param: T('ConfirmPassword') }), trigger: 'blur' },
      {
        validator: (rule, value, callback) => {
          if (value !== changePwdForm.new_password) {
            callback(new Error(T('PasswordNotMatchConfirmPassword')))
          } else {
            callback()
          }
        },
        trigger: 'blur',
      },
    ],
  }))
  const cpwd = ref(null)
  const resetForm = () => {
    changePwdForm.old_password = ''
    changePwdForm.new_password = ''
    changePwdForm.confirmPwd = ''
  }
  const cancelChangePwd = () => {
    emit('update:visible', false)
  }

  const userStore = useUserStore()

  const changePassword = async () => {
    //验证
    const valid = await cpwd.value.validate().catch(_ => false)
    if (!valid) {
      return
    }
    const confirm = await ElMessageBox.confirm(T('Confirm?', { param: T('ChangePassword') }), {
      confirmButtonText: T('Confirm'),
      cancelButtonText: T('Cancel'),
    }).catch(_ => false)
    if (!confirm) {
      return
    }
    const res = await changeCurPwd(changePwdForm).catch(_ => false)
    if (!res) {
      return
    }
    ElMessageBox.alert(T('OperationSuccess'), T('ChangePassword'), {
      autofocus: true,
      confirmButtonText: 'OK',
      callback: (action) => {
        userStore.logout()
        window.location.reload()
      },
    })
  }
</script>

<style scoped lang="scss">
.change-password-note { margin: -4px 0 18px; color: var(--text-tertiary); font-size: 12px; line-height: 1.6; }
.change-password-actions { display: flex; justify-content: flex-end; gap: 8px; padding-top: 4px; }
@media (max-width: 520px) { .change-password-actions .el-button { flex: 1; margin: 0; } }
</style>
