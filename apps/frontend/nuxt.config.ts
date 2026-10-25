export default defineNuxtConfig({
  compatibilityDate: '2026-10-25',
  ssr: false,
  modules: ['@nuxtjs/tailwindcss'],
  css: ['~/assets/css/main.css'],
  runtimeConfig: {
    public: {
      apiBase: '/api/v1',
    },
  },
  vite: {
    server: {
      proxy: {
        '/api': 'http://127.0.0.1:8080',
      },
    },
  },
  app: {
    head: {
      title: 'DBackup',
      meta: [
        { name: 'viewport', content: 'width=device-width, initial-scale=1' },
        { name: 'description', content: 'Database backup control plane' },
      ],
    },
  },
})
