<template>
  <div class="heatmap-svg">
    <svg
      :width="totalWidth"
      :height="totalHeight"
      :viewBox="`0 0 ${totalWidth} ${totalHeight}`"
      xmlns="http://www.w3.org/2000/svg"
      role="img"
      :aria-label="t('cronPreview.heatmapAria')"
    >
      <!-- 顶部小时刻度 -->
      <g>
        <text
          v-for="h in hourLabels"
          :key="`h-${h.value}`"
          :x="labelWidth + h.value * cellSize + cellSize / 2"
          :y="topLabelHeight - 4"
          text-anchor="middle"
          class="label"
        >
          {{ h.value }}
        </text>
      </g>
      <!-- 左侧星期标签 + 格子 -->
      <g
        v-for="(day, rowIdx) in dayLabels"
        :key="`d-${day.value}`"
      >
        <text
          :x="labelWidth - 6"
          :y="topLabelHeight + rowIdx * cellSize + cellSize / 2 + 4"
          text-anchor="end"
          class="label"
        >
          {{ day.label }}
        </text>
        <rect
          v-for="hour in 24"
          :key="`c-${day.value}-${hour - 1}`"
          :x="labelWidth + (hour - 1) * cellSize"
          :y="topLabelHeight + rowIdx * cellSize"
          :width="cellSize - cellGap"
          :height="cellSize - cellGap"
          :fill="cellColor(day.value, hour - 1)"
          rx="2"
        >
          <title>{{ cellTitle(day.value, hour - 1) }}</title>
        </rect>
      </g>
    </svg>
    <div
      v-if="empty"
      class="empty-hint"
    >
      {{ t('cronPreview.heatmapEmpty') }}
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

const props = defineProps({
  // 后端返回的稀疏格式：[{day: 0-6, hour: 0-23, count: n}]
  cells: {
    type: Array,
    default: () => []
  },
  truncated: {
    type: Boolean,
    default: false
  }
})

const cellSize = 14
const cellGap = 2
const labelWidth = 36
const topLabelHeight = 20

const totalWidth = labelWidth + 24 * cellSize + 4
const totalHeight = topLabelHeight + 7 * cellSize + 4

// 星期标签：周一到周日（符合 ISO 习惯，顶部显示周一）
const dayLabels = computed(() => {
  const labels = [
    { value: 1, label: t('cronPreview.monAbbr') },
    { value: 2, label: t('cronPreview.tueAbbr') },
    { value: 3, label: t('cronPreview.wedAbbr') },
    { value: 4, label: t('cronPreview.thuAbbr') },
    { value: 5, label: t('cronPreview.friAbbr') },
    { value: 6, label: t('cronPreview.satAbbr') },
    { value: 0, label: t('cronPreview.sunAbbr') }
  ]
  return labels
})

// 顶部只显示 0/6/12/18，其他留空，避免拥挤
const hourLabels = computed(() => {
  return [0, 6, 12, 18].map((v) => ({ value: v }))
})

// 查表：day-hour -> count
const countMap = computed(() => {
  const m = new Map()
  for (const c of props.cells) {
    m.set(`${c.day}-${c.hour}`, c.count)
  }
  return m
})

const maxCount = computed(() => {
  let max = 0
  for (const c of props.cells) {
    if (c.count > max) max = c.count
  }
  return max
})

const empty = computed(() => props.cells.length === 0)

function cellColor(day, hour) {
  const count = countMap.value.get(`${day}-${hour}`) || 0
  if (count === 0) return '#f0f2f5'
  // 4 档橘色渐变
  const max = maxCount.value || 1
  const ratio = count / max
  if (ratio > 0.66) return '#d97706'
  if (ratio > 0.33) return '#f59e0b'
  if (ratio > 0.1) return '#fbbf24'
  return '#fde68a'
}

function cellTitle(day, hour) {
  const count = countMap.value.get(`${day}-${hour}`) || 0
  const dayName = dayLabels.value.find(d => d.value === day)?.label || ''
  if (count === 0) {
    return `${dayName} ${hour}:00 - ${t('cronPreview.noRuns')}`
  }
  return `${dayName} ${hour}:00 - ${count} ${t('cronPreview.runs')}`
}
</script>

<style scoped>
.heatmap-svg {
  position: relative;
  display: inline-block;
}
.heatmap-svg .label {
  font-size: 10px;
  fill: #64748b;
  font-family: ui-sans-serif, system-ui, -apple-system, sans-serif;
}
.empty-hint {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  color: #94a3b8;
  font-size: 12px;
  pointer-events: none;
}
</style>
