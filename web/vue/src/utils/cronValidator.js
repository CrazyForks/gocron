/**
 * Cron表达式验证器
 * 支持格式：秒 分 时 天 月 周
 * 支持快捷语法：@yearly, @monthly, @weekly, @daily, @midnight, @hourly, @every
 */

import i18n from '@/locales'

const t = (key, params) => i18n.global.t(key, params)

// 快捷语法列表
const SHORTCUTS = [
  '@reboot',
  '@yearly',
  '@annually',
  '@monthly',
  '@weekly',
  '@daily',
  '@midnight',
  '@hourly'
]

// @every 语法正则
const EVERY_PATTERN = /^@every\s+(\d+[smh])+$/

/**
 * 从 spec 中提取 CRON_TZ=/TZ= 前缀，返回 { timezone, spec }
 * 无前缀时 timezone 为空字符串
 */
export function extractTimezone(spec) {
  if (!spec || typeof spec !== 'string') {
    return { timezone: '', spec: spec || '' }
  }
  const trimmed = spec.trim()
  const match = trimmed.match(/^(?:CRON_TZ|TZ)=(\S+)\s+(.+)$/)
  if (match) {
    return { timezone: match[1], spec: match[2] }
  }
  return { timezone: '', spec: trimmed }
}

/**
 * 验证cron表达式
 * @param {string} spec - cron表达式（可带 CRON_TZ= 前缀）
 * @returns {{valid: boolean, message: string}}
 */
export function validateCronSpec(spec) {
  if (!spec || typeof spec !== 'string') {
    return { valid: false, message: t('cronValidator.required') }
  }

  // 剥离 CRON_TZ=/TZ= 前缀后再验证
  const { spec: cronExpr } = extractTimezone(spec)
  const trimmed = cronExpr.trim()

  if (!trimmed) {
    return { valid: false, message: t('cronValidator.required') }
  }

  // 检查快捷语法
  if (trimmed.startsWith('@')) {
    return validateShortcut(trimmed)
  }

  // 检查标准cron表达式
  return validateStandardCron(trimmed)
}

/**
 * 验证快捷语法
 */
function validateShortcut(spec) {
  const lower = spec.toLowerCase()

  // 检查固定快捷语法
  if (SHORTCUTS.includes(lower)) {
    return { valid: true, message: '' }
  }

  // 检查 @every 语法
  if (lower.startsWith('@every')) {
    if (!EVERY_PATTERN.test(lower)) {
      return {
        valid: false,
        message: t('cronValidator.everyFormatError')
      }
    }
    return { valid: true, message: '' }
  }

  return {
    valid: false,
    message: t('cronValidator.shortcutError')
  }
}

/**
 * 验证标准cron表达式（6段式）
 */
function validateStandardCron(spec) {
  const segments = spec.split(/\s+/)

  // 必须是6段
  if (segments.length !== 6) {
    return {
      valid: false,
      message: t('cronValidator.sixFieldsRequired')
    }
  }

  // 字段范围定义
  const ranges = [
    { name: t('cronValidator.fieldSecond'), min: 0, max: 59 },
    { name: t('cronValidator.fieldMinute'), min: 0, max: 59 },
    { name: t('cronValidator.fieldHour'), min: 0, max: 23 },
    { name: t('cronValidator.fieldDay'), min: 1, max: 31 },
    { name: t('cronValidator.fieldMonth'), min: 1, max: 12 },
    { name: t('cronValidator.fieldWeek'), min: 0, max: 7 }
  ]

  // 验证每一段
  for (let i = 0; i < segments.length; i++) {
    const result = validateSegment(segments[i], ranges[i])
    if (!result.valid) {
      return result
    }
  }

  return { valid: true, message: '' }
}

/**
 * 验证单个字段
 */
function validateSegment(segment, range) {
  // 允许的字符
  if (!/^[0-9*/,\-?LW#]+$/.test(segment)) {
    return {
      valid: false,
      message: t('cronValidator.illegalChar', { field: range.name })
    }
  }

  // * 通配符
  if (segment === '*') {
    return { valid: true }
  }

  // ? 占位符（用于天和周）
  if (segment === '?') {
    return { valid: true }
  }

  // 范围：1-5
  if (segment.includes('-')) {
    return validateRange(segment, range)
  }

  // 步长：*/5 或 1-10/2
  if (segment.includes('/')) {
    return validateStep(segment, range)
  }

  // 列表：1,2,3
  if (segment.includes(',')) {
    return validateList(segment, range)
  }

  // 单个数字
  if (/^\d+$/.test(segment)) {
    const num = parseInt(segment, 10)
    if (num < range.min || num > range.max) {
      return {
        valid: false,
        message: t('cronValidator.valueOutOfRange', {
          field: range.name,
          value: num,
          min: range.min,
          max: range.max
        })
      }
    }
    return { valid: true }
  }

  // L, W, # 等特殊字符（简单验证）
  if (/^[LW#]/.test(segment)) {
    return { valid: true }
  }

  return {
    valid: false,
    message: t('cronValidator.formatError', { field: range.name })
  }
}

/**
 * 验证范围表达式：1-5
 */
function validateRange(segment, range) {
  const parts = segment.split('-')
  if (parts.length !== 2) {
    return {
      valid: false,
      message: t('cronValidator.rangeFormatError', { field: range.name })
    }
  }

  const start = parseInt(parts[0], 10)
  const end = parseInt(parts[1], 10)

  if (isNaN(start) || isNaN(end)) {
    return {
      valid: false,
      message: t('cronValidator.rangeNotNumber', { field: range.name })
    }
  }

  if (start < range.min || end > range.max || start > end) {
    return {
      valid: false,
      message: t('cronValidator.rangeInvalid', {
        field: range.name,
        start,
        end
      })
    }
  }

  return { valid: true }
}

/**
 * 验证步长表达式：星号/5 或 1-10/2
 */
function validateStep(segment, range) {
  const parts = segment.split('/')
  if (parts.length !== 2) {
    return {
      valid: false,
      message: t('cronValidator.stepFormatError', { field: range.name })
    }
  }

  const step = parseInt(parts[1], 10)
  if (isNaN(step) || step <= 0) {
    return {
      valid: false,
      message: t('cronValidator.stepNotPositive', { field: range.name })
    }
  }

  // 验证基础部分
  if (parts[0] !== '*') {
    return validateSegment(parts[0], range)
  }

  return { valid: true }
}

/**
 * 验证列表表达式：1,2,3
 */
function validateList(segment, range) {
  const parts = segment.split(',')

  for (const part of parts) {
    const result = validateSegment(part.trim(), range)
    if (!result.valid) {
      return result
    }
  }

  return { valid: true }
}

/**
 * 获取cron表达式示例
 */
export function getCronExamples() {
  return [
    { expr: '0 * * * * *', desc: '每分钟第0秒运行' },
    { expr: '*/20 * * * * *', desc: '每隔20秒运行一次' },
    { expr: '0 30 21 * * *', desc: '每天晚上21:30:00运行' },
    { expr: '0 0 23 * * 6', desc: '每周六晚上23:00:00运行' },
    { expr: '0 0 1 1 * *', desc: '每月1号凌晨1点运行' },
    { expr: '@hourly', desc: '每小时运行一次' },
    { expr: '@daily', desc: '每天运行一次' },
    { expr: '@every 30s', desc: '每隔30秒运行一次' },
    { expr: '@every 1m20s', desc: '每隔1分钟20秒运行一次' }
  ]
}
