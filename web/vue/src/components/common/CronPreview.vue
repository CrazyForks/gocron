<template>
  <div
    class="cron-preview"
    :class="{ 'is-invalid': !!displayError, 'is-loading': loading }"
  >
    <!-- 无内容（刚挂载、spec 空）-->
    <div
      v-if="isEmpty"
      class="empty-state"
    >
      <span>{{ t('cronPreview.waitingInput') }}</span>
    </div>

    <!-- 错误态 -->
    <div
      v-else-if="displayError"
      class="error-state"
    >
      <el-icon><WarningFilled /></el-icon>
      <span>{{ displayError }}</span>
    </div>

    <!-- 正常态（含 optimistic UI：loading 时仍展示上一次结果） -->
    <template v-else>
      <div
        v-if="loading"
        class="loading-hint"
      >
        <el-icon class="is-loading">
          <Loading />
        </el-icon>
        <span>{{ t('cronPreview.computing') }}</span>
      </div>

      <!-- 接下来 N 次 -->
      <div class="section next-runs">
        <div class="section-title">
          <el-icon><Clock /></el-icon>
          <span>{{ t('cronPreview.nextRuns', { count: result.next_runs.length }) }}</span>
          <span
            v-if="result.timezone"
            class="tz-badge"
          >{{ result.timezone }}</span>
        </div>
        <ul
          v-if="result.next_runs.length > 0"
          class="run-list"
        >
          <li
            v-for="(run, idx) in result.next_runs"
            :key="run.unix"
          >
            <span class="idx">#{{ idx + 1 }}</span>
            <span class="ts">{{ formatRun(run) }}</span>
            <span class="rel">{{ relativeTime(run.unix) }}</span>
          </li>
        </ul>
        <div
          v-else
          class="no-runs"
        >
          {{ t('cronPreview.noUpcomingRuns') }}
        </div>
      </div>

      <!-- 热图 -->
      <div class="section heatmap">
        <div class="section-title">
          <el-icon><DataAnalysis /></el-icon>
          <span>{{ t('cronPreview.weeklyDistribution') }}</span>
          <el-tag
            v-if="result.truncated"
            size="small"
            type="warning"
            effect="plain"
          >
            {{ t('cronPreview.truncated') }}
          </el-tag>
        </div>
        <HeatmapSvg
          :cells="result.heatmap_cells || []"
          :truncated="!!result.truncated"
        />
      </div>
    </template>
  </div>
</template>

