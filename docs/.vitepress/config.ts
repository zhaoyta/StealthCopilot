// VitePress 文档站配置
import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'StealthCopilot',
  description: '面向跨境求职者的面试辅助工具',
  lang: 'zh-CN',
  base: '/StealthCopilot/',

  themeConfig: {
    nav: [
      { text: '快速开始', link: '/guide/' },
      { text: '配置指南', link: '/guide/api-keys' },
      { text: '隐私说明', link: '/guide/privacy' },
      { text: '贡献指南', link: '/contributing' },
    ],

    sidebar: [
      {
        text: '使用指南',
        items: [
          { text: '快速开始', link: '/guide/' },
          { text: '服务密钥配置', link: '/guide/api-keys' },
          { text: '面试历史记录', link: '/guide/history' },
          { text: '隐私说明', link: '/guide/privacy' },
        ],
      },
      {
        text: '开发者',
        items: [
          { text: '架构说明', link: '/architecture' },
          { text: '贡献指南', link: '/contributing' },
        ],
      },
    ],

    socialLinks: [
      { icon: 'github', link: 'https://github.com/zhaoyta/stealthcopilot' },
    ],

    editLink: {
      pattern: 'https://github.com/zhaoyta/stealthcopilot/edit/main/docs/:path',
      text: '在 GitHub 上编辑此页',
    },
  },
})
