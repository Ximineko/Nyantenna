import { debugCollector } from '~/debug/collector'

/**
 * 把前端诊断采集接到 Nuxt 上：路由跳转、未捕获异常、Vue 错误。
 * API 错误与鉴权事件已在 stores/auth.ts 的拦截器里记录。
 */
export default defineNuxtPlugin((nuxtApp) => {
  const router = useRouter()

  router.afterEach((to, from) => {
    debugCollector.recordRoute({
      ts: Date.now(),
      from: from.fullPath,
      to: to.fullPath,
      name: String(to.name ?? '')
    })
  })

  window.addEventListener('error', (e) => {
    const ev = e as ErrorEvent
    debugCollector.recordJsError(ev.error || ev.message, 'window.error')
  })

  window.addEventListener('unhandledrejection', (e) => {
    debugCollector.recordJsError((e as PromiseRejectionEvent).reason, 'unhandledrejection')
  })

  nuxtApp.vueApp.config.errorHandler = (err) => {
    debugCollector.recordJsError(err, 'vue.errorHandler')
  }
})
