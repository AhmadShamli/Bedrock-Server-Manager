/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        obsidian: {
          950: '#06090c',
          900: '#0c1015',
          850: '#11171e',
          800: '#17202a',
          700: '#222f3e',
          600: '#334356',
        },
        emerald: {
          DEFAULT: '#10b981',
          dim: '#065f46',
          bright: '#34d399',
          glow: '#059669',
        },
        cyber: {
          cyan: '#06b6d4',
          neon: '#00f0ff',
        }
      },
      fontFamily: {
        mono: ['JetBrains Mono', 'Fira Code', 'monospace'],
        sans: ['Inter', 'system-ui', '-apple-system', 'sans-serif'],
      },
    },
  },
  plugins: [],
}
