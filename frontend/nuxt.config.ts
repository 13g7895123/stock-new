// https://nuxt.com/docs/api/configuration/nuxt-config
export default defineNuxtConfig({
  compatibilityDate: '2025-07-15',
  devtools: { enabled: true },

  app: {
    head: {
      title: '台股監控系統',
      link: [
        { rel: 'icon', type: 'image/png', href: '/9JGckX2a.png' },
      ],
    },
  },

  runtimeConfig: {
    // 僅 server 端可用。由 frontend/.env 的 NUXT_BACKEND_URL 覆蓋
    backendUrl: `http://localhost:8086`,
  },
})

