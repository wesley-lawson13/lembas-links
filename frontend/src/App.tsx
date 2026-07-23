import { Outlet } from 'react-router-dom'
import ThemeToggle from './components/ThemeToggle'

// App is the routed shell around both pages: a fixed-width column with the
// theme toggle pinned top-right and the active page rendered below.
function App() {
  return (
    <div className="page">
      <div className="page-toolbar">
        <ThemeToggle />
      </div>
      <Outlet />
    </div>
  )
}

export default App
