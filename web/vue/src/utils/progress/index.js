//@ts-ignore
import NProgress from 'nprogress'

NProgress.configure({
  // 动画方式
  easing: 'ease',
  // 递增进度条的速度
  speed: 500,
  // 是否显示加载ico
  showSpinner: false,
  // 自动递增间隔
  trickleSpeed: 200,
  // 初始化时的最小百分比
  minimum: 0.3
})

let activeRequests = 0

export function start() {
  if (activeRequests === 0) {
    NProgress.start()
  }
  activeRequests++
}

export function done() {
  activeRequests = Math.max(0, activeRequests - 1)
  if (activeRequests === 0) {
    NProgress.done()
  }
}

export default { start, done }
