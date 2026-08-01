import { useAuthStore } from '~/stores/auth'

// 未登录访问受保护页面时跳登录页；已登录访问登录页时回首页。
// 与旧版 vue-router 守卫（router/index.ts）行为一致。
export default defineNuxtRouteMiddleware((to) => {
  const auth = useAuthStore()
  if (to.path === '/login') {
    return auth.isAuthenticated ? navigateTo('/') : undefined
  }
  if (!auth.isAuthenticated) {
    return navigateTo('/login')
  }
})
