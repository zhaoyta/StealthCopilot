// main.ts — 应用入口，注册全局插件
import { createApp } from 'vue'
import App from './App.vue'
import i18n from './i18n'
import './style.css'

createApp(App)
  .use(i18n)
  .mount('#app')
