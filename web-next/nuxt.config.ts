export default defineNuxtConfig({
  compatibilityDate: '2025-01-01',

  // 产物要嵌进 Go 二进制里由 embed.FS 提供，没有 Node 运行时，
  // 因此关闭 SSR、用 nuxt generate 产出纯静态 SPA。
  // Go 侧 (internal/api/server.go 的 NoRoute) 已有 index.html 回退，
  // 所以可以直接用 history 路由，不需要 hash 模式。
  ssr: false,

  modules: ['@nuxt/ui', '@pinia/nuxt'],
  css: ['~/assets/css/main.css'],

  devtools: { enabled: false },

  app: {
    head: {
      title: 'Nyantenna',
      meta: [{ name: 'viewport', content: 'width=device-width, initial-scale=1' }]
    }
  },

  // 开发时把 /api 代理到本地运行的 Nyantenna 服务
  nitro: {
    devProxy: {
      '/api': { target: 'http://127.0.0.1:7575/api', changeOrigin: true }
    }
  },

  future: { compatibilityVersion: 4 }
})
