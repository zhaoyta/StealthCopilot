// lint-staged 配置 — 使用函数过滤掉自动生成的 .d.ts 文件
// lint-staged 会将暂存文件路径直接传给 ESLint，
// .d.ts 是 Vite 自动生成的声明文件，不需要 lint
module.exports = {
  '*.{vue,ts,js}': (files) => {
    const lintable = files.filter((f) => !f.endsWith('.d.ts'))
    return lintable.length ? [`eslint --fix ${lintable.join(' ')}`] : []
  },
}
