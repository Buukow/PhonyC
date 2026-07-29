/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
  theme: {
    extend: {
      colors: {
        canvas: '#faf8f5',
        primary: '#4a9d9a',
        accent: '#e8b86d',
        warn: '#c17767',
        muted: '#6b8e8e',
      },
      boxShadow: {
        card: '0 20px 25px -5px rgb(0 0 0 / 0.04), 0 8px 10px -6px rgb(0 0 0 / 0.04)',
      },
    },
  },
  plugins: [],
}
