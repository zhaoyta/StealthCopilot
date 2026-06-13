// commitlint 配置 — 强制 Conventional Commits 格式
// 允许的 type：feat/fix/docs/style/refactor/perf/test/chore/ci/revert
module.exports = {
  extends: ['@commitlint/config-conventional'],
  rules: {
    // scope 枚举：各功能模块名称
    'scope-enum': [
      2,
      'always',
      ['audio', 'video', 'ghost', 'hearing', 'speaking', 'rag', 'ui', 'ci', 'docs', 'config', 'deps'],
    ],
  },
}
