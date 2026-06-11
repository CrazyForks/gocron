<template>
  <div class="mcp-token-page art-full-height">
    <ElCard class="art-table-card" shadow="never">
      <!-- Intro -->
      <ElAlert :closable="false" type="info" show-icon style="margin-bottom: 16px">
        <template #title>{{ t('mcpToken.introTitle') }}</template>
        <div class="intro-body">
          <p>{{ t('mcpToken.introDesc') }}</p>
          <div class="endpoint-row">
            <span class="endpoint-label">{{ t('mcpToken.endpoint') }}</span>
            <ElTag type="primary" effect="plain">{{ endpoint }}</ElTag>
            <ElButton size="small" text type="primary" @click="copyText(endpoint)">
              {{ t('mcpToken.copy') }}
            </ElButton>
          </div>
          <p class="endpoint-hint">{{ t('mcpToken.endpointHint') }}</p>
          <p class="tls-warn">{{ t('mcpToken.tlsWarn') }}</p>
        </div>
      </ElAlert>

      <div class="toolbar">
        <span class="text-base font-medium">{{ t('menus.system.mcpToken') }}</span>
        <div>
          <ElButton :loading="loading" @click="loadList">{{ t('mcpToken.refresh') }}</ElButton>
          <ElButton type="primary" @click="openCreate">{{ t('mcpToken.create') }}</ElButton>
        </div>
      </div>

      <ElTable v-loading="loading" :data="list" border style="width: 100%">
        <ElTableColumn type="index" :label="'#'" width="60" align="center" />
        <ElTableColumn prop="name" :label="t('mcpToken.name')" align="center" />
        <ElTableColumn :label="t('mcpToken.lastUsedAt')" align="center">
          <template #default="{ row }">
            <span v-if="row.last_used_at">{{ formatDateTime(row.last_used_at) }}</span>
            <ElTag v-else type="info" effect="plain" size="small">{{ t('mcpToken.never') }}</ElTag>
          </template>
        </ElTableColumn>
        <ElTableColumn :label="t('mcpToken.createdAt')" width="200" align="center">
          <template #default="{ row }">{{ formatDateTime(row.created_at) }}</template>
        </ElTableColumn>
        <ElTableColumn :label="t('mcpToken.operation')" width="140" align="center">
          <template #default="{ row }">
            <ElButton type="danger" size="small" @click="handleRevoke(row)">
              {{ t('mcpToken.revoke') }}
            </ElButton>
          </template>
        </ElTableColumn>
      </ElTable>
    </ElCard>

    <!-- Create dialog -->
    <ElDialog v-model="createVisible" :title="t('mcpToken.create')" width="460px" align-center>
      <ElForm @submit.prevent>
        <ElFormItem :label="t('mcpToken.name')">
          <ElInput
            v-model="createName"
            :placeholder="t('mcpToken.namePlaceholder')"
            maxlength="64"
            clearable
          />
        </ElFormItem>
      </ElForm>
      <template #footer>
        <ElButton @click="createVisible = false">{{ t('mcpToken.cancel') }}</ElButton>
        <ElButton type="primary" :loading="creating" @click="submitCreate">
          {{ t('mcpToken.confirm') }}
        </ElButton>
      </template>
    </ElDialog>

    <!-- Result dialog: plaintext token shown once -->
    <ElDialog
      v-model="resultVisible"
      :title="t('mcpToken.createdTitle')"
      width="620px"
      align-center
      destroy-on-close
    >
      <ElAlert
        type="warning"
        :closable="false"
        show-icon
        :title="t('mcpToken.onceWarn')"
        style="margin-bottom: 16px"
      />
      <div class="token-box">
        <code class="token-value">{{ newToken }}</code>
        <ElButton type="primary" size="small" @click="copyText(newToken)">
          {{ t('mcpToken.copy') }}
        </ElButton>
      </div>

      <div class="config-label">{{ t('mcpToken.configHint') }}</div>
      <pre class="config-pre">{{ configSnippet }}</pre>
      <div style="text-align: right">
        <ElButton size="small" @click="copyText(configSnippet)">{{
          t('mcpToken.copyConfig')
        }}</ElButton>
      </div>

      <template #footer>
        <ElButton type="primary" @click="resultVisible = false">{{ t('mcpToken.done') }}</ElButton>
      </template>
    </ElDialog>
  </div>
