<template>
  <el-main>
    <div class="form-container">
      <el-form ref="form" :model="form" :rules="formRules" label-width="180px" label-position="left" class="password-form">
        <el-form-item :label="t('user.oldPassword')" prop="old_password">
          <el-input v-model="form.old_password" type="password"></el-input>
        </el-form-item>
        <el-form-item :label="t('user.newPassword')" prop="new_password">
          <el-input v-model="form.new_password" type="password" :placeholder="t('user.passwordPlaceholder')"></el-input>
        </el-form-item>
        <el-form-item :label="t('user.confirmNewPassword')" prop="confirm_new_password">
          <el-input v-model="form.confirm_new_password" type="password" :placeholder="t('user.passwordPlaceholder')"></el-input>
        </el-form-item>
        <el-form-item>
          <div class="button-group">
            <el-button type="primary" @click="submit()">{{ t('common.save') }}</el-button>
            <el-button @click="cancel">{{ t('common.cancel') }}</el-button>
          </div>
        </el-form-item>
      </el-form>
    </div>
  </el-main>
</template>

<script>
import { useI18n } from 'vue-i18n'
import userService from '../../api/user'
export default {
  name: 'user-edit-my-password',
  setup() {
    const { t } = useI18n()
    return { t }
  },
  data: function () {
    return {
      form: {
        old_password: '',
        new_password: '',
        confirm_new_password: ''
      },
      formRules: {}
    }
  },
  computed: {
    computedFormRules() {
      return {
        old_password: [
          {required: true, message: this.t('user.oldPasswordRequired'), trigger: 'blur'}
        ],
        new_password: [
          {required: true, message: this.t('user.newPasswordRequired'), trigger: 'blur'}
        ],
        confirm_new_password: [
          {required: true, message: this.t('user.confirmPasswordRequired'), trigger: 'blur'}
        ]
      }
    }
  },
  watch: {
    computedFormRules: {
      handler(newVal) {
        this.formRules = newVal
      },
      immediate: true
    }
  },
  methods: {
    submit () {
      this.$refs['form'].validate((valid) => {
        if (!valid) {
          return false
        }
        this.save()
      })
    },
    save () {
      userService.editMyPassword(this.form, () => {
        this.$router.back()
      })
    },
    cancel () {
      this.$router.back()
    }
  }
}
</script>

<style scoped>
.form-container {
  max-width: 600px;
  margin: 0 auto;
  padding: 20px;
}

.password-form {
  background: white;
  padding: 30px;
  border-radius: 8px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.1);
}

.password-form :deep(.el-form-item:last-child) {
  margin-bottom: 0;
}

.password-form :deep(.el-form-item:last-child .el-form-item__content) {
  margin-left: 0 !important;
}

.button-group {
  display: flex;
  justify-content: center;
  gap: 12px;
  margin-top: 20px;
  width: 100%;
}
</style>
