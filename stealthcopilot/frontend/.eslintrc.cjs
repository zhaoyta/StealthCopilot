// ESLint 配置 — 强制 i18n 规范：禁止硬编码用户可见字符串
// @intlify/vue-i18n/no-raw-text 检测 Vue 模板中的裸字符串
// @intlify/vue-i18n/no-missing-keys 检测使用了不存在的 i18n key
/** @type {import('eslint').Linter.Config} */
module.exports = {
  root: true,
  env: {
    browser: true,
    es2021: true,
    node: true,
  },
  extends: [
    'eslint:recommended',
    'plugin:vue/vue3-recommended',
    'plugin:@typescript-eslint/recommended',
    'plugin:@intlify/vue-i18n/recommended',
  ],
  parser: 'vue-eslint-parser',
  parserOptions: {
    ecmaVersion: 'latest',
    sourceType: 'module',
    parser: '@typescript-eslint/parser',
  },
  settings: {
    'vue-i18n': {
      localeDir: './src/locales/*.json',
      messageSyntaxVersion: '^9.0.0',
    },
  },
  rules: {
    // 禁止 Vue 模板中直接使用用户可见的裸字符串，必须走 $t()
    '@intlify/vue-i18n/no-raw-text': 'error',
    // 检测未在 locale 文件中定义的 key
    '@intlify/vue-i18n/no-missing-keys': 'error',
    // 允许 any 类型（Wails binding 生成代码中常见）
    '@typescript-eslint/no-explicit-any': 'warn',
    // Vue recommended 的排版规则对现有 Wails 页面噪声较大，保留给格式化工具处理
    'vue/max-attributes-per-line': 'off',
    'vue/multiline-html-element-content-newline': 'off',
    'vue/singleline-html-element-content-newline': 'off',
  },
  // 排除规则见 .eslintignore（lint-staged 直接传文件时 ignorePatterns 无效）
  ignorePatterns: [],
}
