// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  compatibilityDate: '2025-07-15',
  devtools: { enabled: true },

  runtimeConfig: {
    // 僅 server 端可用（不暴露給瀏覽器）
    backendUrl: `http://localhost:${process.env.BACKEND_PORT ?? '8080'}`,
  },
})

