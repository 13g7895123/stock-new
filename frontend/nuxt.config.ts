// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  compatibilityDate: '2025-07-15',
  devtools: { enabled: true },

  runtimeConfig: {
    // 僅 server 端可用。由 frontend/.env 的 NUXT_BACKEND_URL 覆蓋
    backendUrl: `http://localhost:8086`,
  },
})

