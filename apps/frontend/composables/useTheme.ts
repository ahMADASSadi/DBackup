export type ThemeName = 'light' | 'dark'

export function useTheme() {
  const theme = useState<ThemeName>('theme', () => 'light')

  function apply(value: ThemeName) {
    theme.value = value
    if (import.meta.client) {
      document.documentElement.classList.toggle('dark', value === 'dark')
      localStorage.setItem('dbackup-theme', value)
    }
  }

  function init() {
    if (!import.meta.client) return
    const saved = localStorage.getItem('dbackup-theme')
    if (saved === 'light' || saved === 'dark') {
      apply(saved)
      return
    }
    apply(window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light')
  }

  function toggle() {
    apply(theme.value === 'dark' ? 'light' : 'dark')
  }

  return { theme, init, toggle, apply }
}