</template>

<script setup lang="ts">
  import { ref, computed, onMounted } from 'vue'
  import { useI18n } from 'vue-i18n'
  import {
    ElButton,
    ElCard,
    ElTable,
    ElTableColumn,
    ElDialog,
    ElForm,
    ElFormItem,
    ElInput,
    ElAlert,
    ElTag,
    ElMessage,
    ElMessageBox
  } from 'element-plus'
  import {
    fetchMcpTokenList,
    createMcpToken,
    removeMcpToken,
    type McpTokenItem
  } from '@/api/mcp-token'
  import { formatDateTime } from '@/utils/date'

  defineOptions({ name: 'McpToken' })

  const { t } = useI18n()

  const endpoint = `${window.location.origin}/mcp`

  const list = ref<McpTokenItem[]>([])
  const loading = ref(false)

  const createVisible = ref(false)
  const createName = ref('')
  const creating = ref(false)

  const resultVisible = ref(false)
  const newToken = ref('')

  const configSnippet = computed(
    () =>
      `{
  "mcpServers": {
    "gocron": {
      "url": "${endpoint}",
      "headers": {
        "Authorization": "Bearer ${newToken.value}"
      }
    }
  }
}`
  )

  async function loadList() {
    loading.value = true
    try {
      list.value = (await fetchMcpTokenList()) || []
    } catch {
      // error toast handled by http util
    } finally {
      loading.value = false
    }
  }

  function openCreate() {
    createName.value = ''
    createVisible.value = true
  }

  async function submitCreate() {
    creating.value = true
    try {
      const res = await createMcpToken(createName.value.trim())
      newToken.value = res.token
      createVisible.value = false
      resultVisible.value = true
      loadList()
    } catch {
      // error toast handled by http util
    } finally {
      creating.value = false
    }
  }

  async function handleRevoke(row: McpTokenItem) {
    try {
      await ElMessageBox.confirm(
        t('mcpToken.confirmRevoke', { name: row.name }),
        t('mcpToken.revoke'),
        {
          confirmButtonText: t('mcpToken.confirm'),
          cancelButtonText: t('mcpToken.cancel'),
          type: 'warning',
          center: true
        }
      )
    } catch {
      return
    }
    try {
      await removeMcpToken(row.id)
      ElMessage.success(t('mcpToken.revokeSuccess'))
      loadList()
    } catch {
      // error toast handled by http util
    }
  }

  function copyText(text: string) {
    navigator.clipboard
      .writeText(text)
      .then(() => ElMessage.success(t('mcpToken.copySuccess')))
      .catch(() => ElMessage.error(t('mcpToken.copyFailed')))
  }

  onMounted(loadList)
</script>

<style scoped>
  .mcp-token-page {
    display: flex;
    flex-direction: column;
  }

  .intro-body {
    font-size: 13px;
    line-height: 1.7;
  }

  .endpoint-row {
    display: flex;
    gap: 8px;
    align-items: center;
    margin: 6px 0;
  }

  .endpoint-label {
    color: var(--el-text-color-secondary);
  }

  .endpoint-hint {
    margin: 0 0 2px;
    color: var(--el-text-color-secondary);
  }

  .tls-warn {
    margin: 0;
    color: var(--el-color-warning);
  }

  .toolbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 14px;
  }

  .token-box {
    display: flex;
    gap: 10px;
    align-items: center;
    margin-bottom: 18px;
  }

  .token-value {
    flex: 1;
    padding: 10px 12px;
    overflow-x: auto;
    font-family: monospace;
    font-size: 13px;
    word-break: break-all;
    background: var(--el-fill-color-light);
    border-radius: 4px;
  }

  .config-label {
    margin-bottom: 6px;
    font-size: 13px;
    color: var(--el-text-color-secondary);
  }

  .config-pre {
    padding: 12px;
    margin: 0 0 8px;
    overflow-x: auto;
    font-family: monospace;
    font-size: 12px;
    background: var(--el-fill-color-light);
    border-radius: 4px;
  }
</style>
