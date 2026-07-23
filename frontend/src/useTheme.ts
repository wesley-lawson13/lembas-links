import { useState } from 'react'

export type Theme = 'light' | 'dark'

// Reads the current theme from the <html> data-theme attribute (set pre-paint
// by index.html) and returns it with a setter that applies the attribute and
// persists the explicit choice. Called only by ThemeToggle — everything else
// is themed purely via CSS variables.
export function useTheme(): [Theme, (t: Theme) => void] {
  const [theme, setThemeState] = useState<Theme>(() =>
    document.documentElement.dataset.theme === 'dark' ? 'dark' : 'light',
  )

  const setTheme = (t: Theme) => {
    document.documentElement.dataset.theme = t
    localStorage.setItem('lembas_theme', t)
    setThemeState(t)
  }

  return [theme, setTheme]
}
