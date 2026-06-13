// i18n.ts — 国际化配置入口
// 默认语言中文，fallback 也为中文，防止 key 缺失时显示 key 字符串。
// 所有 UI 文本必须通过 t() 调用，禁止硬编码字符串（ESLint 强制检查）。
import { createI18n } from 'vue-i18n'
import zhCN from './locales/zh-CN.json'
import enUS from './locales/en-US.json'

const i18n = createI18n({
  legacy: false,         // 使用 Composition API 模式
  locale: 'zh-CN',      // 默认语言
  fallbackLocale: 'zh-CN',
  messages: {
    'zh-CN': zhCN,
    'en-US': enUS,
  },
})

export default i18n
