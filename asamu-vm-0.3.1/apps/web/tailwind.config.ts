import type { Config } from 'tailwindcss'

export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        asamu: {
          canvas: 'var(--asamu-canvas)',
          grid: 'var(--asamu-grid)',
          ink: 'var(--asamu-ink)',
          blue: 'var(--asamu-blue)',
          yellow: 'var(--asamu-yellow)',
          success: 'var(--asamu-success)',
          danger: 'var(--asamu-danger)',
          muted: 'var(--asamu-muted)',
          card: 'var(--asamu-card)',
          soft: 'var(--asamu-soft)',
          line: 'var(--asamu-line)',
        },
      },
      fontFamily: { display: ['"Arial Black"', 'Inter', '"Microsoft YaHei"', 'sans-serif'] },
      boxShadow: {
        pixel: '4px 4px 0 #10233F',
        pixelSm: '2px 2px 0 #10233F',
      },
    },
  },
  plugins: [],
} satisfies Config
