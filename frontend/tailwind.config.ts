/** @type {import('tailwindcss').Config} */
const { fontFamily } = require('tailwindcss/defaultTheme');

module.exports = {
  content: ['./src/**/*.{html,js,svelte,ts}'],
  darkMode: 'class',
  theme: {
    extend: {
      screens: {
        'xs': '450px',
      },
      fontFamily: {
        sans: ['Inter', ...fontFamily.sans],
        mono: ['"JetBrains Mono"', ...fontFamily.mono]
      },
      colors: {
        'primary': '#0a0a0a', 
        'secondary': '#141414',
        'tertiary': '#2d2d2d',
        'accent': '#450292',
        'accent-hover': '#5809b5',
        'text-primary': '#f9fafb',
        'text-secondary': '#a1a1aa',
      },
      keyframes: {
        'fade-in-up': {
            '0%': { opacity: '0', transform: 'translateY(10px) scale(0.98)' },
            '100%': { opacity: '1', transform: 'translateY(0) scale(1)' },
        },
        'shimmer': {
          '100%': { transform: 'translateX(100%)' },
        },
        'gradient-spin': {
          '0%': { backgroundPosition: '0% 50%' },
          '50%': { backgroundPosition: '100% 50%' },
          '100%': { backgroundPosition: '0% 50%' },
        }
      },
      animation: {
        'fade-in-up': 'fade-in-up 0.4s ease-out forwards',
        'shimmer': 'shimmer 1.5s infinite',
        'gradient-spin': 'gradient-spin 4s linear infinite',
      }
    }
  },
  plugins: [
    require('@tailwindcss/typography'),
  ]
};