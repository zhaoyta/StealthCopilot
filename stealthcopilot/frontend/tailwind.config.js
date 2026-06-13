/** @type {import('tailwindcss').Config} */
export default {
  // 扫描所有 Vue/TS 文件，移除未使用的 CSS 类（生产构建）
  content: ['./index.html', './src/**/*.{vue,js,ts,jsx,tsx}'],
  theme: {
    extend: {},
  },
  plugins: [],
}

