<template>
  <el-main>
    <el-form :inline="true">
      <el-form-item :label="t('audit.module')">
        <el-select v-model.trim="searchParams.module" style="width: 150px">
          <el-option :label="t('message.all')" value=""></el-option>
          <el-option
            v-for="item in moduleList"
            :key="item.value"
            :label="item.label"
            :value="item.value"
          ></el-option>
        </el-select>
      </el-form-item>
      <el-form-item :label="t('audit.action')">
        <el-select v-model.trim="searchParams.action" style="width: 160px">
          <el-option :label="t('message.all')" value=""></el-option>
          <el-option
            v-for="item in actionList"
            :key="item.value"
            :label="item.label"
            :value="item.value"
          ></el-option>
        </el-select>
      </el-form-item>
      <el-form-item :label="t('user.username')">
        <el-input v-model.trim="searchParams.username"></el-input>
      </el-form-item>
      <el-form-item :label="t('common.date')">
        <el-date-picker
          v-model="dateRange"
          type="daterange"
          value-format="YYYY-MM-DD"
          :range-separator="'-'"
          :start-placeholder="t('common.date')"
          :end-placeholder="t('common.date')"
          style="width: 240px"
        ></el-date-picker>
      </el-form-item>
      <el-form-item>
        <el-button type="primary" @click="search()">{{ t('common.search') }}</el-button>
      </el-form-item>
    </el-form>
    <el-pagination
      background
      layout="prev, pager, next, sizes, total"
      :total="logTotal"
      v-model:current-page="searchParams.page"
      v-model:page-size="searchParams.page_size"
      @size-change="changePageSize"
      @current-change="changePage"
    ></el-pagination>
    <el-table :data="logs" border ref="table" style="width: 100%">
      <el-table-column :label="t('system.loginTime')" width="180">
        <template #default="scope">
          {{ $filters.formatTime(scope.row.created) }}
        </template>
      </el-table-column>
      <el-table-column prop="username" :label="t('user.username')"></el-table-column>
      <el-table-column :label="t('audit.module')" width="100">
        <template #default="scope">
          <el-tag :type="moduleTagType(scope.row.module)" size="small">
            {{ moduleLabel(scope.row.module) }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column :label="t('audit.action')" width="120">
        <template #default="scope">
          <el-tag :type="actionTagType(scope.row.action)" size="small">
            {{ actionLabel(scope.row.action) }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column :label="t('audit.target')">
        <template #default="scope">
          {{ scope.row.target_name || scope.row.target_id }}
        </template>
      </el-table-column>
      <el-table-column prop="ip" :label="t('system.loginIp')"></el-table-column>
      <el-table-column :label="t('audit.detail')" width="100">
        <template #default="scope">
          <el-button
            v-if="scope.row.detail"
            type="info"
            size="small"
            @click="showDetail(scope.row)"
          >{{ t('taskLog.viewOutput') }}</el-button>
        </template>
      </el-table-column>
    </el-table>
    <el-dialog :title="t('audit.detail')" v-model="dialogVisible" width="50%">
      <pre>{{ currentDetail }}</pre>
    </el-dialog>
  </el-main>
</template>

<script>
import { useI18n } from 'vue-i18n'
import auditService from '../../api/audit'

export default {
  name: 'audit-log',
  setup() {
    const { t } = useI18n()
    return { t }
  },
  data() {
    return {
      logs: [],
      logTotal: 0,
      searchParams: {
        page_size: 20,
        page: 1,
        module: '',
        action: '',
        username: '',
        start_date: '',
        end_date: ''
      },
      dateRange: [],
      dialogVisible: false,
      currentDetail: '',
      moduleList: [],
      actionList: []
    }
  },
  computed: {
    computedModuleList() {
      return [
        { value: 'task', label: this.t('audit.module_task') },
        { value: 'host', label: this.t('audit.module_host') },
        { value: 'user', label: this.t('audit.module_user') },
        { value: 'system', label: this.t('audit.module_system') }
      ]
    },
    computedActionList() {
      return [
        { value: 'create', label: this.t('audit.action_create') },
        { value: 'update', label: this.t('audit.action_update') },
        { value: 'delete', label: this.t('audit.action_delete') },
        { value: 'enable', label: this.t('audit.action_enable') },
        { value: 'disable', label: this.t('audit.action_disable') },
        { value: 'run', label: this.t('audit.action_run') },
        { value: 'batch-enable', label: this.t('audit.action_batch_enable') },
        { value: 'batch-disable', label: this.t('audit.action_batch_disable') },
        { value: 'batch-remove', label: this.t('audit.action_batch_remove') },
        { value: 'change-password', label: this.t('audit.action_change_password') },
        { value: 'reset-password', label: this.t('audit.action_reset_password') }
      ]
    }
  },
  watch: {
    computedModuleList: {
      handler(newVal) {
        this.moduleList = newVal
      },
      immediate: true
    },
    computedActionList: {
      handler(newVal) {
        this.actionList = newVal
      },
      immediate: true
    }
  },
  created() {
    this.search()
  },
  methods: {
    changePage(page) {
      this.searchParams.page = page
      this.search()
    },
    changePageSize(pageSize) {
      this.searchParams.page_size = pageSize
      this.search()
    },
    search() {
      if (this.dateRange && this.dateRange.length === 2) {
        this.searchParams.start_date = this.dateRange[0]
        this.searchParams.end_date = this.dateRange[1]
      } else {
        this.searchParams.start_date = ''
        this.searchParams.end_date = ''
      }
      auditService.list(this.searchParams, data => {
        this.logs = data.data
        this.logTotal = data.total
      })
    },
    moduleLabel(module) {
      const found = this.moduleList.find(item => item.value === module)
      return found ? found.label : module
    },
    moduleTagType(module) {
      const types = {
        task: '',
        host: 'success',
        user: 'warning',
        system: 'danger'
      }
      return types[module] || 'info'
    },
    actionLabel(action) {
      const found = this.actionList.find(item => item.value === action)
      return found ? found.label : action
    },
    actionTagType(action) {
      const types = {
        create: 'success',
        update: 'warning',
        delete: 'danger',
        enable: 'success',
        disable: 'info',
        run: '',
        'batch-enable': 'success',
        'batch-disable': 'info',
        'batch-remove': 'danger',
        'change-password': 'warning',
        'reset-password': 'warning'
      }
      return types[action] || 'info'
    },
    showDetail(row) {
      this.currentDetail = row.detail
      this.dialogVisible = true
    }
  }
}
</script>

<style scoped>
pre {
  white-space: pre-wrap;
  word-wrap: break-word;
  padding: 10px;
  background-color: #4c4c4c;
  color: white;
}
</style>
