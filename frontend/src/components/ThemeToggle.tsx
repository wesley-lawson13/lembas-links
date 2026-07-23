import { useTheme } from '../useTheme'

// The ☀/☾ pill from the design's browser strip, relocated to the page header:
// the active side is filled with the accent color. Rendered once in App's toolbar.
function ThemeToggle() {
  const [theme, setTheme] = useTheme()

  return (
    <div className="theme-toggle">
      <button
        type="button"
        title="Parchment (light) theme"
        className={theme === 'light' ? 'active' : ''}
        onClick={() => setTheme('light')}
      >
        ☀
      </button>
      <button
        type="button"
        title="Rivendell (dark) theme"
        className={theme === 'dark' ? 'active' : ''}
        onClick={() => setTheme('dark')}
      >
        ☾
      </button>
    </div>
  )
}

export default ThemeToggle
