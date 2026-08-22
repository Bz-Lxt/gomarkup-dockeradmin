/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{vue,ts}'],
  theme: {
    extend: {
      colors: {
        ink: {
          0: 'var(--ink-0)',
          1: 'var(--ink-1)',
          2: 'var(--ink-2)',
        },
        line: 'var(--line)',
        signal: 'var(--signal)',
        'signal-dim': 'var(--signal-dim)',
        ok: 'var(--ok)',
        warn: 'var(--warn)',
        danger: 'var(--danger)',
        'text-hi': 'var(--text-hi)',
        'text-lo': 'var(--text-lo)',
      },
      fontFamily: {
        display: ['"Chakra Petch"', 'sans-serif'],
        body: ['"IBM Plex Sans"', 'sans-serif'],
        mono: ['"IBM Plex Mono"', 'monospace'],
      },
      keyframes: {
        fadeSlideUp: {
          '0%': { opacity: '0', transform: 'translateY(12px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' },
        },
        pulseDot: {
          '0%, 100%': { opacity: '1', boxShadow: '0 0 0 0 currentColor' },
          '50%': { opacity: '.65', boxShadow: '0 0 0 4px transparent' },
        },
        flashUp: { '0%': { color: 'var(--signal)' }, '100%': { color: 'var(--text-hi)' } },
      },
      animation: {
        'fade-up': 'fadeSlideUp .4s ease both',
        'pulse-dot': 'pulseDot 2s ease-in-out infinite',
        'flash-up': 'flashUp .3s ease',
      },
    },
  },
  plugins: [],
}