<script setup>
import { ref, watch, computed, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import { WarningFilled, Clock, Loading, DataAnalysis } from '@element-plus/icons-vue'
import dayjs from 'dayjs'
import HeatmapSvg from './HeatmapSvg.vue'
import taskApi from '@/api/task'

const { t, locale } = useI18n()

const props = defineProps({
  spec: { type: String, default: '' },
  timezone: { type: String, default: '' }
})

// ====== 前端快速校验：明显不像 cron 的输入不走后端 ======
// 允许 @ 快捷 或 5-6 段的 [数字 * / , - ? L W #] 组合
const ROUGH_CRON_RE = /^(@[a-zA-Z]+(\s+[\d\smhs]+)?|[\d*\-,/?LW# ]{3,100})$/

// ====== 状态 ======
const loading = ref(false)
const result = ref({ next_runs: [], heatmap_cells: [], timezone: '', truncated: false })
const errorMsg = ref('')
const hasFetchedOnce = ref(false)

// 轻量内存 cache：同一 (spec, tz) 组合复用
const cache = new Map()
const CACHE_MAX = 32

let debounceTimer = null
let currentRequestId = 0

const isEmpty = computed(() => {
  return !props.spec || props.spec.trim() === ''
})

// Optimistic UI：错误只在"没有上一次成功结果"时才独占显示
const displayError = computed(() => {
  if (!errorMsg.value) return ''
  if (hasFetchedOnce.value && result.value.next_runs.length > 0) {
    // 上一次有结果 → 不显示大错，沉默回落
    return ''
  }
  return errorMsg.value
})

function fetchPreview() {
  const spec = (props.spec || '').trim()
  if (!spec) {
    errorMsg.value = ''
    return
  }

  // 1. 前端快速初筛
  if (!ROUGH_CRON_RE.test(spec)) {
    errorMsg.value = t('cronPreview.invalidSyntax')
    hasFetchedOnce.value = false
    return
  }

  // 2. Cache 命中
  const key = `${spec}|${props.timezone || ''}`
  if (cache.has(key)) {
    applyResult(cache.get(key))
    return
  }

  // 3. 发请求
  const reqId = ++currentRequestId
  loading.value = true

  taskApi.cronPreview(
    { spec, timezone: props.timezone || '', count: 10 },
    (data) => {
      // 丢弃过期响应
      if (reqId !== currentRequestId) return
      loading.value = false
      if (data === null || data === undefined) {
        errorMsg.value = t('cronPreview.requestFailed')
        return
      }
      // LRU-lite：超过上限删最早
      if (cache.size >= CACHE_MAX) {
        const firstKey = cache.keys().next().value
        cache.delete(firstKey)
      }
      cache.set(key, data)
      applyResult(data)
    }
  )
}

function applyResult(data) {
  if (!data.valid) {
    errorMsg.value = data.error || t('cronPreview.invalidSyntax')
    return
  }
  errorMsg.value = ''
  result.value = data
  hasFetchedOnce.value = true
}

function scheduleFetch() {
  if (debounceTimer) clearTimeout(debounceTimer)
  debounceTimer = setTimeout(fetchPreview, 400)
}

watch(
  () => [props.spec, props.timezone],
  () => scheduleFetch(),
  { immediate: true }
)

onBeforeUnmount(() => {
  if (debounceTimer) clearTimeout(debounceTimer)
  // 作废所有在途请求
  currentRequestId++
})

// ====== 展示工具函数 ======
function formatRun(run) {
  // run.iso 是 RFC3339 with timezone，用 dayjs 直接解析保留时区偏移
  return dayjs(run.iso).format('YYYY-MM-DD HH:mm:ss') + ' ' + weekdayName(run.weekday)
}

function weekdayName(wd) {
  const keys = ['sun', 'mon', 'tue', 'wed', 'thu', 'fri', 'sat']
  return t(`cronPreview.${keys[wd]}`)
}

function relativeTime(unix) {
  const diffSec = unix - Math.floor(Date.now() / 1000)
  if (diffSec < 0) return ''
  if (diffSec < 60) return t('cronPreview.inSeconds', { n: diffSec })
  const diffMin = Math.floor(diffSec / 60)
  if (diffMin < 60) return t('cronPreview.inMinutes', { n: diffMin })
  const diffH = Math.floor(diffMin / 60)
  const remM = diffMin % 60
  if (diffH < 24) return remM > 0
    ? t('cronPreview.inHoursMinutes', { h: diffH, m: remM })
    : t('cronPreview.inHours', { n: diffH })
  const diffD = Math.floor(diffH / 24)
  const remH = diffH % 24
  return remH > 0
    ? t('cronPreview.inDaysHours', { d: diffD, h: remH })
    : t('cronPreview.inDays', { n: diffD })
}
</script>

<style scoped>
.cron-preview {
  border: 1px solid #e4e7ed;
  border-radius: 6px;
  padding: 12px 16px;
  background: #fafbfc;
  min-height: 80px;
  transition: border-color 0.2s;
  position: relative;
}
.cron-preview.is-invalid {
  border-color: #f56c6c;
  background: #fff5f5;
}
.cron-preview.is-loading {
  opacity: 0.92;
}

.empty-state, .error-state, .no-runs {
  color: #909399;
  font-size: 13px;
  display: flex;
  align-items: center;
  gap: 6px;
}
.error-state {
  color: #f56c6c;
}

.loading-hint {
  position: absolute;
  top: 8px;
  right: 12px;
  font-size: 12px;
  color: #909399;
  display: flex;
  align-items: center;
  gap: 4px;
}
.loading-hint .is-loading {
  animation: rotating 1.2s linear infinite;
}
@keyframes rotating {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

.section {
  margin-top: 8px;
}
.section:first-child {
  margin-top: 0;
}
.section-title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  font-weight: 500;
  color: #303133;
  margin-bottom: 8px;
}
.tz-badge {
  font-size: 11px;
  color: #909399;
  background: #ebeef5;
  padding: 1px 6px;
  border-radius: 3px;
  font-weight: normal;
}

.run-list {
  list-style: none;
  padding: 0;
  margin: 0;
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 4px 16px;
}
.run-list li {
  display: flex;
  align-items: baseline;
  gap: 8px;
  font-size: 13px;
  font-variant-numeric: tabular-nums;
  color: #606266;
}
.run-list .idx {
  color: #a8abb2;
  font-size: 11px;
  min-width: 22px;
}
.run-list .ts {
  color: #303133;
}
.run-list .rel {
  margin-left: auto;
  color: #909399;
  font-size: 12px;
}

@media (max-width: 768px) {
  .run-list {
    grid-template-columns: 1fr;
  }
}
</style>
